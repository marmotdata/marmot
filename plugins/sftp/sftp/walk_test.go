package sftp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// osFS presents a directory on disk through the same interface the SFTP
// connection does, so the walk and the file readers can be exercised
// without a server.
type osFS struct{}

func (osFS) ReadDir(dir string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Lstat, not Stat, so a symlink is reported as a symlink, which is
	// what an SFTP server sends for a directory listing.
	infos := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := os.Lstat(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (osFS) Stat(path string) (os.FileInfo, error)   { return os.Stat(path) }
func (osFS) RealPath(path string) (string, error)    { return filepath.EvalSymlinks(path) }
func (osFS) Open(path string) (io.ReadCloser, error) { return os.Open(path) }

const ordersCSV = `order_id,customer,amount,ordered_on,shipped,note
1,alice,42.50,2026-09-01,true,first order
2,bob,17.00,2026-09-02,false,
3,carol,8.25,2026-09-03,true,rush
`

const eventsJSONL = `{"event_id":1,"kind":"click","actor":{"id":"u1","name":"alice"},"ts":"2026-09-01T10:00:00Z"}
{"event_id":2,"kind":"view","actor":{"id":"u2","name":"bob"},"ts":"2026-09-01T10:05:00Z","score":1.5}
{"event_id":3,"kind":"click","actor":{"id":"u3","name":null},"ts":"2026-09-01T10:09:00Z"}
`

const archiveCSV = `order_id,customer,amount,ordered_on,shipped,note
9,dave,3.00,2026-08-30,false,archived
`

// writeTree lays out the same tree the end to end tests seed on the real
// server: two dated drop folders, an empty staging folder, and a symlink
// pointing back at the root.
func writeTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, root, "incoming/2026-09/orders.csv", ordersCSV)
	writeFile(t, root, "incoming/2026-09/events.jsonl", eventsJSONL)
	writeFile(t, root, "incoming/notes.txt", "operational notes for the incoming feed\n")
	writeFile(t, root, "archive/2026-08/orders.csv", archiveCSV)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "staging"), 0o755))
	require.NoError(t, os.Symlink(root, filepath.Join(root, "loop")))

	return root
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()

	full := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// walkTree runs only the walk over a tree on disk.
func walkTree(t *testing.T, root string, overrides pluginsdk.RawConfig) *walker {
	t.Helper()

	config := testConfig(t, root, overrides)
	w := newWalker(osFS{}, config)
	for _, r := range config.RootDirectories {
		if err := w.walkRoot(context.Background(), r); err != nil {
			t.Fatalf("walking %s: %v", r, err)
		}
	}
	return w
}

func dirNames(w *walker) []string {
	names := make([]string, 0, len(w.directories))
	for _, d := range w.directories {
		names = append(names, d.Name)
	}
	return names
}

func fileNames(w *walker) []string {
	names := make([]string, 0, len(w.files))
	for _, f := range w.files {
		names = append(names, f.Name)
	}
	return names
}

func TestChildName_AtTheTopOfARootIsBare(t *testing.T) {
	assert.Equal(t, "orders.csv", childName("", "orders.csv"))
}

func TestChildName_JoinsWithASlash(t *testing.T) {
	assert.Equal(t, "incoming/2026-09/orders.csv", childName("incoming/2026-09", "orders.csv"))
}

func TestCleanPath_DropsATrailingSlash(t *testing.T) {
	assert.Equal(t, "/data/incoming", cleanPath("/data/incoming/"))
}

func TestCleanPath_LeavesTheServerRootAlone(t *testing.T) {
	assert.Equal(t, "/", cleanPath("/"))
}

func TestNormaliseRoots_DefaultsToTheServerRoot(t *testing.T) {
	assert.Equal(t, []string{"/"}, normaliseRoots(nil))
}

func TestNormaliseRoots_DropsDuplicatesWrittenDifferently(t *testing.T) {
	assert.Equal(t, []string{"/data"}, normaliseRoots([]string{"/data", "/data/", " /data "}))
}

func TestExtension_IsLowercaseWithoutTheDot(t *testing.T) {
	assert.Equal(t, "csv", extension("Orders.CSV"))
}

func TestExtension_IsEmptyWhenThereIsNone(t *testing.T) {
	assert.Equal(t, "", extension("README"))
}

func TestExtension_IsEmptyForAHiddenFile(t *testing.T) {
	assert.Equal(t, "", extension(".gitignore"))
}

func TestWalk_NamesDirectoriesRelativeToTheirRoot(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	assert.ElementsMatch(t,
		[]string{"archive", "archive/2026-08", "incoming", "incoming/2026-09", "staging"},
		dirNames(w))
}

func TestWalk_DoesNotCatalogueTheRootItself(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	assert.NotContains(t, dirNames(w), "")
}

func TestWalk_NamesFilesByTheirDirectory(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	assert.ElementsMatch(t, []string{
		"archive/2026-08/orders.csv",
		"incoming/2026-09/events.jsonl",
		"incoming/2026-09/orders.csv",
		"incoming/notes.txt",
	}, fileNames(w))
}

func TestWalk_NamesAFileAtTheTopOfARootBare(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manifest.csv", "id\n1\n")

	w := walkTree(t, root, nil)

	require.Len(t, w.files, 1)
	assert.Equal(t, "manifest.csv", w.files[0].Name)
	assert.Empty(t, w.files[0].Directory)
}

func TestWalk_RecordsImmediateContentsOfADirectory(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	incoming := dirRecordNamed(t, w, "incoming")
	assert.Equal(t, 1, incoming.FileCount)
	assert.Equal(t, 1, incoming.DirCount)
	assert.Equal(t, int64(len("operational notes for the incoming feed\n")), incoming.SizeBytes)
}

