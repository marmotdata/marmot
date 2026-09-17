package sftp

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// dirRecord is one directory the walk found.
type dirRecord struct {
	// Name is the asset name: the path below the root it was reached
	// from, with no leading slash. The root itself has no name.
	Name string
	// Path is the absolute path on the server.
	Path string
	// Parent is the asset name of the directory above it, empty for a
	// directory sitting at the top of a root.
	Parent   string
	Root     string
	Modified time.Time
	// FileCount, DirCount and SizeBytes describe the immediate contents,
	// not the whole subtree, and count everything present rather than
	// only what was catalogued.
	FileCount int
	DirCount  int
	SizeBytes int64
}

// fileRecord is one file the walk found.
type fileRecord struct {
	Name string
	Path string
	// Directory is the asset name of the folder holding the file, empty
	// for a file sitting at the top of a root.
	Directory string
	Root      string
	Size      int64
	Modified  time.Time
	Mode      string
	OwnerUID  uint32
	OwnerGID  uint32
	HasOwner  bool
	Extension string
}

// walker turns a server's directory tree into flat lists of directories
// and files, bounded by the configured depth and file limits.
type walker struct {
	fs     fileSystem
	config *Config

	directories []dirRecord
	files       []fileRecord

	// visited holds the canonical path of every directory already walked,
	// so a symlink pointing back up the tree, or one root nested inside
	// another, cannot make the walk repeat itself or run forever.
	visited map[string]bool

	// stopped is set once max_files is reached; the walk then unwinds
	// without listing anything more.
	stopped     bool
	depthWarned bool
	symlinks    int
}

func newWalker(fs fileSystem, config *Config) *walker {
	return &walker{fs: fs, config: config, visited: make(map[string]bool)}
}

// walkRoot walks one configured root. The root itself is not catalogued:
// the directories directly below it are the top of the tree.
func (w *walker) walkRoot(ctx context.Context, root string) error {
	if w.stopped {
		return nil
	}

	if w.markVisited(root) {
		log.Debug().Str("root", root).Msg("Root already walked as part of another root")
		return nil
	}

	return w.walk(ctx, entry{path: root}, "", root, 0)
}

// walk lists one directory and recurses into the directories below it.
// dir.info is nil for a configured root, which is walked but never
// catalogued.
func (w *walker) walk(ctx context.Context, dir entry, name, root string, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	entries, err := w.fs.ReadDir(dir.path)
	if err != nil {
		return fmt.Errorf("listing %s: %w", dir.path, err)
	}

	// Servers list in whatever order suits them, so a run's output would
	// otherwise change between runs over the same tree.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	record := dirRecord{Name: name, Path: dir.path, Root: root}
	if parent := path.Dir(name); name != "" && parent != "." {
		record.Parent = parent
	}
	if dir.info != nil {
		record.Modified = dir.info.ModTime()
	}

	var files []entry
	var subdirs []entry

	for _, e := range entries {
		child, ok := w.resolve(dir.path, e)
		if !ok {
			continue
		}

		switch {
		case child.info.IsDir():
			record.DirCount++
			subdirs = append(subdirs, child)
		case child.info.Mode().IsRegular():
			record.FileCount++
			record.SizeBytes += child.info.Size()
			files = append(files, child)
		default:
			// Sockets, pipes and devices hold no data to catalogue.
		}
	}

	if name != "" {
		w.directories = append(w.directories, record)
	}

	for _, f := range files {
		if len(w.files) >= w.config.MaxFiles {
			log.Warn().Int("max_files", w.config.MaxFiles).Msg("Reached max_files, stopping the walk")
			w.stopped = true
			return nil
		}
		if w.config.StructuredOnly && !isStructured(f.info.Name()) {
			continue
		}
		w.files = append(w.files, w.fileRecord(f, name, root))
	}

	for _, d := range subdirs {
		if w.stopped {
			return nil
		}
		if depth+1 > w.config.MaxDepth {
			if !w.depthWarned {
				log.Warn().Int("max_depth", w.config.MaxDepth).Str("directory", d.path).Msg("Reached max_depth, not walking deeper")
				w.depthWarned = true
			}
			continue
		}
		if w.markVisited(d.path) {
			log.Debug().Str("directory", d.path).Msg("Already walked, skipping")
			continue
		}
		if err := w.walk(ctx, d, childName(name, d.info.Name()), root, depth+1); err != nil {
			// One unreadable directory should not lose the rest of the tree.
			log.Warn().Err(err).Str("directory", d.path).Msg("Failed to list directory")
		}
	}

	return nil
}

// entry is a directory entry paired with the path it sits at. For a
// followed symlink, info describes the target rather than the link.
type entry struct {
	path string
	info os.FileInfo
}

// resolve decides what an entry is. A symlink is skipped unless
// follow_symlinks is on, in which case what it points at decides.
func (w *walker) resolve(dir string, info os.FileInfo) (entry, bool) {
	childPath := path.Join(dir, info.Name())

	if info.Mode()&os.ModeSymlink == 0 {
		return entry{path: childPath, info: info}, true
	}

	w.symlinks++
	if !w.config.FollowSymlinks {
		log.Debug().Str("path", childPath).Msg("Skipping symlink, follow_symlinks is off")
		return entry{}, false
	}

	target, err := w.fs.Stat(childPath)
	if err != nil {
		log.Warn().Err(err).Str("path", childPath).Msg("Failed to resolve symlink")
		return entry{}, false
	}

	// The link's own name is what the tree calls it, so keep it rather
	// than the target's.
	return entry{path: childPath, info: renamed{FileInfo: target, name: info.Name()}}, true
}

// renamed presents a symlink target under the link's own name.
type renamed struct {
	os.FileInfo
	name string
}

func (r renamed) Name() string { return r.name }

// markVisited records a directory by its canonical path and reports
// whether it had already been seen.
func (w *walker) markVisited(dir string) bool {
	canonical, err := w.fs.RealPath(dir)
	if err != nil {
		// Without a canonical path the raw one still catches the common
		// case of a link pointing straight back at its own tree.
		canonical = dir
	}
	if w.visited[canonical] {
		return true
	}
	w.visited[canonical] = true
	return false
}

func (w *walker) fileRecord(f entry, directory, root string) fileRecord {
	uid, gid, hasOwner := ownerIDs(f.info)

	return fileRecord{
		Name:      childName(directory, f.info.Name()),
		Path:      f.path,
		Directory: directory,
		Root:      root,
		Size:      f.info.Size(),
		Modified:  f.info.ModTime(),
		Mode:      f.info.Mode().String(),
		OwnerUID:  uid,
		OwnerGID:  gid,
		HasOwner:  hasOwner,
		Extension: extension(f.info.Name()),
	}
}

// childName is how a directory or file below another one is named: the
// parent's name and its own, joined by a slash. Entries at the top of a
// root have no parent name, so they are named bare.
func childName(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// cleanPath normalises a path so the same directory written two ways
// resolves to one string.
func cleanPath(p string) string {
	if p == "" {
		return ""
	}
	return path.Clean(p)
}

// extension is a file's lowercase extension without the dot, empty when
// the name has none. A leading-dot name like ".gitignore" has no
// extension, it is just a hidden file.
func extension(name string) string {
	ext := path.Ext(name)
	if ext == name {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}
