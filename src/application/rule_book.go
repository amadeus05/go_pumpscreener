package application

import (
	"sort"
	"time"

	"pumpscreener/src/domain"
)

type RuleBook struct {
	rules []domain.Rule
}

func NewRuleBook(rules []domain.Rule) *RuleBook {
	book := &RuleBook{}
	book.Replace(rules)
	return book
}

func (b *RuleBook) Replace(rules []domain.Rule) {
	b.rules = append(b.rules[:0], rules...)
	sort.Slice(b.rules, func(i, j int) bool {
		return b.rules[i].ID < b.rules[j].ID
	})
}

func (b *RuleBook) All() []domain.Rule {
	copied := make([]domain.Rule, len(b.rules))
	copy(copied, b.rules)
	return copied
}

func (b *RuleBook) Active() []domain.Rule {
	active := make([]domain.Rule, 0, len(b.rules))
	for _, rule := range b.rules {
		if rule.IsActive() {
			active = append(active, rule)
		}
	}
	return active
}

func (b *RuleBook) Intervals() []time.Duration {
	seen := make(map[time.Duration]struct{})
	for _, rule := range b.rules {
		if !rule.IsActive() {
			continue
		}
		seen[rule.Interval] = struct{}{}
	}

	intervals := make([]time.Duration, 0, len(seen))
	for interval := range seen {
		intervals = append(intervals, interval)
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i] < intervals[j]
	})
	return intervals
}

func (b *RuleBook) CountActive() int {
	count := 0
	for _, rule := range b.rules {
		if rule.IsActive() {
			count++
		}
	}
	return count
}
