package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, fmt.Errorf("duration is empty")
	}

	unit := value[len(value)-1]
	number := strings.TrimSpace(value[:len(value)-1])
	amount, err := strconv.Atoi(number)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("invalid duration %q", value)
	}

	switch unit {
	case 's':
		return time.Duration(amount) * time.Second, nil
	case 'm':
		return time.Duration(amount) * time.Minute, nil
	case 'h':
		return time.Duration(amount) * time.Hour, nil
	case 'd':
		return time.Duration(amount) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit %q", unit)
	}
}

func HumanDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}

	days := value / (24 * time.Hour)
	value -= days * 24 * time.Hour
	hours := value / time.Hour
	value -= hours * time.Hour
	minutes := value / time.Minute
	value -= minutes * time.Minute
	seconds := value / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm %02ds", days, hours, minutes, seconds)
	}
	return fmt.Sprintf("%02dh %02dm %02ds", hours, minutes, seconds)
}
