package provider

import (
	"fmt"
	"sync"
)

// Registry holds all registered provider adapters and dispatches callbacks.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]ProviderAdapter
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]ProviderAdapter)}
}

// Register adds or replaces an adapter for the given provider name.
func (r *Registry) Register(adapter ProviderAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Name()] = adapter
}

// Get returns the adapter for the given provider name or an error if not found.
func (r *Registry) Get(name string) (ProviderAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", name)
	}
	return a, nil
}

// All returns a copy of all registered adapters.
func (r *Registry) All() []ProviderAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderAdapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		out = append(out, a)
	}
	return out
}

// Names returns the list of registered provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	return names
}
