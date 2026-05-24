package domain

import (
	"fmt"
	"strings"
)

type RuleMode string

const (
	RuleModeInstant RuleMode = "instant"
	RuleModeHold    RuleMode = "hold"
)

func ParseRuleMode(value string) (RuleMode, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return RuleModeInstant, nil
	}

	switch RuleMode(value) {
	case RuleModeInstant:
		return RuleModeInstant, nil
	case RuleModeHold:
		return RuleModeHold, nil
	default:
		return "", fmt.Errorf("unknown rule mode %q: use instant or hold", value)
	}
}

func (m RuleMode) String() string {
	if m == "" {
		return string(RuleModeInstant)
	}
	return string(m)
}
