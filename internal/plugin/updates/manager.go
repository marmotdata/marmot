// Package updates moves core plugins to other versions while Marmot runs, so a security fix in a plugin reaches an install without waiting for a Marmot release. The version each plugin should run at lives in Postgres, so every replica agrees on it; the signed plugin index says which versions exist, which Marmot releases they work with and which have advisories against them. Nothing here touches the network on the startup path: the index is checked in the background and plugins switch over as their new version is installed.
package updates

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/background"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/plugin/index"
	"github.com/marmotdata/marmot/internal/plugin/install"
)

// Policy says which updates Marmot applies by itself.
type Policy string

const (
	// PolicyOff applies nothing; admins move plugins by hand.
	PolicyOff Policy = "off"
	// PolicySecurity moves a plugin off a version with an advisory against it, to the smallest version without one.
	PolicySecurity Policy = "security"
	// PolicyPatch also applies patch releases within the plugin's current minor version.
	PolicyPatch Policy = "patch"
)

// ParsePolicy reads a policy from config, defaulting to security.
func ParsePolicy(s string) (Policy, error) {
	switch Policy(s) {
	case "":
		return PolicySecurity, nil
	case PolicyOff, PolicySecurity, PolicyPatch:
		return Policy(s), nil
	}
	return "", fmt.Errorf("unknown plugin auto-update policy %q (want off, security or patch)", s)
}

// Origins of the version a plugin runs at.
const (
	OriginRelease = "release"
	OriginManual  = "manual"
	OriginAuto    = "auto"
	OriginLocal   = "local"
)

// Errors the API maps to client errors.
var (
	ErrUnknownPlugin  = errors.New("not a core plugin")
	ErrUnknownVersion = errors.New("version is not in the plugin index")
	ErrYanked         = errors.New("version has been yanked")
	ErrIncompatible   = errors.New("version does not support this Marmot release")
	ErrNoIndex        = errors.New("no plugin index has been fetched yet")
	ErrDisabled       = errors.New("plugin updates are disabled")
	ErrNoPrevious     = errors.New("plugin has no previous version to roll back to")
	ErrLocalOverride  = errors.New("plugin is installed locally and overrides managed versions")
)

// Config configures a Manager.
type Config struct {
	DB *pgxpool.Pool
	// Install is how plugins are pulled; the manager supplies Pins itself.
	Install install.Options
	// IndexEnabled turns on checking the plugin index. Off, plugins run the release's pins or whatever an admin set before.
	IndexEnabled bool
	// IndexRef is the OCI reference or https:// URL of the index, defaulting to the index beside the plugins.
	IndexRef      string
	AutoUpdate    Policy
	CheckInterval time.Duration
	// Keys overrides the keys built into the binary, for tests.
	Keys []index.PublicKey
}

// Manager owns which version each core plugin runs at.
type Manager struct {
	cfg      Config
	store    *store
	manifest *install.Manifest
	keys     []index.PublicKey

	mu      sync.RWMutex
	desired map[string]row
	idx     *index.Index
	idxRow  indexRow
	// loaded is the version of each core plugin this replica has registered from the cache.
	loaded map[string]string
	errs   map[string]string

	// switching serialises version switches per plugin, so a slow download of one plugin never holds up another.
	switchMu  sync.Mutex
	switching map[string]*sync.Mutex
	task      *background.SingletonTask
	// install pulls a plugin version and registers it; tests replace it.
	install func(ctx context.Context, name string, pin install.ManifestPlugin) error
}

// New creates a Manager. Call Load before plugins are installed at startup.
func New(cfg Config) (*Manager, error) {
	manifest, err := install.CoreManifest()
	if err != nil {
		return nil, err
	}
	keys := cfg.Keys
	if keys == nil {
		if keys, err = index.TrustedKeys(); err != nil {
			return nil, err
		}
	}
	if cfg.IndexRef == "" {
		if cfg.IndexRef, err = cfg.Install.DefaultIndexRef(); err != nil {
			return nil, err
		}
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 12 * time.Hour
	}
	if cfg.AutoUpdate == "" {
		cfg.AutoUpdate = PolicySecurity
	}
	m := &Manager{
		cfg:       cfg,
		store:     &store{db: cfg.DB},
		manifest:  manifest,
		keys:      keys,
		desired:   map[string]row{},
		loaded:    map[string]string{},
		errs:      map[string]string{},
		switching: map[string]*sync.Mutex{},
	}
	m.install = m.installAndRegister
	return m, nil
}

