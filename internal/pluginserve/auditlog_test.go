package pluginserve

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func appendEntries(t *testing.T, path string, queryIDs ...string) {
	t.Helper()
	l, err := OpenAuditLog(path)
	require.NoError(t, err)
	for _, id := range queryIDs {
		require.NoError(t, l.Append(AuditEntry{Event: AuditExecute, QueryID: id, Policy: "none"}))
	}
	require.NoError(t, l.Close())
}

func TestAuditLogChainsAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	appendEntries(t, path, "q1", "q2")
	appendEntries(t, path, "q3")

	n, err := VerifyAuditLog(path)
	require.NoError(t, err)
	assert.Equal(t, 3, n, "a restarted process continues the chain from the last line")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestAuditLogTightensAnExistingFilesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	require.NoError(t, os.WriteFile(path, nil, 0o644)) //nolint:gosec // G306: the test checks the mode is tightened
	appendEntries(t, path, "q1")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestVerifyAuditLogDetectsTampering(t *testing.T) {
	lines := func(t *testing.T) (string, [][]byte) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "audit.log")
		appendEntries(t, path, "q1", "q2", "q3", "q4")
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return path, bytes.SplitAfter(data, []byte("\n"))[:4]
	}
	write := func(t *testing.T, path string, ls [][]byte) {
		t.Helper()
		require.NoError(t, os.WriteFile(path, bytes.Join(ls, nil), 0o600))
	}

	t.Run("an edited line", func(t *testing.T) {
		path, ls := lines(t)
		ls[1] = bytes.Replace(ls[1], []byte(`"q2"`), []byte(`"qX"`), 1)
		write(t, path, ls)
		n, err := VerifyAuditLog(path)
		assert.ErrorIs(t, err, ErrAuditChainBroken)
		assert.Contains(t, err.Error(), "line 3")
		assert.Equal(t, 2, n)
	})

	t.Run("a removed line", func(t *testing.T) {
		path, ls := lines(t)
		write(t, path, append(ls[:1:1], ls[2:]...))
		_, err := VerifyAuditLog(path)
		assert.ErrorIs(t, err, ErrAuditChainBroken)
		assert.Contains(t, err.Error(), "line 2")
	})

	t.Run("a removed first line", func(t *testing.T) {
		path, ls := lines(t)
		write(t, path, ls[1:])
		_, err := VerifyAuditLog(path)
		assert.ErrorIs(t, err, ErrAuditChainBroken)
		assert.Contains(t, err.Error(), "line 1")
	})

	t.Run("lines appended by hand", func(t *testing.T) {
		path, ls := lines(t)
		write(t, path, append(ls, []byte(`{"event":"execute","prev":""}`+"\n")))
		_, err := VerifyAuditLog(path)
		assert.ErrorIs(t, err, ErrAuditChainBroken)
	})
}

func TestOpenAuditLogRefusesAPartialLastLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	appendEntries(t, path, "q1")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString(`{"event":"exec`)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, err = OpenAuditLog(path)
	assert.ErrorContains(t, err, "partial line")
}

func TestOpenAuditLogReadsALastLineAcrossBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	l, err := OpenAuditLog(path)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		// Longer than lastLine's read block, so the last line spans more than one.
		require.NoError(t, l.Append(AuditEntry{Event: AuditExecute, Error: string(bytes.Repeat([]byte("x"), 5000))}))
	}
	require.NoError(t, l.Close())
	appendEntries(t, path, "q4")

	n, err := VerifyAuditLog(path)
	require.NoError(t, err)
	assert.Equal(t, 4, n)
}
