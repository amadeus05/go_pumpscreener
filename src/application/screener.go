package application

import (
	"fmt"
	"sync"
	"time"

	"pumpscreener/src/domain"
)

type Screener struct {
	mu        sync.RWMutex
	rules     *RuleBook
	windows   *PriceWindows
	cooldowns *Cooldowns
	holds     map[string]holdState
}

type holdState struct {
	since  time.Time
	signal domain.Signal
}

func NewScreener(rules []domain.Rule, maxPointsPerSymbol int) *Screener {
	book := NewRuleBook(rules)
	windows := NewPriceWindows(0, maxPointsPerSymbol)
	windows.SetIntervals(book.Intervals())

	return &Screener{
		rules:     book,
		windows:   windows,
		cooldowns: NewCooldowns(),
		holds:     make(map[string]holdState),
	}
}

func (s *Screener) ReplaceRules(rules []domain.Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rules.Replace(rules)
	s.windows.SetIntervals(s.rules.Intervals())
}

func (s *Screener) ProcessTick(tick domain.Tick) []domain.Signal {
	s.mu.Lock()
	defer s.mu.Unlock()

	active := s.rules.Active()
	if len(active) == 0 {
		return nil
	}

	s.windows.Add(tick)

	signals := make([]domain.Signal, 0, len(active))
	for _, rule := range active {
		if !s.cooldowns.Ready(rule, tick.Symbol, tick.Time) {
			continue
		}

		stats := s.windows.Stats(tick.Symbol, rule.Interval)
		signal, ok := domain.CheckRule(rule, tick, stats)
		if !ok {
			delete(s.holds, holdKey(rule.ID, tick.Symbol))
			continue
		}

		if rule.EffectiveMode() == domain.RuleModeHold {
			ready, heldSignal := s.confirmHold(rule, tick.Symbol, tick.Time, signal)
			if !ready {
				continue
			}
			signal = heldSignal
		}

		s.cooldowns.Hit(rule, tick.Symbol, tick.Time)
		delete(s.holds, holdKey(rule.ID, tick.Symbol))
		signals = append(signals, signal)
	}

	return signals
}

func (s *Screener) confirmHold(rule domain.Rule, symbol string, now time.Time, signal domain.Signal) (bool, domain.Signal) {
	key := holdKey(rule.ID, symbol)
	state, exists := s.holds[key]
	if !exists {
		s.holds[key] = holdState{since: now, signal: signal}
		return false, domain.Signal{}
	}

	state.signal = signal
	s.holds[key] = state
	if now.Sub(state.since) < rule.Hold {
		return false, domain.Signal{}
	}

	signal.Triggered = now
	return true, signal
}

func holdKey(ruleID domain.RuleID, symbol string) string {
	return fmt.Sprintf("%d:%s", ruleID, symbol)
}

func (s *Screener) ActiveRules() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rules.CountActive()
}

func (s *Screener) TrackedSymbols() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.windows.Symbols()
}