func (m *Manager) installAndRegister(ctx context.Context, name string, pin install.ManifestPlugin) error {
	path, err := install.EnsureVersion(ctx, m.InstallOptions(), name, pin)
	if err != nil {
		return err
	}
	return plugin.ReplaceBinary(name, path)
}

// InstallOptions returns install options that resolve each plugin to the version it should run at.
func (m *Manager) InstallOptions() install.Options {
	opts := m.cfg.Install
	opts.Pins = m
	return opts
}

// Load reads the desired versions and the stored index from Postgres. It is one query each, so it is cheap enough for the startup path.
func (m *Manager) Load(ctx context.Context) error {
	if err := m.refreshDesired(ctx); err != nil {
		return err
	}
	return m.refreshIndex(ctx)
}

// MarkLoaded records which versions startup registered, so reconciling knows what it is moving from.
func (m *Manager) MarkLoaded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name := range m.manifest.Plugins {
		if install.HasLocalOverride(name) {
			continue
		}
		if _, err := plugin.GetRegistry().Get(name); err != nil {
			continue
		}
		// Startup falls back to the release's version when the chosen one could not be installed; recording what really loaded makes reconciling retry the chosen one.
		pin, _ := m.effectiveLocked(name)
		version := pin.Version
		if !install.Cached(m.InstallOptions(), name, version) {
			version = m.manifest.Plugins[name].Version
		}
		m.loaded[name] = version
	}
}

// Pin implements install.PinSource: the version a plugin should run at when it is not the release's pin.
func (m *Manager) Pin(name string) (install.ManifestPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pin, origin := m.effectiveLocked(name)
	return pin, origin != OriginRelease
}

// effectiveLocked resolves the version a plugin should run at. A stored version wins, except where Marmot has since been upgraded to a release pinning something newer than it, or the index says it does not support this Marmot; a pinned version always wins.
func (m *Manager) effectiveLocked(name string) (install.ManifestPlugin, string) {
	base, hasBase := m.manifest.Plugins[name]
	r, ok := m.desired[name]
	if !ok {
		return base, OriginRelease
	}
	stored := install.ManifestPlugin{Version: r.Version, Digest: r.Digest}
	if !m.trustedLocked(name, r) {
		return base, OriginRelease
	}
	if r.Pinned {
		return stored, r.Source
	}
	if hasBase && r.ReleaseVersion != "" && index.Less(r.ReleaseVersion, base.Version) && index.Less(r.Version, base.Version) {
		return base, OriginRelease
	}
	if m.idx != nil {
		if v, found := m.idx.Plugins[name].Find(r.Version); found && !v.Compatible(plugin.HostVersion()) && hasBase {
			return base, OriginRelease
		}
	}
	return stored, r.Source
}

// trustedLocked reports whether a stored row names a version the signed index or this release vouches for, digest and all. Rows are only ever written from the verified index, so one that does not match was written some other way, such as by hand in the database, and must not choose the code that runs.
func (m *Manager) trustedLocked(name string, r row) bool {
	if base, ok := m.manifest.Plugins[name]; ok && base.Version == r.Version && base.Digest == r.Digest {
		return true
	}
	if m.idx == nil {
		return false
	}
	v, found := m.idx.Plugins[name].Find(r.Version)
	return found && v.Digest == r.Digest
}

