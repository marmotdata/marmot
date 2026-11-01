//go:build !windows

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginTree makes <tmp>/plugins/marmot-plugin-x with safe permissions and points MARMOT_PLUGINS_DIR at it.
func pluginTree(t *testing.T) (dir, bin string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "plugins")
	require.NoError(t, os.Mkdir(dir, 0o755))
	bin = filepath.Join(dir, "marmot-plugin-x")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
	t.Setenv(PluginsDirEnvVar, dir)
	return dir, bin
}

func TestCheckBinaryTrustAcceptsSafeTree(t *testing.T) {
	_, bin := pluginTree(t)
	assert.NoError(t, checkBinaryTrust(bin))
}

func TestCheckBinaryTrustStopsAtPluginRoot(t *testing.T) {
	dir, bin := pluginTree(t)
	// The parent of the plugin root plays the part of a world-writable emptyDir mount.
	require.NoError(t, os.Chmod(filepath.Dir(dir), 0o777))
	assert.NoError(t, checkBinaryTrust(bin))
}

func TestCheckBinaryTrustRefusesWritableBinary(t *testing.T) {
	_, bin := pluginTree(t)
	require.NoError(t, os.Chmod(bin, 0o757))
	err := checkBinaryTrust(bin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "world-writable")
}

func TestCheckBinaryTrustRefusesWritablePluginDir(t *testing.T) {
	dir, bin := pluginTree(t)
	require.NoError(t, os.Chmod(dir, 0o777))
	assert.Error(t, checkBinaryTrust(bin))
}

func TestCheckBinaryTrustRefusesForeignGroupWrite(t *testing.T) {
	_, bin := pluginTree(t)
	require.NoError(t, os.Chmod(bin, 0o775))

	groups, err := os.Getgroups()
	require.NoError(t, err)
	foreign := -1
	for _, g := range groups {
		if g != os.Getegid() {
			foreign = g
			break
		}
	}
	if foreign < 0 {
		t.Skip("no supplementary group to hand the file to")
	}
	require.NoError(t, os.Chown(bin, -1, foreign))

	err = checkBinaryTrust(bin)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writable by group")
}

func TestCheckBinaryTrustAllowsOwnGroupWrite(t *testing.T) {
	_, bin := pluginTree(t)
	require.NoError(t, os.Chmod(bin, 0o775))
	assert.NoError(t, checkBinaryTrust(bin))
}

// A symlink in a safe directory must not launder a binary that lives somewhere writable.
func TestCheckBinaryTrustFollowsSymlinks(t *testing.T) {
	dir, _ := pluginTree(t)
	unsafeDir := filepath.Join(t.TempDir(), "drop")
	require.NoError(t, os.Mkdir(unsafeDir, 0o777))
	require.NoError(t, os.Chmod(unsafeDir, 0o777))
	target := filepath.Join(unsafeDir, "evil")
	require.NoError(t, os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755))

	link := filepath.Join(dir, "marmot-plugin-linked")
	require.NoError(t, os.Symlink(target, link))

	assert.Error(t, checkBinaryTrust(link))
}

func TestRegisterExternalPluginRefusesUntrustedBinary(t *testing.T) {
	_, bin := pluginTree(t)
	require.NoError(t, os.Chmod(bin, 0o757))

	registered, err := registerExternalPlugin(bin)
	assert.False(t, registered)
	assert.Error(t, err)
}

func TestCheckOwnershipStickyDirectory(t *testing.T) {
	info, err := os.Stat("/tmp")
	if err != nil || info.Mode()&os.ModeSticky == 0 {
		t.Skip("/tmp is not a sticky directory here")
	}
	assert.NoError(t, checkOwnership("/tmp"), "root-owned sticky /tmp is safe")

	sticky := filepath.Join(t.TempDir(), "sticky")
	require.NoError(t, os.Mkdir(sticky, 0o777))
	require.NoError(t, os.Chmod(sticky, 0o777|os.ModeSticky))
	assert.Error(t, checkOwnership(sticky), "a sticky directory owned by an ordinary user is not")
}
