package updates

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/plugin/index"
	"github.com/marmotdata/marmot/internal/plugin/install"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

// testPlugin is a core plugin from the embedded manifest, so the tests run against a real release pin.
const testPlugin = "kafka"

type harness struct {
	t        *testing.T
	m        *Manager
	priv     ed25519.PrivateKey
	base     install.ManifestPlugin
	installs []string
}

func newHarness(t *testing.T, policy Policy) *harness {
	t.Helper()
	t.Setenv(plugin.PluginsDirEnvVar, t.TempDir())
	plugin.SetHostVersion("0.15.0")
	t.Cleanup(func() { plugin.SetHostVersion("dev") })

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	m, err := New(Config{
		DB:           pgtest.TempDB(t),
		IndexEnabled: true,
		AutoUpdate:   policy,
		Keys:         []index.PublicKey{{ID: "test", Key: pub}},
	})
	require.NoError(t, err)

	h := &harness{t: t, m: m, priv: priv, base: m.manifest.Plugins[testPlugin]}
	m.install = func(_ context.Context, name string, pin install.ManifestPlugin) error {
		h.installs = append(h.installs, name+"@"+pin.Version)
		plugin.GetRegistry().Replace(pluginsdk.Meta{ID: name, Name: name}, nil)
		return nil
	}
	// Startup registered the plugin at the release's pin.
	plugin.GetRegistry().Replace(pluginsdk.Meta{ID: testPlugin, Name: testPlugin}, nil)
	require.NoError(t, m.Load(context.Background()))
	m.MarkLoaded()
	return h
}

// publish signs an index listing the given versions of the test plugin, which sit around the release's pin, and has the manager accept it.
func (h *harness) publish(version int64, versions []index.Version, advisories []index.Advisory) (bool, error) {
	h.t.Helper()
	body, err := json.Marshal(index.Index{
		Version: version,
		Expires: time.Now().Add(time.Hour),
		Plugins: map[string]index.Plugin{testPlugin: {Versions: versions, Advisories: advisories}},
	})
	require.NoError(h.t, err)
	sig, err := index.Sign(body, "test", h.priv)
	require.NoError(h.t, err)
	ok, err := h.m.accept(context.Background(), body, sig)
	if err == nil {
		require.NoError(h.t, h.m.refreshIndex(context.Background()))
	}
	return ok, err
}

func (h *harness) status() PluginStatus {
	h.t.Helper()
	st, err := h.m.Status(context.Background())
	require.NoError(h.t, err)
	for _, p := range st.Plugins {
		if p.ID == testPlugin {
			return p
		}
	}
	h.t.Fatalf("%s missing from status", testPlugin)
	return PluginStatus{}
}

func versionsAround(base install.ManifestPlugin) []index.Version {
	return []index.Version{
		{Version: base.Version, Digest: base.Digest},
		{Version: "99.0.1", Digest: "sha256:fix"},
		{Version: "99.0.2", Digest: "sha256:patch"},
		{Version: "99.1.0", Digest: "sha256:minor", MarmotVersion: ">=0.20.0"},
	}
}

func TestSecurityPolicyMovesOffVulnerableVersion(t *testing.T) {
	h := newHarness(t, PolicySecurity)
	ctx := context.Background()

	_, err := h.publish(1, versionsAround(h.base), nil)
	require.NoError(t, err)
	require.NoError(t, h.m.autoUpdate(ctx))
	assert.Equal(t, h.base.Version, h.status().Version, "no advisory, so the security policy leaves it")

	advisory := index.Advisory{ID: "MPA-1", Severity: "high", Affected: "<99.0.1", Fixed: "99.0.1"}
	_, err = h.publish(2, versionsAround(h.base), []index.Advisory{advisory})
	require.NoError(t, err)
	assert.Len(t, h.status().Advisories, 1)

	require.NoError(t, h.m.autoUpdate(ctx))
	st := h.status()
	assert.Equal(t, "99.0.1", st.Version, "smallest step off the vulnerable version")
	assert.Equal(t, OriginAuto, st.Origin)
	assert.Equal(t, h.base.Version, st.PreviousVersion)
	assert.Empty(t, st.Advisories)
	assert.Equal(t, "99.0.2", st.Latest, "99.1.0 needs a newer Marmot")
	assert.True(t, st.UpdateAvailable)

	h.m.Reconcile(ctx)
	assert.Contains(t, h.installs, testPlugin+"@99.0.1")
}