func (m *Manager) refreshDesired(ctx context.Context) error {
	rows, err := m.store.list(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.desired = rows
	m.mu.Unlock()
	return nil
}

// refreshIndex reloads the stored index when another replica has stored a newer one. The stored body was verified when it was fetched and is verified again here, so a row written to the database by hand cannot smuggle versions in.
func (m *Manager) refreshIndex(ctx context.Context) error {
	m.mu.RLock()
	current := m.idxRow.Version
	m.mu.RUnlock()

	r, err := m.store.index(ctx, false)
	if err != nil {
		return err
	}
	// Only ever move forward: a stored index older than the one already held is what restoring or planting an old, still validly signed index looks like, and it would hide newer advisories.
	if r.Version > 0 && r.Version < current {
		log.Error().Int64("stored", r.Version).Int64("held", current).Msg("Stored plugin index is older than the one already verified, ignoring it")
		return nil
	}
	var idx *index.Index
	if r.Version > current {
		withBody, err := m.store.index(ctx, true)
		if err != nil {
			return err
		}
		if idx, err = index.Verify(withBody.Body, withBody.Signature, m.keys); err != nil {
			return fmt.Errorf("stored plugin index: %w", err)
		}
		r = withBody
		r.Body, r.Signature = nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if idx != nil {
		m.idx = idx
	}
	m.idxRow = r
	return nil
}

// Start keeps this replica's plugins at their desired versions and, when the index is enabled, checks it periodically.
func (m *Manager) Start(ctx context.Context) {
	go m.listen(ctx)
	go func() {
		// The backstop for a missed notification, such as one sent while the listening connection was being re-established.
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.Reconcile(ctx)
			}
		}
	}()

	if m.cfg.IndexEnabled {
		m.task = background.NewSingletonTask(background.SingletonConfig{
			Name:         "plugin-index-check",
			DB:           m.cfg.DB,
			Interval:     m.cfg.CheckInterval,
			InitialDelay: time.Minute,
			TaskFn: func(ctx context.Context) error {
				_, err := m.Check(ctx, false)
				return err
			},
		})
		m.task.Start(ctx)
	}
}

// Stop stops the index checks.
func (m *Manager) Stop() {
	if m.task != nil {
		m.task.Stop()
	}
}

