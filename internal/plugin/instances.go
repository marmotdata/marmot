package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// InstanceStatus describes one long-running plugin instance for status
// endpoints and the admin UI.
type InstanceStatus struct {
	PluginID      string    `json:"plugin_id"`
	StartedAt     time.Time `json:"started_at"`
	LastUsedAt    time.Time `json:"last_used_at"`
	ActiveQueries int       `json:"active_queries"`
	Restarts      int       `json:"restarts"`
} // @name PluginInstanceStatus

// InstanceManager keeps query-capable plugin processes alive between calls,
// unlike the discovery path which spawns a process per call. Instances are
// keyed by plugin id, started lazily, health-checked with a ping before
// reuse, restarted when dead and reaped after sitting idle; connection
// pooling per target config is the plugin's own concern.
type InstanceManager struct {
	mu        sync.Mutex
	instances map[string]*instance
	restarts  map[string]int
	idleTTL   time.Duration
	stop      chan struct{}
	stopOnce  sync.Once
}

type instance struct {
	process   *pluginsdk.PluginProcess
	startedAt time.Time
	lastUsed  time.Time
	active    int
}

func NewInstanceManager(idleTTL time.Duration) *InstanceManager {
	m := &InstanceManager{
		instances: make(map[string]*instance),
		restarts:  make(map[string]int),
		idleTTL:   idleTTL,
		stop:      make(chan struct{}),
	}
	go m.reapLoop()
	return m
}

// Stop kills every instance and stops the reaper.
func (m *InstanceManager) Stop() {
	m.stopOnce.Do(func() { close(m.stop) })

	m.mu.Lock()
	defer m.mu.Unlock()
	for id, inst := range m.instances {
		inst.process.Kill()
		delete(m.instances, id)
	}
}

func (m *InstanceManager) reapLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.reapIdle()
		}
	}
}

func (m *InstanceManager) reapIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, inst := range m.instances {
		if inst.active == 0 && time.Since(inst.lastUsed) > m.idleTTL {
			log.Debug().Str("plugin", id).Msg("Reaping idle plugin instance")
			inst.process.Kill()
			delete(m.instances, id)
		}
	}
}

// acquire returns a live process for the plugin, starting or restarting one
// as needed, and marks it busy until release is called.
func (m *InstanceManager) acquire(pluginID string) (*pluginsdk.PluginProcess, error) {
	entry, err := GetRegistry().Get(pluginID)
	if err != nil {
		return nil, err
	}
	external, ok := entry.Source.(interface{ BinaryPath() string })
	if !ok {
		return nil, fmt.Errorf("plugin %s is not an external plugin", pluginID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[pluginID]; ok {
		if inst.process.Exited() || inst.process.Ping() != nil {
			log.Warn().Str("plugin", pluginID).Msg("Plugin instance died, restarting")
			inst.process.Kill()
			delete(m.instances, pluginID)
			m.restarts[pluginID]++
		} else {
			inst.lastUsed = time.Now()
			inst.active++
			return inst.process, nil
		}
	}

	process, err := pluginsdk.Open(external.BinaryPath(), pluginLogger())
	if err != nil {
		return nil, fmt.Errorf("starting plugin instance %s: %w", pluginID, err)
	}
	m.instances[pluginID] = &instance{
		process:   process,
		startedAt: time.Now(),
		lastUsed:  time.Now(),
		active:    1,
	}
	log.Info().Str("plugin", pluginID).Msg("Started long-running plugin instance")
	return process, nil
}

func (m *InstanceManager) release(pluginID string, process *pluginsdk.PluginProcess) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inst, ok := m.instances[pluginID]; ok && inst.process == process && inst.active > 0 {
		inst.active--
		inst.lastUsed = time.Now()
	}
}

// PlanQuery asks the plugin what a statement would touch.
func (m *InstanceManager) PlanQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	process, err := m.acquire(pluginID)
	if err != nil {
		return nil, err
	}
	defer m.release(pluginID, process)
	return process.Source.PlanQuery(ctx, pluginsdk.RawConfig(config), req)
}

// ExecuteQuery runs a statement on the plugin's engine. The instance stays
// busy until the returned stream is closed, which keeps the reaper away
// while rows are still flowing.
func (m *InstanceManager) ExecuteQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	process, err := m.acquire(pluginID)
	if err != nil {
		return nil, err
	}

	stream, err := process.Source.ExecuteQuery(ctx, pluginsdk.RawConfig(config), req)
	if err != nil {
		m.release(pluginID, process)
		return nil, err
	}
	return &managedStream{
		QueryStream: stream,
		release:     func() { m.release(pluginID, process) },
	}, nil
}

// Status reports every live instance.
func (m *InstanceManager) Status() []InstanceStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	statuses := make([]InstanceStatus, 0, len(m.instances))
	for id, inst := range m.instances {
		statuses = append(statuses, InstanceStatus{
			PluginID:      id,
			StartedAt:     inst.startedAt,
			LastUsedAt:    inst.lastUsed,
			ActiveQueries: inst.active,
			Restarts:      m.restarts[id],
		})
	}
	return statuses
}

// managedStream releases the instance's busy mark exactly once when the
// stream is closed.
type managedStream struct {
	pluginsdk.QueryStream
	release     func()
	releaseOnce sync.Once
}

func (s *managedStream) Close() error {
	err := s.QueryStream.Close()
	s.releaseOnce.Do(s.release)
	return err
}
