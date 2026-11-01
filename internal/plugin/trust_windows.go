//go:build windows

package plugin

// checkOwnership is a no-op on Windows, which has no Unix ownership bits; access to plugin directories there is governed by ACLs the installer sets.
func checkOwnership(string) error { return nil }
