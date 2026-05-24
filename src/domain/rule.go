package domain

import (
	"fmt"
	"time"
)

type RuleID int64

type Rule struct {
	ID        RuleID
	Direction Direction
	Percent   float64
	Interval  time.Duration
	Cooldown  time.Duration
	Mode      RuleMode
	Hold      time.Duration
	Enabled   bool
	CreatedAt time.Time
}

func NewRule(direction Direction, percent float64, interval, cooldown time.Duration) (Rule, error) {
	if percent <= 0 {
		return Rule{}, fmt.Errorf("percent must be greater than zero")
	}
	if interval <= 0 {
		return Rule{}, fmt.Errorf("interval must be greater than zero")
	}
	if cooldown <= 0 {
		return Rule{}, fmt.Errorf("cooldown must be greater than zero")
	}

	return Rule{
		Direction: direction,
		Percent:   percent,
		Interval:  interval,
		Cooldown:  cooldown,
		Mode:      RuleModeInstant,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (r Rule) IsActive() bool {
	if !r.Enabled || r.Percent <= 0 || r.Interval <= 0 || r.Cooldown <= 0 {
		return false
	}
	if r.EffectiveMode() == RuleModeHold && r.Hold <= 0 {
		return false
	}
	return true
}

func (r Rule) EffectiveMode() RuleMode {
	if r.Mode == "" {
		return RuleModeInstant
	}
	return r.Mode
}
