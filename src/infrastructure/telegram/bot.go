package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pumpscreener/src/core"
	"pumpscreener/src/domain"
)

type CommandHandler interface {
	AddRule(ctx context.Context, direction string, percent string, interval string, cooldown string, modeValues ...string) (string, error)
	ListRules(ctx context.Context) (string, error)
	DeleteRule(ctx context.Context, id string) (string, error)
	SetRuleEnabled(ctx context.Context, id string, enabled bool) (string, error)
	ListBlacklist(ctx context.Context) (string, error)
	AddBlacklist(ctx context.Context, symbol string) (string, error)
	RemoveBlacklist(ctx context.Context, symbol string) (string, error)
	Status(ctx context.Context) string
}

type Bot struct {
	token      string
	chatID     string
	apiBase    string
	httpClient *http.Client
	handler    CommandHandler
	offset     int64
}

func NewBot(token, chatID string, handler CommandHandler) *Bot {
	return &Bot{
		token:   token,
		chatID:  chatID,
		apiBase: "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		handler: handler,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	if b.token == "" || b.chatID == "" {
		return nil
	}

	for {
		updates, err := b.getUpdates(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
				continue
			}
		}

		for _, update := range updates {
			if update.UpdateID >= b.offset {
				b.offset = update.UpdateID + 1
			}
			if update.Message.Text == "" {
				continue
			}
			if strconv.FormatInt(update.Message.Chat.ID, 10) != b.chatID {
				continue
			}
			reply := b.handle(ctx, update.Message.Text)
			_ = b.SendMessage(ctx, reply)
		}
	}
}

func (b *Bot) NotifySignal(ctx context.Context, signal domain.Signal) error {
	if !b.Enabled() {
		return nil
	}
	return b.SendMessage(ctx, FormatSignal(signal))
}

func (b *Bot) SendMessage(ctx context.Context, text string) error {
	if !b.Enabled() {
		return nil
	}

	payload := map[string]string{
		"chat_id": b.chatID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiBase+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage returned status %d", resp.StatusCode)
	}
	return nil
}

func (b *Bot) Enabled() bool {
	return b.token != "" && b.chatID != ""
}

func (b *Bot) getUpdates(ctx context.Context) ([]update, error) {
	values := url.Values{}
	values.Set("timeout", "25")
	values.Set("offset", strconv.FormatInt(b.offset, 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.apiBase+"/getUpdates?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload updatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, fmt.Errorf("telegram getUpdates failed")
	}
	return payload.Result, nil
}

func (b *Bot) handle(ctx context.Context, text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Команда пустая"
	}

	switch parts[0] {
	case "/start", "/help":
		return help()
	case "/status":
		return b.handler.Status(ctx)
	case "/add":
		if len(parts) != 5 && len(parts) != 6 && len(parts) != 7 {
			return "Формат: /add up 3 30m 15m или /add up 3 5m 1m hold 3m"
		}
		return result(b.handler.AddRule(ctx, parts[1], parts[2], parts[3], parts[4], parts[5:]...))
	case "/rules":
		return result(b.handler.ListRules(ctx))
	case "/delete":
		if len(parts) != 2 {
			return "Формат: /delete 3"
		}
		return result(b.handler.DeleteRule(ctx, parts[1]))
	case "/pause":
		if len(parts) != 2 {
			return "Формат: /pause 3"
		}
		return result(b.handler.SetRuleEnabled(ctx, parts[1], false))
	case "/resume":
		if len(parts) != 2 {
			return "Формат: /resume 3"
		}
		return result(b.handler.SetRuleEnabled(ctx, parts[1], true))
	case "/blacklist":
		return result(b.handler.ListBlacklist(ctx))
	case "/blacklist_add":
		if len(parts) != 2 {
			return "Формат: /blacklist_add BTCUSDT"
		}
		return result(b.handler.AddBlacklist(ctx, parts[1]))
	case "/blacklist_remove":
		if len(parts) != 2 {
			return "Формат: /blacklist_remove BTCUSDT"
		}
		return result(b.handler.RemoveBlacklist(ctx, parts[1]))
	default:
		return "Неизвестная команда. Напиши /help"
	}
}

func result(message string, err error) string {
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	return message
}

func help() string {
	return strings.Join([]string{
		"Команды pumpscreener:",
		"/add up 3 30m 15m",
		"/add up 3 5m 1m hold 3m",
		"/add down 5 1h 30m",
		"/rules",
		"/delete 3",
		"/pause 3",
		"/resume 3",
		"/blacklist",
		"/blacklist_add BTCUSDT",
		"/blacklist_remove BTCUSDT",
		"/status",
	}, "\n")
}

func FormatSignal(signal domain.Signal) string {
	return fmt.Sprintf(
		"RULE TRIGGERED: %s %s\nDaily signal: #%d for rule #%d + %s\nMode: %s%s\nMove: %.2f%% within %s\nRule: %s%.2f%% / %s\nPrice: %.8g USDT\nCooldown: %s\nBybit: https://www.bybit.com/trade/usdt/%s",
		signal.Symbol,
		signal.Rule.Direction,
		signal.DailyNo,
		signal.Rule.ID,
		signal.Symbol,
		signal.Rule.EffectiveMode(),
		formatHold(signal.Rule),
		signal.Move,
		core.HumanDuration(signal.Window),
		signal.Rule.Direction.Sign(),
		signal.Rule.Percent,
		core.HumanDuration(signal.Rule.Interval),
		signal.Price,
		core.HumanDuration(signal.Rule.Cooldown),
		signal.Symbol,
	)
}

func formatHold(rule domain.Rule) string {
	if rule.EffectiveMode() != domain.RuleModeHold {
		return ""
	}
	return " " + core.HumanDuration(rule.Hold)
}

type updatesResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}
