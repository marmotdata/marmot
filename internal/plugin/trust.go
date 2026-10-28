package plugin

import (
	"fmt"
	"path/filepath"
	"strings"
)

// checkBinaryTrust refuses a plugin binary that another user could have written or swapped. It applies the rule OpenSSH uses for authorized_keys under StrictModes: the file and each directory above it, up to and including the plugin directory it was found in, must be owned by root or by this process's user and must not be writable by anyone else. Symlinks are resolved first, so the rule holds for wherever the binary really lives.
func checkBinaryTrust(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolving plugin binary: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return fmt.Errorf("resolving plugin binary: %w", err)
	}

	stop := trustRoot(resolved)
	for current := resolved; ; current = filepath.Dir(current) {
		if err := checkOwnership(current); err != nil {
			return fmt.Errorf("refusing plugin %s: %w; fix with chmod go-w and make sure it is owned by this user or root", path, err)
		}
		if current == stop || current == filepath.Dir(current) {
			return nil
		}
	}
}

// trustRoot returns the plugin directory containing path, where the ownership walk stops, or the filesystem root when path sits outside every plugin directory
func trustRoot(path string) string {
	for _, dir := range []string{CacheDir(), PluginsDir()} {
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue
		}
		if root, err = filepath.Abs(root); err == nil && strings.HasPrefix(path, root+string(filepath.Separator)) {
			return root
		}
	}
	return filepath.VolumeName(path) + string(filepath.Separator)
}
