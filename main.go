package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pumpscreener/src/application"
	"pumpscreener/src/core"
	"pumpscreener/src/domain"
	"pumpscreener/src/infrastructure/bybit"
	"pumpscreener/src/infrastructure/httpserver"
	"pumpscreener/src/infrastructure/lognotifier"
	"pumpscreener/src/infrastructure/storage"
	"pumpscreener/src/infrastructure/telegram"
)

type appStore interface {
	application.RuleRepository
	application.BlacklistRepository
	application.SignalRepository
	EnsureBlacklist(ctx context.Context, symbols []string) error
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := core.LoadConfig()
	state := core.NewAppState()
	log.Printf("pumpscreener starting")

	store, storageDescription, err := openStore(ctx, cfg)
	if err != nil {
		log.Fatalf("open storage: %v", err)
	}
	log.Printf("storage ready: %s", storageDescription)
	if err := store.EnsureBlacklist(ctx, cfg.DefaultBlacklist); err != nil {
		log.Fatalf("seed blacklist: %v", err)
	}

	rules, err := store.ListRules(ctx)
	if err != nil {
		log.Fatalf("load rules: %v", err)
	}
	log.Printf("rules loaded: %d", len(rules))
	screener := application.NewScreener(rules, cfg.MaxPointsPerSymbol, cfg.PriceBucketInterval)
	log.Printf("price buckets: interval=%s max_per_symbol=%d", cfg.PriceBucketInterval, cfg.MaxPointsPerSymbol)
	bybitClient := bybit.NewClient(cfg.BybitRestURL)
	exchangeNow, err := bybitClient.ServerTime(ctx)
	if err != nil {
		log.Fatalf("sync bybit time: %v", err)
	}
	clock := bybit.NewClock(exchangeNow, time.Now().UTC())
	log.Printf("bybit time synced: exchange=%s offset=%s", exchangeNow.Format(time.RFC3339Nano), clock.Offset())

	blacklist, err := store.ListBlacklist(ctx)
	if err != nil {
		log.Fatalf("load blacklist: %v", err)
	}
	log.Printf("blacklist loaded: %d", len(blacklist))
	log.Printf("loading bybit instruments: %s", cfg.BybitRestURL)
	pairs, err := bybitClient.LinearUSDTPerpetuals(ctx)
	if err != nil {
		log.Fatalf("load bybit pairs: %v", err)
	}
	registry := bybit.NewRegistry(pairs, blacklist)
	log.Printf("bybit instruments loaded: total=%d allowed=%d", len(pairs), registry.Count())
	commands := application.NewCommands(store, store, registry, screener, state, cfg.MaxRules, cfg.MaxInterval)
	state.SetStats(screener.ActiveRules(), screener.TrackedSymbols(), registry.Count())

	ticks := make(chan domain.Tick, 2048)
	bot := telegram.NewBot(cfg.TelegramBotToken, cfg.TelegramChatID, commands)
	notifier := application.Notifier(lognotifier.New())
	if bot.Enabled() {
		notifier = bot
		log.Printf("telegram enabled")
	} else {
		log.Printf("telegram disabled: TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is empty; signals will be written to log")
	}
	web := httpserver.New(cfg.Port, state)
	stream := bybit.NewStream(cfg.BybitWebSocketURL, clock)

	go runHTTP(ctx, web)
	if bot.Enabled() {
		go runTelegram(ctx, bot)
	}
	go runBybitStream(ctx, state, stream, registry.Symbols(), ticks)

	processTicks(ctx, state, screener, store, notifier, registry, ticks)
}

func openStore(ctx context.Context, cfg core.Config) (appStore, string, error) {
	switch cfg.StorageBackend {
	case "file", "":
		store, err := storage.Open(cfg.DatabasePath)
		return store, fmt.Sprintf("file:%s", cfg.DatabasePath), err
	case "supabase", "postgres", "postgresql":
		store, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
		return store, "supabase/postgres", err
	default:
		return nil, "", fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func runHTTP(ctx context.Context, server *httpserver.Server) {
	if err := server.Run(ctx); err != nil && ctx.Err() == nil {
		log.Printf("http server stopped: %v", err)
	}
}

func runTelegram(ctx context.Context, bot *telegram.Bot) {
	if err := bot.Run(ctx); err != nil && ctx.Err() == nil {
		log.Printf("telegram stopped: %v", err)
	}
}

func runBybitStream(ctx context.Context, state *core.AppState, stream *bybit.Stream, symbols []string, ticks chan<- domain.Tick) {
	state.SetWebSocketState("connecting")
	err := stream.Run(ctx, symbols, ticks)
	if err != nil && ctx.Err() == nil {
		state.SetError(err)
		state.SetWebSocketState("degraded")
		log.Printf("bybit stream stopped: %v", err)
	}
}

func processTicks(
	ctx context.Context,
	state *core.AppState,
	screener *application.Screener,
	store application.SignalRepository,
	notifier application.Notifier,
	registry *bybit.Registry,
	ticks <-chan domain.Tick,
) {
	state.SetWebSocketState("running")
	statsTimer := time.NewTicker(5 * time.Second)
	defer statsTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-statsTimer.C:
			state.SetStats(screener.ActiveRules(), screener.TrackedSymbols(), registry.Count())
		case tick := <-ticks:
			if !registry.Allows(tick.Symbol) {
				continue
			}
			state.MarkTick(tick.Time)
			for _, signal := range screener.ProcessTick(tick) {
				dailyNo, err := store.NextDailySignalNumber(ctx, signal.Rule.ID, signal.Symbol, signal.Triggered)
				if err != nil {
					log.Printf("next daily signal number: %v", err)
					dailyNo = 1
				}
				signal.DailyNo = dailyNo

				if err := store.SaveSignal(ctx, signal); err != nil {
					log.Printf("save signal: %v", err)
				}
				if err := notifier.NotifySignal(ctx, signal); err != nil {
					log.Printf("notify signal: %v", err)
				}
			}
		}
	}
}
