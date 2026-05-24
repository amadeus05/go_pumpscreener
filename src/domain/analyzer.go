package domain

import "math"

type WindowStats struct {
	MinPrice float64
	MaxPrice float64
}

func CheckRule(rule Rule, tick Tick, stats WindowStats) (Signal, bool) {
	if !rule.IsActive() || tick.Price <= 0 {
		return Signal{}, false
	}

	var move float64
	switch rule.Direction {
	case DirectionUp:
		if stats.MinPrice <= 0 {
			return Signal{}, false
		}
		move = percentChange(stats.MinPrice, tick.Price)
	case DirectionDown:
		if stats.MaxPrice <= 0 {
			return Signal{}, false
		}
		move = percentChange(stats.MaxPrice, tick.Price)
	default:
		return Signal{}, false
	}

	if math.Abs(move) < rule.Percent {
		return Signal{}, false
	}
	if rule.Direction == DirectionUp && move < 0 {
		return Signal{}, false
	}
	if rule.Direction == DirectionDown && move > 0 {
		return Signal{}, false
	}

	return Signal{
		Rule:      rule,
		Symbol:    tick.Symbol,
		Price:     tick.Price,
		Move:      move,
		Window:    rule.Interval,
		Triggered: tick.Time,
	}, true
}

func percentChange(from, to float64) float64 {
	return ((to - from) / from) * 100
}
