package application

import (
	"context"
	"time"

	"pumpscreener/src/domain"
)

type RuleRepository interface {
	ListRules(ctx context.Context) ([]domain.Rule, error)
	CreateRule(ctx context.Context, rule domain.Rule) (domain.Rule, error)
	SetRuleEnabled(ctx context.Context, id domain.RuleID, enabled bool) error
	DeleteRule(ctx context.Context, id domain.RuleID) error
}

type BlacklistRepository interface {
	ListBlacklist(ctx context.Context) ([]string, error)
	AddBlacklist(ctx context.Context, symbol string) error
	RemoveBlacklist(ctx context.Context, symbol string) error
}

type BlacklistRuntime interface {
	ApplyBlacklist(blacklist []string)
}

type SignalRepository interface {
	SaveSignal(ctx context.Context, signal domain.Signal) error
	NextDailySignalNumber(ctx context.Context, ruleID domain.RuleID, symbol string, at time.Time) (int, error)
}

type Notifier interface {
	NotifySignal(ctx context.Context, signal domain.Signal) error
}
