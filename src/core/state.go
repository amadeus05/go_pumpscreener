package core

import (
	"sync"
	"time"
)

type AppState struct {
	startedAt time.Time

	mu             sync.RWMutex
	websocketState string
	activeRules    int
	trackedSymbols int
	knownPairs     int
	lastTickAt     time.Time
	lastError      string
}

func NewAppState() *AppState {
	return &AppState{
		startedAt:      time.Now().UTC(),
		websocketState: "starting",
	}
}

func (s *AppState) SetWebSocketState(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.websocketState = value
}

func (s *AppState) SetStats(activeRules, trackedSymbols, knownPairs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeRules = activeRules
	s.trackedSymbols = trackedSymbols
	s.knownPairs = knownPairs
}

func (s *AppState) MarkTick(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTickAt = at
}

func (s *AppState) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.lastError = ""
		return
	}
	s.lastError = err.Error()
}

func (s *AppState) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return StateSnapshot{
		StartedAt:      s.startedAt,
		Uptime:         time.Since(s.startedAt),
		WebSocketState: s.websocketState,
		ActiveRules:    s.activeRules,
		TrackedSymbols: s.trackedSymbols,
		KnownPairs:     s.knownPairs,
		LastTickAt:     s.lastTickAt,
		LastError:      s.lastError,
	}
}

type StateSnapshot struct {
	StartedAt      time.Time
	Uptime         time.Duration
	WebSocketState string
	ActiveRules    int
	TrackedSymbols int
	KnownPairs     int
	LastTickAt     time.Time
	LastError      string
}
