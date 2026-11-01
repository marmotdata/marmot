package plugin

import (
	"bytes"
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

// InstanceManager keeps query-capable plugin processes alive between data
// queries, unlike the discovery path which spawns a process per call. Instances are
// keyed by plugin id, started lazily, health-checked with a ping before
// reuse, restarted when dead and reaped after sitting idle; connection
// pooling per target config is the plugin's own concern.
type InstanceManager struct {
	mu        sync.Mutex
	instances map[string]*instance
	restarts  map[string]int
	// starting serialises the health check and start of each plugin's process, so a slow start blocks only queries to that plugin.
	starting map[string]*sync.Mutex
	// retired holds instances of a plugin version that has since been replaced, kept alive only until their in-flight queries finish.
	retired  map[*pluginsdk.PluginProcess]*instance
	idleTTL  time.Duration
	stop     chan struct{}
	stopOnce sync.Once
}

type instance struct {
	process   *pluginsdk.PluginProcess
	sha256    []byte
	startedAt time.Time
	lastUsed  time.Time
	active    int
}

func NewInstanceManager(idleTTL time.Duration) *InstanceManager {
	m := &InstanceManager{
		instances: make(map[string]*instance),
		restarts:  make(map[string]int),
		starting:  make(map[string]*sync.Mutex),
		retired:   make(map[*pluginsdk.PluginProcess]*instance),
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
	for process := range m.retired {
		process.Kill()
		delete(m.retired, process)
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
// as needed, and marks it busy until release is called. The slow parts, the
// ping and the process start, run outside the manager's lock.
// registeredSource returns the registered source of a plugin, which the unkeyed query calls run on.
func registeredSource(pluginID string) (Source, error) {
	entry, err := GetRegistry().Get(pluginID)
	if err != nil {
		return nil, err
	}
	return entry.Source, nil
}

// acquire returns a live instance of source, kept under key. Key is the plugin id for registered plugins, and whatever the caller chose for a source it resolved itself.
func (m *InstanceManager) acquire(pluginID string, source Source) (*pluginsdk.PluginProcess, error) {
	external, ok := source.(interface{ Binary() Binary })
	if !ok {
		return nil, fmt.Errorf("plugin %s is not an external plugin", pluginID)
	}

	start := m.startLock(pluginID)
	start.Lock()
	defer start.Unlock()

	// Marking the instance busy before pinging it keeps the reaper from killing it in between.
	m.mu.Lock()
	inst, ok := m.instances[pluginID]
	if ok && !bytes.Equal(inst.sha256, external.Binary().SHA256) {
		// The plugin moved to another version: new queries start the new binary and the old process goes once its last query finishes.
		log.Info().Str("plugin", pluginID).Msg("Plugin version changed, retiring instance")
		delete(m.instances, pluginID)
		if inst.active == 0 {
			inst.process.Kill()
		} else {
			m.retired[inst.process] = inst
		}
		ok = false
	}
	if ok {
		inst.active++
	}
	m.mu.Unlock()

	if ok {
		if !inst.process.Exited() && inst.process.Ping() == nil {
			m.mu.Lock()
			inst.lastUsed = time.Now()
			m.mu.Unlock()
			return inst.process, nil
		}
		log.Warn().Str("plugin", pluginID).Msg("Plugin instance died, restarting")
		m.mu.Lock()
		inst.active--
		if m.instances[pluginID] == inst {
			delete(m.instances, pluginID)
			m.restarts[pluginID]++
		}
		// Another query may still be streaming from this process after a ping that merely timed out, so it is only killed once nobody is using it.
		if inst.active == 0 {
			inst.process.Kill()
		} else {
			m.retired[inst.process] = inst
		}
		m.mu.Unlock()
	}

	process, err := external.Binary().open()
	if err != nil {
		return nil, fmt.Errorf("starting plugin instance %s: %w", pluginID, err)
	}
	m.mu.Lock()
	m.instances[pluginID] = &instance{
		process:   process,
		sha256:    external.Binary().SHA256,
		startedAt: time.Now(),
		lastUsed:  time.Now(),
		active:    1,
	}
	m.mu.Unlock()
	log.Info().Str("plugin", pluginID).Msg("Started long-running plugin instance")
	return process, nil
}

func (m *InstanceManager) startLock(pluginID string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.starting[pluginID]
	if !ok {
		lock = &sync.Mutex{}
		m.starting[pluginID] = lock
	}
	return lock
}

func (m *InstanceManager) release(pluginID string, process *pluginsdk.PluginProcess) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inst, ok := m.instances[pluginID]; ok && inst.process == process && inst.active > 0 {
		inst.active--
		inst.lastUsed = time.Now()
		return
	}
	if inst, ok := m.retired[process]; ok {
		inst.active--
		if inst.active <= 0 {
			process.Kill()
			delete(m.retired, process)
		}
	}
}

// PlanQuery asks the plugin what a statement would touch.
func (m *InstanceManager) PlanQuery(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	source, err := registeredSource(pluginID)
	if err != nil {
		return nil, err
	}
	return m.PlanQueryOn(ctx, pluginID, source, config, req)
}

// PlanQueryOn is PlanQuery against a source the caller resolved, such as one plugin version among several, keeping its instance under key.
func (m *InstanceManager) PlanQueryOn(ctx context.Context, key string, source Source, config map[string]any, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	pluginID := key
	process, err := m.acquire(pluginID, source)
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
	source, err := registeredSource(pluginID)
	if err != nil {
		return nil, err
	}
	return m.ExecuteQueryOn(ctx, pluginID, source, config, req)
}

// ExecuteQueryOn is ExecuteQuery against a source the caller resolved, keeping its instance under key.
func (m *InstanceManager) ExecuteQueryOn(ctx context.Context, key string, source Source, config map[string]any, req pluginsdk.QueryRequest) (pluginsdk.QueryStream, error) {
	pluginID := key
	process, err := m.acquire(pluginID, source)
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
