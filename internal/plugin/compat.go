package plugin

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

var (
	hostVersionMu sync.RWMutex
	hostVersion   = "dev"
)

// SetHostVersion records the Marmot release this binary was built as, which only the command package knows, so plugin compatibility can be checked against it.
func SetHostVersion(v string) {
	if v == "" {
		return
	}
	hostVersionMu.Lock()
	hostVersion = v
	hostVersionMu.Unlock()
}

// HostVersion returns the Marmot release plugins are checked against, or "dev" for an unversioned build.
func HostVersion() string {
	hostVersionMu.RLock()
	defer hostVersionMu.RUnlock()
	return hostVersion
}

// SupportsMarmot reports whether a plugin's Marmot version constraint admits host. An empty constraint admits every release, and so does a host that is not a release (a dev or snapshot build), since there is nothing meaningful to compare.
func SupportsMarmot(constraint, host string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, fmt.Errorf("invalid marmot version constraint %q: %w", constraint, err)
	}
	v, err := semver.NewVersion(strings.TrimPrefix(host, "v"))
	if err != nil {
		return true, nil
	}
	// Prereleases of the host are compared as the release they lead up to, otherwise ">=0.14.0" would refuse 0.14.0-rc.1.
	if v.Prerelease() != "" {
		release := semver.New(v.Major(), v.Minor(), v.Patch(), "", "")
		v = release
	}
	return c.Check(v), nil
}

// SupportsProtocol reports whether a plugin speaks the host's plugin protocol. Plugins built before the SDK reported it leave it zero, and they all speak version 1.
func SupportsProtocol(protocol int) bool {
	if protocol == 0 {
		protocol = 1
	}
	return protocol == pluginsdk.ProtocolVersion
}

// checkCompatibility refuses a plugin that says it does not work with this Marmot release.
func checkCompatibility(meta *pluginsdk.Meta) error {
	if !SupportsProtocol(meta.ProtocolVersion) {
		return fmt.Errorf("plugin %s speaks protocol %d, this Marmot speaks %d", meta.ID, meta.ProtocolVersion, pluginsdk.ProtocolVersion)
	}
	ok, err := SupportsMarmot(meta.MarmotVersion, HostVersion())
	if err != nil {
		return fmt.Errorf("plugin %s: %w", meta.ID, err)
	}
	if !ok {
		return fmt.Errorf("plugin %s needs Marmot %s, this is %s", meta.ID, meta.MarmotVersion, HostVersion())
	}
	return nil
}
