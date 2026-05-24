package core

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

const HardMaxRuleInterval = 30 * time.Minute

type Config struct {
	Port               string
	TelegramBotToken   string
	TelegramChatID     string
	DatabasePath       string
	BybitRestURL       string
	BybitWebSocketURL  string
	MaxRules           int
	MaxInterval        time.Duration
	MaxPointsPerSymbol int
	DefaultBlacklist   []string
}

func LoadConfig() Config {
	_ = LoadDotEnv(".env")

	maxInterval := envDuration("MAX_INTERVAL", HardMaxRuleInterval)
	if maxInterval > HardMaxRuleInterval {
		maxInterval = HardMaxRuleInterval
	}

	return Config{
		Port:               env("PORT", "8000"),
		TelegramBotToken:   env("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:     env("TELEGRAM_CHAT_ID", ""),
		DatabasePath:       env("DATABASE_PATH", "pumpscreener.db"),
		BybitRestURL:       env("BYBIT_REST_URL", "https://api.bybit.com"),
		BybitWebSocketURL:  env("BYBIT_WS_URL", "wss://stream.bybit.com/v5/public/linear"),
		MaxRules:           envInt("MAX_RULES", 20),
		MaxInterval:        maxInterval,
		MaxPointsPerSymbol: envInt("MAX_POINTS_PER_SYMBOL", 4096),
		DefaultBlacklist:   envList("BLACKLIST"),
	}
}

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, ""))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := env(key, "")
	if value == "" {
		return fallback
	}

	duration, err := ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func envList(key string) []string {
	value := env(key, "")
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.ToUpper(strings.TrimSpace(part))
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
