//go:build !windows

package plugin

import (
	"fmt"
	"os"
	"syscall"
)

// checkOwnership fails when path is owned by a user other than root or this process, or is writable by others. Group write is tolerated only for this process's own group, which covers Kubernetes fsGroup volumes.
func checkOwnership(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%s: cannot read ownership", path)
	}

	// A root-owned sticky directory such as /tmp is the one safe shared case: nobody else can rename or replace what we put there.
	if info.IsDir() && info.Mode()&os.ModeSticky != 0 && st.Uid == 0 {
		return nil
	}

	mode := info.Mode().Perm()
	switch {
	case st.Uid != 0 && int(st.Uid) != os.Geteuid():
		return fmt.Errorf("%s is owned by uid %d", path, st.Uid)
	case mode&0o002 != 0:
		return fmt.Errorf("%s is world-writable", path)
	case mode&0o020 != 0 && int(st.Gid) != os.Getegid():
		return fmt.Errorf("%s is writable by group %d", path, st.Gid)
	}
	return nil
}
