package plugin

import (
	"fmt"
	"sync"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*RegistryEntry
	sources map[string]Source
}

// RegistryEntry pairs a plugin's metadata with the runtime source that
// dispatches Validate/Discover calls to the plugin binary.
type RegistryEntry struct {
	Meta   pluginsdk.Meta
	Source Source
}

var globalRegistry = &Registry{
	plugins: make(map[string]*RegistryEntry),
	sources: make(map[string]Source),
}

func GetRegistry() *Registry {
	return globalRegistry
}

func (r *Registry) Register(meta pluginsdk.Meta, source Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[meta.ID]; exists {
		return fmt.Errorf("plugin %s already registered", meta.ID)
	}

	r.plugins[meta.ID] = &RegistryEntry{
		Meta:   meta,
		Source: source,
	}
	r.sources[meta.ID] = source

	return nil
}

func (r *Registry) Get(id string) (*RegistryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", id)
	}

	return entry, nil
}

func (r *Registry) GetSource(id string) (Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	source, exists := r.sources[id]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", id)
	}

	return source, nil
}

func (r *Registry) List() []pluginsdk.Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metas := make([]pluginsdk.Meta, 0, len(r.plugins))
	for _, entry := range r.plugins {
		metas = append(metas, entry.Meta)
	}

	return metas
}
