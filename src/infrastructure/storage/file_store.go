package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pumpscreener/src/domain"
)

type Store struct {
	path string
	mu   sync.Mutex
	data fileData
}

type fileData struct {
	NextRuleID int64          `json:"next_rule_id"`
	Rules      []storedRule   `json:"rules"`
	Blacklist  []string       `json:"blacklist"`
	Signals    []storedSignal `json:"signals"`
}

type storedRule struct {
	ID        int64   `json:"id"`
	Direction string  `json:"direction"`
	Percent   float64 `json:"percent"`
	Interval  int64   `json:"interval_seconds"`
	Cooldown  int64   `json:"cooldown_seconds"`
	Mode      string  `json:"mode"`
	Hold      int64   `json:"hold_seconds"`
	Enabled   bool    `json:"enabled"`
	CreatedAt string  `json:"created_at"`
}

type storedSignal struct {
	RuleID    int64   `json:"rule_id"`
	Symbol    string  `json:"symbol"`
	DailyNo   int     `json:"daily_no"`
	Price     float64 `json:"price"`
	Move      float64 `json:"move"`
	Triggered string  `json:"triggered_at"`
}

func Open(path string) (*Store, error) {
	store := &Store{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ListRules(ctx context.Context) ([]domain.Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rules := make([]domain.Rule, 0, len(s.data.Rules))
	for _, item := range s.data.Rules {
		rule, err := item.toDomain()
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (s *Store) CreateRule(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.NextRuleID == 0 {
		s.data.NextRuleID = 1
	}
	rule.ID = domain.RuleID(s.data.NextRuleID)
	s.data.NextRuleID++
	s.data.Rules = append(s.data.Rules, fromRule(rule))
	return rule, s.save()
}

func (s *Store) SetRuleEnabled(ctx context.Context, id domain.RuleID, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Rules {
		if s.data.Rules[i].ID == int64(id) {
			s.data.Rules[i].Enabled = enabled
			return s.save()
		}
	}
	return fmt.Errorf("rule %d not found", id)
}

func (s *Store) DeleteRule(ctx context.Context, id domain.RuleID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Rules {
		if s.data.Rules[i].ID == int64(id) {
			s.data.Rules = append(s.data.Rules[:i], s.data.Rules[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("rule %d not found", id)
}

func (s *Store) ListBlacklist(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := append([]string(nil), s.data.Blacklist...)
	return items, nil
}

func (s *Store) AddBlacklist(ctx context.Context, symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	for _, item := range s.data.Blacklist {
		if item == symbol {
			return nil
		}
	}
	s.data.Blacklist = append(s.data.Blacklist, symbol)
	return s.save()
}

func (s *Store) RemoveBlacklist(ctx context.Context, symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	for i, item := range s.data.Blacklist {
		if item == symbol {
			s.data.Blacklist = append(s.data.Blacklist[:i], s.data.Blacklist[i+1:]...)
			return s.save()
		}
	}
	return nil
}

func (s *Store) SaveSignal(ctx context.Context, signal domain.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Signals = append(s.data.Signals, storedSignal{
		RuleID:    int64(signal.Rule.ID),
		Symbol:    signal.Symbol,
		DailyNo:   signal.DailyNo,
		Price:     signal.Price,
		Move:      signal.Move,
		Triggered: signal.Triggered.Format(time.RFC3339),
	})
	if len(s.data.Signals) > 500 {
		s.data.Signals = s.data.Signals[len(s.data.Signals)-500:]
	}
	return s.save()
}

func (s *Store) NextDailySignalNumber(ctx context.Context, ruleID domain.RuleID, symbol string, at time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dayStart := time.Date(at.UTC().Year(), at.UTC().Month(), at.UTC().Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	count := 0

	for _, signal := range s.data.Signals {
		if signal.RuleID != int64(ruleID) || signal.Symbol != symbol {
			continue
		}
		triggeredAt, err := time.Parse(time.RFC3339, signal.Triggered)
		if err != nil {
			continue
		}
		if !triggeredAt.Before(dayStart) && triggeredAt.Before(dayEnd) {
			count++
		}
	}

	return count + 1, nil
}

func (s *Store) EnsureBlacklist(ctx context.Context, symbols []string) error {
	for _, symbol := range symbols {
		if err := s.AddBlacklist(ctx, symbol); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) load() error {
	content, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.data.NextRuleID = 1
		return s.save()
	}
	if err != nil {
		return err
	}
	if len(content) == 0 {
		s.data.NextRuleID = 1
		return nil
	}
	if err := json.Unmarshal(content, &s.data); err != nil {
		return err
	}
	if s.data.NextRuleID == 0 {
		s.data.NextRuleID = 1
	}
	return nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil && filepath.Dir(s.path) != "." {
		return err
	}

	content, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, content, 0644)
}

func fromRule(rule domain.Rule) storedRule {
	return storedRule{
		ID:        int64(rule.ID),
		Direction: rule.Direction.String(),
		Percent:   rule.Percent,
		Interval:  int64(rule.Interval.Seconds()),
		Cooldown:  int64(rule.Cooldown.Seconds()),
		Mode:      rule.EffectiveMode().String(),
		Hold:      int64(rule.Hold.Seconds()),
		Enabled:   rule.Enabled,
		CreatedAt: rule.CreatedAt.Format(time.RFC3339),
	}
}

func (r storedRule) toDomain() (domain.Rule, error) {
	direction, err := domain.ParseDirection(r.Direction)
	if err != nil {
		return domain.Rule{}, err
	}
	mode, err := domain.ParseRuleMode(r.Mode)
	if err != nil {
		return domain.Rule{}, err
	}
	createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt)
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return domain.Rule{
		ID:        domain.RuleID(r.ID),
		Direction: direction,
		Percent:   r.Percent,
		Interval:  time.Duration(r.Interval) * time.Second,
		Cooldown:  time.Duration(r.Cooldown) * time.Second,
		Mode:      mode,
		Hold:      time.Duration(r.Hold) * time.Second,
		Enabled:   r.Enabled,
		CreatedAt: createdAt,
	}, nil
}
