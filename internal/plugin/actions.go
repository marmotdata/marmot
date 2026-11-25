package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// ErrNoActions is returned for a plugin whose Source does not implement the SDK's Actor.
var ErrNoActions = errors.New("plugin does not support actions")

// ErrNoTriggers is returned for a plugin whose Source does not implement the SDK's Watcher.
var ErrNoTriggers = errors.New("plugin does not support triggers")

// ActionHost runs plugin actions, and the triggers the serving process's policy names, on an InstanceManager's processes. It is a type of its own, rather than methods on InstanceManager, because Marmot's server holds an InstanceManager for data queries: keeping actions off it means nothing in the server, not even a type assertion on its query runner, can reach a local plugin's actions. Only the serving process (`marmot ingest --serve`) builds an ActionHost, and TestOnlyTheServingProcessHostsActions holds that in place.
//
// Each call takes a gate, run against the manifest of the very process that will act, after it is acquired and before the plugin is called. The serving process does its whole local check there, so a plugin replaced by a new version between the check and the call cannot act on a check made against the old one.
type ActionHost struct {
	m *InstanceManager

	mu sync.Mutex
	// manifests caches each process's manifest: Actions must not vary over a process's life, and fetching Meta on every call would add a round trip to every action. A new or restarted process is a new key, so its manifest is fetched afresh.
	manifests map[*pluginsdk.PluginProcess][]pluginsdk.ActionSpec
}

// NewActionHost hosts actions on m's plugin processes.
func NewActionHost(m *InstanceManager) *ActionHost {
	return &ActionHost{m: m, manifests: make(map[*pluginsdk.PluginProcess][]pluginsdk.ActionSpec)}
}

// ActionManifest returns the plugin's action manifest, starting its process if need be.
func (a *ActionHost) ActionManifest(ctx context.Context, pluginID string) ([]pluginsdk.ActionSpec, error) {
	process, err := a.acquire(pluginID)
	if err != nil {
		return nil, err
	}
	defer a.m.release(pluginID, process)
	return a.manifest(ctx, process)
}

// PlanAction asks the plugin what an action would change, once gate has passed the request against the process's own manifest.
func (a *ActionHost) PlanAction(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.ActionRequest, gate func([]pluginsdk.ActionSpec) error) (*pluginsdk.ActionPlan, error) {
	process, err := a.gated(ctx, pluginID, gate)
	if err != nil {
		return nil, err
	}
	defer a.m.release(pluginID, process)
	return process.Source.PlanAction(ctx, pluginsdk.RawConfig(config), req)
}

// ExecuteAction runs an action, once gate has passed the request against the process's own manifest.
func (a *ActionHost) ExecuteAction(ctx context.Context, pluginID string, config map[string]any, req pluginsdk.ActionRequest, gate func([]pluginsdk.ActionSpec) error) (*pluginsdk.ActionResult, error) {
	process, err := a.gated(ctx, pluginID, gate)
	if err != nil {
		return nil, err
	}
	defer a.m.release(pluginID, process)
	return process.Source.ExecuteAction(ctx, pluginsdk.RawConfig(config), req)
}

// gated acquires the plugin's process and runs gate on its manifest, releasing the process again when either fails. gate's error is returned unwrapped, so a gRPC status it carries reaches the caller as it was.
func (a *ActionHost) gated(ctx context.Context, pluginID string, gate func([]pluginsdk.ActionSpec) error) (*pluginsdk.PluginProcess, error) {
	process, err := a.acquire(pluginID)
	if err != nil {
		return nil, err
	}
	specs, err := a.manifest(ctx, process)
	if err == nil {
		err = gate(specs)
	}
	if err != nil {
		a.m.release(pluginID, process)
		return nil, err
	}
	return process, nil
}

func (a *ActionHost) acquire(pluginID string) (*pluginsdk.PluginProcess, error) {
	source, err := registeredSource(pluginID)
	if err != nil {
		return nil, err
	}
	return a.m.acquire(pluginID, source)
}

// manifest returns the process's manifest from its own Meta. Meta.ActionsDigest is never read: the digest a request is checked against is computed by the serving process from Actions, so a plugin cannot report one manifest and act on another. A manifest that fails the SDK's own validation is refused rather than trusted, though Serve in the plugin refuses to start with one.
func (a *ActionHost) manifest(ctx context.Context, process *pluginsdk.PluginProcess) ([]pluginsdk.ActionSpec, error) {
	a.mu.Lock()
	specs, ok := a.manifests[process]
	a.mu.Unlock()
	if ok {
		return specs, nil
	}

	meta, err := process.Source.GetMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the plugin's action manifest: %w", err)
	}
	if !meta.SupportsActions {
		return nil, ErrNoActions
	}
	if err := pluginsdk.ValidateManifest(meta.Actions); err != nil {
		return nil, fmt.Errorf("the plugin's action manifest is invalid: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	// Processes that have exited are dropped here, so the cache holds no more than the live processes and the few just replaced.
	for p := range a.manifests {
		if p.Exited() {
			delete(a.manifests, p)
		}
	}
	a.manifests[process] = meta.Actions
	return meta.Actions, nil
}

// TriggerManifest returns the plugin's trigger manifest, starting its process if need be.
func (a *ActionHost) TriggerManifest(ctx context.Context, pluginID string) ([]pluginsdk.TriggerSpec, error) {
	process, err := a.acquire(pluginID)
	if err != nil {
		return nil, err
	}
	defer a.m.release(pluginID, process)
	return triggerManifest(ctx, process)
}

// WatchTrigger starts a watch once gate has passed the trigger and its parameters against the process's own trigger manifest. The process is held until the stream is closed, so it is not reaped as idle while it watches.
func (a *ActionHost) WatchTrigger(ctx context.Context, pluginID string, config map[string]any, trigger string, params map[string]any, gate func([]pluginsdk.TriggerSpec) error) (pluginsdk.TriggerStream, error) {
	process, err := a.acquire(pluginID)
	if err != nil {
		return nil, err
	}
	specs, err := triggerManifest(ctx, process)
	if err == nil {
		err = gate(specs)
	}
	var stream pluginsdk.TriggerStream
	if err == nil {
		stream, err = process.Source.WatchTrigger(ctx, pluginsdk.RawConfig(config), trigger, params)
	}
	if err != nil {
		a.m.release(pluginID, process)
		return nil, err
	}
	return &heldStream{TriggerStream: stream, release: func() { a.m.release(pluginID, process) }}, nil
}

// triggerManifest reads a process's trigger manifest from its Meta. Like actions, the digest is computed by the serving process, never read from Meta.
func triggerManifest(ctx context.Context, process *pluginsdk.PluginProcess) ([]pluginsdk.TriggerSpec, error) {
	meta, err := process.Source.GetMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the plugin's trigger manifest: %w", err)
	}
	if !meta.SupportsTriggers {
		return nil, ErrNoTriggers
	}
	if err := pluginsdk.ValidateTriggers(meta.Triggers); err != nil {
		return nil, fmt.Errorf("the plugin's trigger manifest is invalid: %w", err)
	}
	return meta.Triggers, nil
}

// heldStream releases its plugin process once, when it is closed.
type heldStream struct {
	pluginsdk.TriggerStream
	release func()
	once    sync.Once
}

func (s *heldStream) Close() error {
	err := s.TriggerStream.Close()
	s.once.Do(s.release)
	return err
}