func TestPatchPolicyAndPinning(t *testing.T) {
	h := newHarness(t, PolicyPatch)
	ctx := context.Background()

	_, err := h.publish(1, versionsAround(h.base), nil)
	require.NoError(t, err)
	require.NoError(t, h.m.SetVersion(ctx, testPlugin, "99.0.1", "alice"))
	assert.Equal(t, OriginManual, h.status().Origin)

	require.NoError(t, h.m.autoUpdate(ctx))
	assert.Equal(t, "99.0.2", h.status().Version)

	require.NoError(t, h.m.Rollback(ctx, testPlugin, "alice"))
	st := h.status()
	assert.Equal(t, "99.0.1", st.Version)
	assert.True(t, st.Pinned, "a rollback pins so auto-update does not undo it")

	require.NoError(t, h.m.autoUpdate(ctx))
	assert.Equal(t, "99.0.1", h.status().Version, "pinned plugins are left alone")

	require.NoError(t, h.m.Reset(ctx, testPlugin, "alice"))
	st = h.status()
	assert.Equal(t, h.base.Version, st.Version)
	assert.Equal(t, OriginRelease, st.Origin)
}

func TestSetVersionRefusals(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx := context.Background()

	assert.ErrorIs(t, h.m.SetVersion(ctx, testPlugin, "99.0.1", "alice"), ErrNoIndex)

	versions := versionsAround(h.base)
	versions = append(versions, index.Version{Version: "99.0.3", Digest: "sha256:bad", Yanked: true})
	_, err := h.publish(1, versions, nil)
	require.NoError(t, err)

	assert.ErrorIs(t, h.m.SetVersion(ctx, testPlugin, "1.2.3", "alice"), ErrUnknownVersion)
	assert.ErrorIs(t, h.m.SetVersion(ctx, testPlugin, "99.0.3", "alice"), ErrYanked)
	assert.ErrorIs(t, h.m.SetVersion(ctx, testPlugin, "99.1.0", "alice"), ErrIncompatible)
	assert.ErrorIs(t, h.m.SetVersion(ctx, "not-a-plugin", "99.0.1", "alice"), ErrUnknownPlugin)
	assert.ErrorIs(t, h.m.Rollback(ctx, testPlugin, "alice"), ErrNoPrevious)
}

func TestOlderIndexIsRefused(t *testing.T) {
	h := newHarness(t, PolicyOff)

	saved, err := h.publish(5, versionsAround(h.base), nil)
	require.NoError(t, err)
	assert.True(t, saved)

	_, err = h.publish(4, versionsAround(h.base), nil)
	assert.ErrorIs(t, err, errStaleIndex)

	saved, err = h.publish(5, versionsAround(h.base), nil)
	require.NoError(t, err)
	assert.False(t, saved, "the same index again is not news")
}

func TestReleasePinIsAFloorAfterUpgrade(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx := context.Background()
	_, err := h.publish(1, append(versionsAround(h.base), index.Version{Version: "0.0.1", Digest: "sha256:old"}), nil)
	require.NoError(t, err)

	// A version chosen under an older release, which this release's newer pin supersedes.
	require.NoError(t, h.m.store.put(ctx, row{PluginID: testPlugin, Version: "0.0.1", Digest: "sha256:old", Source: OriginManual, ReleaseVersion: "0.0.1"}))
	st := h.status()
	assert.Equal(t, h.base.Version, st.Version)
	assert.Equal(t, OriginRelease, st.Origin)

	require.NoError(t, h.m.store.put(ctx, row{PluginID: testPlugin, Version: "0.0.1", Digest: "sha256:old", Source: OriginManual, ReleaseVersion: "0.0.1", Pinned: true}))
	assert.Equal(t, "0.0.1", h.status().Version, "a pinned version holds through an upgrade")
}