func (m *Manager) listen(ctx context.Context) {
	for ctx.Err() == nil {
		err := m.listenOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		log.Warn().Err(err).Msg("Plugin version listener stopped, retrying")
		// Jitter keeps replicas that lost their connections together from reconnecting in lockstep.
		wait := 5*time.Second + rand.N(2*time.Second) //nolint:gosec // G404: retry jitter, not a secret
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// listenOnce listens on its own connection rather than one taken from the pool, so it never counts against the pool the rest of Marmot queries through.
func (m *Manager) listenOnce(ctx context.Context) error {
	c, err := pgx.ConnectConfig(ctx, m.cfg.DB.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer c.Close(context.Background())

	if _, err := c.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return err
	}
	for {
		if _, err := c.WaitForNotification(ctx); err != nil {
			return err
		}
		// Changes tend to come in bursts, such as several plugins updated at once, so later notifications in a short window are folded into one reconcile.
		drainCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		for {
			if _, err := c.WaitForNotification(drainCtx); err != nil {
				break
			}
		}
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		m.Reconcile(ctx)
	}
}

// Reconcile moves every plugin this replica has loaded to the version it should run at. Failures are logged and kept for the status page; the plugin stays on the version it had.
func (m *Manager) Reconcile(ctx context.Context) {
	if err := m.Load(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to read plugin versions")
		return
	}

	m.mu.RLock()
	names := make([]string, 0, len(m.loaded))
	for name := range m.loaded {
		names = append(names, name)
	}
	m.mu.RUnlock()
	sort.Strings(names)

	for _, name := range names {
		if err := m.switchTo(ctx, name); err != nil {
			log.Error().Err(err).Str("plugin", name).Msg("Failed to switch plugin version")
		}
	}
}

// EnsureCurrent makes sure a plugin is registered at the version it should run at before a job uses it. It reads that plugin's row afresh, so a replica that missed a notification still never runs a version an admin moved away from.
func (m *Manager) EnsureCurrent(ctx context.Context, name string) error {
	if _, ok := m.manifest.Plugins[name]; !ok {
		return nil
	}
	r, ok, err := m.store.get(ctx, name)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if ok {
		m.desired[name] = r
	} else {
		delete(m.desired, name)
	}
	m.mu.Unlock()
	return m.switchTo(ctx, name)
}

// switchTo installs and registers the version a plugin should run at, if it is not what this replica runs already.
func (m *Manager) switchTo(ctx context.Context, name string) error {
	if install.HasLocalOverride(name) {
		return nil
	}
	m.switchMu.Lock()
	lock, ok := m.switching[name]
	if !ok {
		lock = &sync.Mutex{}
		m.switching[name] = lock
	}
	m.switchMu.Unlock()
	lock.Lock()
	defer lock.Unlock()

	m.mu.RLock()
	want, _ := m.effectiveLocked(name)
	have := m.loaded[name]
	m.mu.RUnlock()

	if have == want.Version {
		if _, err := plugin.GetRegistry().Get(name); err == nil {
			return nil
		}
	}

	// A hung registry must not hold this plugin's switch lock, and so every later switch, forever.
	installCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	err := m.install(installCtx, name, want)
	cancel()

	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.errs[name] = fmt.Sprintf("switching to %s: %v", want.Version, err)
		return err
	}
	delete(m.errs, name)
	m.loaded[name] = want.Version
	if have != "" {
		log.Info().Str("plugin", name).Str("from", have).Str("to", want.Version).Msg("Plugin version switched")
	}
	return nil
}

// Check fetches the index, verifies it and stores it if it is newer than the one stored, then applies the auto-update policy. Unless forced it does nothing when another replica checked within half the interval.
func (m *Manager) Check(ctx context.Context, force bool) (bool, error) {
	if !m.cfg.IndexEnabled {
		return false, ErrDisabled
	}
	if !force {
		r, err := m.store.index(ctx, false)
		if err == nil && r.CheckedAt != nil && time.Since(*r.CheckedAt) < m.cfg.CheckInterval/2 {
			return false, nil
		}
	}

	updated, err := m.fetch(ctx)
	if recErr := m.store.recordCheck(ctx, err); recErr != nil {
		log.Warn().Err(recErr).Msg("Failed to record plugin index check")
	}
	if err != nil {
		log.Warn().Err(err).Str("index", m.cfg.IndexRef).Msg("Plugin index check failed")
		return false, err
	}
	if err := m.refreshIndex(ctx); err != nil {
		return updated, err
	}
	if err := m.autoUpdate(ctx); err != nil {
		return updated, err
	}
	return updated, nil
}

func (m *Manager) fetch(ctx context.Context) (bool, error) {
	body, sig, err := install.FetchIndex(ctx, m.cfg.Install, m.cfg.IndexRef)
	if err != nil {
		return false, err
	}
	return m.accept(ctx, body, sig)
}

// accept verifies a fetched index and stores it if it is newer than the stored one.
func (m *Manager) accept(ctx context.Context, body, sig []byte) (bool, error) {
	idx, err := index.Verify(body, sig, m.keys)
	if err != nil {
		return false, err
	}
	if idx.Expired(time.Now()) {
		return false, fmt.Errorf("plugin index expired at %s", idx.Expires.Format(time.RFC3339))
	}

	stored, err := m.store.indexVersion(ctx)
	if err != nil {
		return false, err
	}
	if idx.Version < stored {
		return false, fmt.Errorf("%w (fetched %d, have %d)", errStaleIndex, idx.Version, stored)
	}
	if idx.Version == stored {
		return false, nil
	}
	saved, err := m.store.saveIndex(ctx, idx.Version, body, sig, idx.Expires)
	if err != nil {
		return false, err
	}
	if saved {
		log.Info().Int64("version", idx.Version).Msg("Plugin index updated")
	}
	return saved, nil
}

// autoUpdate moves unpinned plugins as the policy allows.
func (m *Manager) autoUpdate(ctx context.Context) error {
	if m.cfg.AutoUpdate == PolicyOff {
		return nil
	}
	m.mu.RLock()
	idx := m.idx
	m.mu.RUnlock()
	if idx == nil {
		return nil
	}

	names := make([]string, 0, len(m.manifest.Plugins))
	for name := range m.manifest.Plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	host := plugin.HostVersion()
	for _, name := range names {
		p, ok := idx.Plugins[name]
		if !ok {
			continue
		}
		m.mu.RLock()
		current, _ := m.effectiveLocked(name)
		pinned := m.desired[name].Pinned
		m.mu.RUnlock()
		if pinned {
			continue
		}

		var target index.Version
		found := false
		if m.cfg.AutoUpdate == PolicyPatch {
			target, found = p.LatestPatch(current.Version, host)
		}
		if !found && len(p.Affecting(current.Version)) > 0 {
			target, found = p.SecurityFix(current.Version, host)
		}
		if !found || target.Version == current.Version {
			continue
		}

		log.Info().Str("plugin", name).Str("from", current.Version).Str("to", target.Version).Str("policy", string(m.cfg.AutoUpdate)).Msg("Auto-updating plugin")
		if err := m.put(ctx, name, target, OriginAuto, "auto-update"); err != nil {
			return err
		}
	}
	return nil
}

// SetVersion moves a plugin to a version from the index.
func (m *Manager) SetVersion(ctx context.Context, name, version, by string) error {
	if err := m.manageable(ctx, name); err != nil {
		return err
	}
	m.mu.RLock()
	idx := m.idx
	m.mu.RUnlock()
	if idx == nil {
		return ErrNoIndex
	}
	v, ok := idx.Plugins[name].Find(version)
	if !ok {
		return ErrUnknownVersion
	}
	if v.Yanked {
		return ErrYanked
	}
	if !v.Compatible(plugin.HostVersion()) {
		return ErrIncompatible
	}
	if err := m.put(ctx, name, v, OriginManual, by); err != nil {
		return err
	}
	return m.switchTo(ctx, name)
}

// put stores a plugin's new version, keeping what it ran before for rollback.
func (m *Manager) put(ctx context.Context, name string, v index.Version, source, by string) error {
	m.mu.RLock()
	current, _ := m.effectiveLocked(name)
	existing := m.desired[name]
	m.mu.RUnlock()

	r := row{
		PluginID:        name,
		Version:         v.Version,
		Digest:          v.Digest,
		Source:          source,
		Pinned:          existing.Pinned,
		PreviousVersion: current.Version,
		PreviousDigest:  current.Digest,
		UpdatedBy:       by,
	}
	if err := m.save(ctx, r); err != nil {
		return err
	}
	return nil
}

// SetPinned pins a plugin at the version it runs now, or unpins it. Unpinning a plugin that runs the release's pin forgets it, so it follows the release again.
func (m *Manager) SetPinned(ctx context.Context, name string, pinned bool, by string) error {
	if err := m.manageable(ctx, name); err != nil {
		return err
	}
	m.mu.RLock()
	current, origin := m.effectiveLocked(name)
	r, exists := m.desired[name]
	base := m.manifest.Plugins[name]
	m.mu.RUnlock()

	if !pinned && exists && r.Version == base.Version {
		return m.Reset(ctx, name, by)
	}
	if !exists || origin == OriginRelease {
		r = row{PluginID: name, Version: current.Version, Digest: current.Digest, Source: OriginManual}
	}
	r.Pinned = pinned
	r.UpdatedBy = by
	if err := m.save(ctx, r); err != nil {
		return err
	}
	return m.switchTo(ctx, name)
}

// Rollback moves a plugin back to the version it ran before its last change and pins it there, so the auto-update policy does not move it straight forward again.
func (m *Manager) Rollback(ctx context.Context, name, by string) error {
	if err := m.manageable(ctx, name); err != nil {
		return err
	}
	m.mu.RLock()
	r, ok := m.desired[name]
	m.mu.RUnlock()
	if !ok || r.PreviousVersion == "" {
		return ErrNoPrevious
	}
	r.Version, r.PreviousVersion = r.PreviousVersion, r.Version
	r.Digest, r.PreviousDigest = r.PreviousDigest, r.Digest
	r.Source = OriginManual
	r.Pinned = true
	r.UpdatedBy = by
	if err := m.save(ctx, r); err != nil {
		return err
	}
	return m.switchTo(ctx, name)
}

// Reset returns a plugin to the version this Marmot release pins.
func (m *Manager) Reset(ctx context.Context, name, by string) error {
	if err := m.manageable(ctx, name); err != nil {
		return err
	}
	if err := m.store.delete(ctx, name); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.desired, name)
	m.mu.Unlock()
	log.Info().Str("plugin", name).Str("by", by).Msg("Plugin reset to release version")
	return m.switchTo(ctx, name)
}

// save stores a plugin's row, noting the release's pin at the time so a later Marmot upgrade can tell whether its own pin supersedes the choice.
func (m *Manager) save(ctx context.Context, r row) error {
	r.ReleaseVersion = m.manifest.Plugins[r.PluginID].Version
	if err := m.store.put(ctx, r); err != nil {
		return err
	}
	m.mu.Lock()
	m.desired[r.PluginID] = r
	m.mu.Unlock()
	return nil
}

// manageable checks a plugin's version can be changed here, then reloads the desired versions and index, since another replica may have changed either since this one last looked.
func (m *Manager) manageable(ctx context.Context, name string) error {
	if _, ok := m.manifest.Plugins[name]; !ok {
		return ErrUnknownPlugin
	}
	if install.HasLocalOverride(name) {
		return ErrLocalOverride
	}
	return m.Load(ctx)
}
