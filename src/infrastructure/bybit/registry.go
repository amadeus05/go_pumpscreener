package bybit

import (
	"strings"
	"sync"
)

type Registry struct {
	mu      sync.RWMutex
	allowed map[string]struct{}
	all     []string
}

func NewRegistry(symbols, blacklist []string) *Registry {
	blacklisted := make(map[string]struct{}, len(blacklist))
	for _, symbol := range blacklist {
		blacklisted[strings.ToUpper(strings.TrimSpace(symbol))] = struct{}{}
	}

	allowed := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if _, skip := blacklisted[symbol]; skip {
			continue
		}
		allowed[symbol] = struct{}{}
	}

	return &Registry{allowed: allowed, all: symbols}
}

func (r *Registry) ApplyBlacklist(blacklist []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	updated := NewRegistry(r.all, blacklist)
	r.allowed = updated.allowed
}

func (r *Registry) Allows(symbol string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.allowed[strings.ToUpper(symbol)]
	return ok
}

func (r *Registry) Symbols() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	symbols := make([]string, 0, len(r.allowed))
	for symbol := range r.allowed {
		symbols = append(symbols, symbol)
	}
	return symbols
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.allowed)
}
