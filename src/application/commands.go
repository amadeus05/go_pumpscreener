package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"pumpscreener/src/core"
	"pumpscreener/src/domain"
)

type Commands struct {
	rules            RuleRepository
	blacklist        BlacklistRepository
	blacklistRuntime BlacklistRuntime
	screener         *Screener
	state            *core.AppState
	maxRules         int
	maxInterval      time.Duration
}

func NewCommands(rules RuleRepository, blacklist BlacklistRepository, blacklistRuntime BlacklistRuntime, screener *Screener, state *core.AppState, maxRules int, maxInterval time.Duration) *Commands {
	return &Commands{
		rules:            rules,
		blacklist:        blacklist,
		blacklistRuntime: blacklistRuntime,
		screener:         screener,
		state:            state,
		maxRules:         maxRules,
		maxInterval:      maxInterval,
	}
}

func (c *Commands) AddRule(ctx context.Context, directionValue, percentValue, intervalValue, cooldownValue string, modeValues ...string) (string, error) {
	existing, err := c.rules.ListRules(ctx)
	if err != nil {
		return "", err
	}
	if c.maxRules > 0 && len(existing) >= c.maxRules {
		return "", fmt.Errorf("max rules limit reached: %d", c.maxRules)
	}

	direction, err := domain.ParseDirection(directionValue)
	if err != nil {
		return "", err
	}
	percent, err := strconv.ParseFloat(percentValue, 64)
	if err != nil {
		return "", fmt.Errorf("invalid percent %q", percentValue)
	}
	interval, err := core.ParseDuration(intervalValue)
	if err != nil {
		return "", err
	}
	if c.maxInterval > 0 && interval > c.maxInterval {
		return "", fmt.Errorf("interval is too large, max is %s", core.HumanDuration(c.maxInterval))
	}
	cooldown, err := core.ParseDuration(cooldownValue)
	if err != nil {
		return "", err
	}

	rule, err := domain.NewRule(direction, percent, interval, cooldown)
	if err != nil {
		return "", err
	}
	if len(modeValues) > 0 {
		mode, hold, err := parseMode(modeValues, interval)
		if err != nil {
			return "", err
		}
		rule.Mode = mode
		rule.Hold = hold
	}
	rule, err = c.rules.CreateRule(ctx, rule)
	if err != nil {
		return "", err
	}

	if err := c.reloadRules(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Правило #%d добавлено: %s %.2f%% за %s, cooldown %s, mode %s%s", rule.ID, rule.Direction, rule.Percent, core.HumanDuration(rule.Interval), core.HumanDuration(rule.Cooldown), rule.EffectiveMode(), formatHold(rule)), nil
}

func (c *Commands) ListRules(ctx context.Context) (string, error) {
	rules, err := c.rules.ListRules(ctx)
	if err != nil {
		return "", err
	}
	if len(rules) == 0 {
		return "Правил пока нет. Добавь: /add up 3 30m 15m", nil
	}

	lines := make([]string, 0, len(rules)+1)
	lines = append(lines, "Правила:")
	for _, rule := range rules {
		status := "on"
		if !rule.Enabled {
			status = "off"
		}
		lines = append(lines, fmt.Sprintf("#%d [%s] %s %.2f%% за %s, cooldown %s, mode %s%s", rule.ID, status, rule.Direction, rule.Percent, core.HumanDuration(rule.Interval), core.HumanDuration(rule.Cooldown), rule.EffectiveMode(), formatHold(rule)))
	}
	return strings.Join(lines, "\n"), nil
}

func (c *Commands) RuleItems(ctx context.Context) ([]domain.Rule, error) {
	return c.rules.ListRules(ctx)
}

func (c *Commands) DeleteRule(ctx context.Context, idValue string) (string, error) {
	id, err := parseRuleID(idValue)
	if err != nil {
		return "", err
	}
	if err := c.rules.DeleteRule(ctx, id); err != nil {
		return "", err
	}
	if err := c.reloadRules(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("Правило #%d удалено", id), nil
}

func (c *Commands) SetRuleEnabled(ctx context.Context, idValue string, enabled bool) (string, error) {
	id, err := parseRuleID(idValue)
	if err != nil {
		return "", err
	}
	if err := c.rules.SetRuleEnabled(ctx, id, enabled); err != nil {
		return "", err
	}
	if err := c.reloadRules(ctx); err != nil {
		return "", err
	}

	action := "включено"
	if !enabled {
		action = "выключено"
	}
	return fmt.Sprintf("Правило #%d %s", id, action), nil
}

func (c *Commands) ListBlacklist(ctx context.Context) (string, error) {
	items, err := c.blacklist.ListBlacklist(ctx)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "Blacklist пуст", nil
	}
	return "Blacklist:\n" + strings.Join(items, "\n"), nil
}

func (c *Commands) AddBlacklist(ctx context.Context, symbol string) (string, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return "", fmt.Errorf("symbol is empty")
	}
	if err := c.blacklist.AddBlacklist(ctx, symbol); err != nil {
		return "", err
	}
	if err := c.reloadBlacklist(ctx); err != nil {
		return "", err
	}
	return symbol + " добавлен в blacklist", nil
}

func (c *Commands) RemoveBlacklist(ctx context.Context, symbol string) (string, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return "", fmt.Errorf("symbol is empty")
	}
	if err := c.blacklist.RemoveBlacklist(ctx, symbol); err != nil {
		return "", err
	}
	if err := c.reloadBlacklist(ctx); err != nil {
		return "", err
	}
	return symbol + " удален из blacklist", nil
}

func (c *Commands) Status(ctx context.Context) string {
	snapshot := c.state.Snapshot()
	return fmt.Sprintf(
		"Status: %s\nUptime: %s\nPairs: %d\nTracked: %d\nActive rules: %d",
		snapshot.WebSocketState,
		core.HumanDuration(snapshot.Uptime),
		snapshot.KnownPairs,
		snapshot.TrackedSymbols,
		snapshot.ActiveRules,
	)
}

func (c *Commands) reloadRules(ctx context.Context) error {
	rules, err := c.rules.ListRules(ctx)
	if err != nil {
		return err
	}
	c.screener.ReplaceRules(rules)
	return nil
}

func (c *Commands) reloadBlacklist(ctx context.Context) error {
	if c.blacklistRuntime == nil {
		return nil
	}
	items, err := c.blacklist.ListBlacklist(ctx)
	if err != nil {
		return err
	}
	c.blacklistRuntime.ApplyBlacklist(items)
	return nil
}

func parseRuleID(value string) (domain.RuleID, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid rule id %q", value)
	}
	return domain.RuleID(id), nil
}

func parseMode(values []string, defaultHold time.Duration) (domain.RuleMode, time.Duration, error) {
	if len(values) == 0 {
		return domain.RuleModeInstant, 0, nil
	}
	if len(values) != 1 && len(values) != 2 {
		return "", 0, fmt.Errorf("mode format: instant, hold, or hold 3m")
	}

	mode, err := domain.ParseRuleMode(values[0])
	if err != nil {
		return "", 0, err
	}
	if mode == domain.RuleModeInstant {
		return mode, 0, nil
	}
	if len(values) == 1 {
		if defaultHold <= 0 {
			return "", 0, fmt.Errorf("hold duration is empty")
		}
		return mode, defaultHold, nil
	}

	hold, err := core.ParseDuration(values[1])
	if err != nil {
		return "", 0, err
	}
	return mode, hold, nil
}

func formatHold(rule domain.Rule) string {
	if rule.EffectiveMode() != domain.RuleModeHold {
		return ""
	}
	return " " + core.HumanDuration(rule.Hold)
}
