package application

import (
	"fmt"
	"time"

	"pumpscreener/src/domain"
)

type Cooldowns struct {
	nextAllowed map[string]time.Time
}

func NewCooldowns() *Cooldowns {
	return &Cooldowns{nextAllowed: make(map[string]time.Time)}
}

func (c *Cooldowns) Ready(rule domain.Rule, symbol string, now time.Time) bool {
	return !now.Before(c.nextAllowed[key(rule.ID, symbol)])
}

func (c *Cooldowns) Hit(rule domain.Rule, symbol string, now time.Time) {
	c.nextAllowed[key(rule.ID, symbol)] = now.Add(rule.Cooldown)
}

func (c *Cooldowns) ForgetRule(ruleID domain.RuleID) {
	prefix := fmt.Sprintf("%d:", ruleID)
	for item := range c.nextAllowed {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			delete(c.nextAllowed, item)
		}
	}
}

func key(ruleID domain.RuleID, symbol string) string {
	return fmt.Sprintf("%d:%s", ruleID, symbol)
}