func TestExplicitDowngradeHolds(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx := context.Background()

	_, err := h.publish(1, append(versionsAround(h.base), index.Version{Version: "0.0.1", Digest: "sha256:old"}), nil)
	require.NoError(t, err)
	require.NoError(t, h.m.SetVersion(ctx, testPlugin, "0.0.1", "alice"))

	st := h.status()
	assert.Equal(t, "0.0.1", st.Version, "a version chosen under this release holds even below its pin")
	assert.Equal(t, OriginManual, st.Origin)
	assert.Contains(t, h.installs, testPlugin+"@0.0.1")
}

func TestSetVersionSeesIndexAnotherReplicaStored(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx := context.Background()

	body, err := json.Marshal(index.Index{Version: 9, Expires: time.Now().Add(time.Hour), Plugins: map[string]index.Plugin{testPlugin: {Versions: versionsAround(h.base)}}})
	require.NoError(t, err)
	sig, err := index.Sign(body, "test", h.priv)
	require.NoError(t, err)
	// Stored straight into the database, as another replica's check would, without this manager hearing of it.
	_, err = h.m.store.saveIndex(ctx, 9, body, sig, time.Now().Add(time.Hour))
	require.NoError(t, err)

	require.NoError(t, h.m.SetVersion(ctx, testPlugin, "99.0.1", "alice"))
	assert.Equal(t, "99.0.1", h.status().Version)
}

func TestNotificationSwitchesPlugin(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	switched := make(chan string, 4)
	register := h.m.install
	h.m.install = func(ctx context.Context, name string, pin install.ManifestPlugin) error {
		mu.Lock()
		defer mu.Unlock()
		switched <- pin.Version
		return register(ctx, name, pin)
	}
	_, err := h.publish(1, versionsAround(h.base), nil)
	require.NoError(t, err)
	go h.m.listen(ctx)
	// Give LISTEN a moment to be registered before another replica writes.
	time.Sleep(500 * time.Millisecond)

	other := &store{db: h.m.cfg.DB}
	for _, v := range []index.Version{versionsAround(h.base)[1], versionsAround(h.base)[2]} {
		require.NoError(t, other.put(ctx, row{PluginID: testPlugin, Version: v.Version, Digest: v.Digest, Source: OriginManual}))
	}

	select {
	case v := <-switched:
		assert.Equal(t, "99.0.2", v, "a burst of changes is folded into one switch to the latest")
	case <-time.After(10 * time.Second):
		t.Fatal("no switch after another replica changed the version")
	}
}

func TestRowNotInSignedIndexIsIgnored(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx := context.Background()
	_, err := h.publish(1, versionsAround(h.base), nil)
	require.NoError(t, err)

	// Written straight into the database: a real version, but a digest the index does not vouch for.
	require.NoError(t, h.m.store.put(ctx, row{PluginID: testPlugin, Version: "99.0.1", Digest: "sha256:planted", Source: OriginManual, Pinned: true}))
	st := h.status()
	assert.Equal(t, h.base.Version, st.Version)
	assert.Equal(t, OriginRelease, st.Origin)
	assert.Contains(t, st.Error, "not in the signed plugin index")

	h.m.Reconcile(ctx)
	assert.NotContains(t, h.installs, testPlugin+"@99.0.1")
}

func TestOlderStoredIndexIsIgnored(t *testing.T) {
	h := newHarness(t, PolicyOff)
	ctx := context.Background()

	advisory := index.Advisory{ID: "MPA-1", Severity: "high", Affected: "<99.0.1", Fixed: "99.0.1"}
	_, err := h.publish(5, versionsAround(h.base), []index.Advisory{advisory})
	require.NoError(t, err)

	// An older, validly signed index without the advisory, planted over the stored one.
	body, err := json.Marshal(index.Index{Version: 4, Expires: time.Now().Add(time.Hour), Plugins: map[string]index.Plugin{testPlugin: {Versions: versionsAround(h.base)}}})
	require.NoError(t, err)
	sig, err := index.Sign(body, "test", h.priv)
	require.NoError(t, err)
	_, err = h.m.cfg.DB.Exec(ctx, `UPDATE core_plugin_index SET version = 4, body = $1, signature = $2`, body, sig)
	require.NoError(t, err)

	require.NoError(t, h.m.Load(ctx))
	assert.Len(t, h.status().Advisories, 1, "the advisory from the newer index still applies")
}
