package application

import (
	"testing"
	"time"

	"pumpscreener/src/domain"
)

func TestParseModeHoldUsesRuleIntervalByDefault(t *testing.T) {
	mode, hold, err := parseMode([]string{"hold"}, 5*time.Minute)
	if err != nil {
		t.Fatalf("parseMode returned error: %v", err)
	}
	if mode != domain.RuleModeHold {
		t.Fatalf("mode = %q, want %q", mode, domain.RuleModeHold)
	}
	if hold != 5*time.Minute {
		t.Fatalf("hold = %s, want 5m", hold)
	}
}

func TestParseModeHoldDurationOverrideStillWorks(t *testing.T) {
	mode, hold, err := parseMode([]string{"hold", "3m"}, 5*time.Minute)
	if err != nil {
		t.Fatalf("parseMode returned error: %v", err)
	}
	if mode != domain.RuleModeHold {
		t.Fatalf("mode = %q, want %q", mode, domain.RuleModeHold)
	}
	if hold != 3*time.Minute {
		t.Fatalf("hold = %s, want 3m", hold)
	}
}