func TestWalk_RecordsAnEmptyDirectory(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	staging := dirRecordNamed(t, w, "staging")
	assert.Equal(t, 0, staging.FileCount)
	assert.Equal(t, 0, staging.DirCount)
	assert.Equal(t, int64(0), staging.SizeBytes)
}

func TestWalk_RecordsTheParentOfANestedDirectory(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	assert.Equal(t, "incoming", dirRecordNamed(t, w, "incoming/2026-09").Parent)
}

func TestWalk_LeavesTheParentEmptyAtTheTopOfARoot(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	assert.Empty(t, dirRecordNamed(t, w, "incoming").Parent)
}

func TestWalk_RecordsAFilesSizeAndMode(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	orders := fileRecordNamed(t, w, "incoming/2026-09/orders.csv")
	assert.Equal(t, int64(len(ordersCSV)), orders.Size)
	assert.Equal(t, "csv", orders.Extension)
	assert.False(t, orders.Modified.IsZero())
	assert.Equal(t, "-rw-r--r--", orders.Mode)
}

func TestWalk_SkipsSymlinksByDefault(t *testing.T) {
	w := walkTree(t, writeTree(t), nil)

	assert.NotContains(t, dirNames(w), "loop")
	assert.Equal(t, 1, w.symlinks, "the symlink should be seen and skipped, not missed")
}

func TestWalk_FollowsSymlinksWhenConfigured(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, outside, "orders.csv", ordersCSV)
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "shortcut")))

	w := walkTree(t, root, pluginsdk.RawConfig{"follow_symlinks": true})

	assert.Contains(t, dirNames(w), "shortcut")
	assert.Contains(t, fileNames(w), "shortcut/orders.csv")
}

func TestWalk_ASymlinkToADirectoryAlreadyInTheTreeIsNotWalkedTwice(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "real/orders.csv", ordersCSV)
	require.NoError(t, os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "shortcut")))

	w := walkTree(t, root, pluginsdk.RawConfig{"follow_symlinks": true})

	assert.Len(t, dirNames(w), 1)
	assert.Len(t, fileNames(w), 1)
}

func TestWalk_ASymlinkLoopDoesNotRepeatDirectories(t *testing.T) {
	// "loop" points back at the root it sits in. Following it naively
	// would walk the whole tree again, and again, forever.
	w := walkTree(t, writeTree(t), pluginsdk.RawConfig{"follow_symlinks": true})

	assert.ElementsMatch(t,
		[]string{"archive", "archive/2026-08", "incoming", "incoming/2026-09", "staging"},
		dirNames(w))
}

func TestWalk_StopsAtMaxDepth(t *testing.T) {
	w := walkTree(t, writeTree(t), pluginsdk.RawConfig{"max_depth": 1})

	assert.ElementsMatch(t, []string{"archive", "incoming", "staging"}, dirNames(w))
	assert.True(t, w.depthWarned)
}

func TestWalk_MaxDepthStillCataloguesFilesAtTheDepthReached(t *testing.T) {
	w := walkTree(t, writeTree(t), pluginsdk.RawConfig{"max_depth": 1})

	assert.Equal(t, []string{"incoming/notes.txt"}, fileNames(w))
}

func TestWalk_StopsAtMaxFiles(t *testing.T) {
	w := walkTree(t, writeTree(t), pluginsdk.RawConfig{"max_files": 2})

	assert.Len(t, w.files, 2)
	assert.True(t, w.stopped)
}

func TestWalk_StructuredOnlyDropsPlainFiles(t *testing.T) {
	w := walkTree(t, writeTree(t), pluginsdk.RawConfig{"structured_only": true})

	assert.NotContains(t, fileNames(w), "incoming/notes.txt")
	assert.Contains(t, fileNames(w), "incoming/2026-09/orders.csv")
}

func TestWalk_StructuredOnlyKeepsTheDirectoryHoldingThem(t *testing.T) {
	w := walkTree(t, writeTree(t), pluginsdk.RawConfig{"structured_only": true})

	assert.Contains(t, dirNames(w), "incoming")
}

func TestWalk_MissingRootIsAnError(t *testing.T) {
	config := testConfig(t, filepath.Join(t.TempDir(), "nowhere"), nil)
	w := newWalker(osFS{}, config)

	err := w.walkRoot(context.Background(), config.RootDirectories[0])

	require.Error(t, err)
}

func TestWalk_OneUnreadableDirectoryDoesNotLoseTheRest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "open/orders.csv", ordersCSV)
	writeFile(t, root, "closed/secret.csv", ordersCSV)
	closed := filepath.Join(root, "closed")
	require.NoError(t, os.Chmod(closed, 0o000))
	t.Cleanup(func() { _ = os.Chmod(closed, 0o755) })

	w := walkTree(t, root, nil)

	assert.Contains(t, dirNames(w), "open")
	assert.Contains(t, fileNames(w), "open/orders.csv")
}

func TestWalk_NestedRootsAreWalkedOnce(t *testing.T) {
	root := writeTree(t)

	w := walkTree(t, root, pluginsdk.RawConfig{
		"root_directories": []string{root, filepath.Join(root, "incoming")},
	})

	assert.ElementsMatch(t,
		[]string{"archive", "archive/2026-08", "incoming", "incoming/2026-09", "staging"},
		dirNames(w))
}

func dirRecordNamed(t *testing.T, w *walker, name string) dirRecord {
	t.Helper()

	for _, d := range w.directories {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("no directory named %q in %v", name, dirNames(w))
	return dirRecord{}
}

func fileRecordNamed(t *testing.T, w *walker, name string) fileRecord {
	t.Helper()

	for _, f := range w.files {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no file named %q in %v", name, fileNames(w))
	return fileRecord{}
}
