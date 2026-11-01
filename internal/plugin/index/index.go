// Package index reads the signed plugin index: every published version of each core plugin, the Marmot releases each works with and the security advisories against them. Marmot trusts it only once its signature checks out against a key built into the binary, which keeps the property the embedded release pins give: whoever controls the registry or a mirror cannot choose the code that runs.
package index

import (
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/marmotdata/marmot/internal/plugin"
)

// Index lists every published version of the core plugins.
type Index struct {
	// Version increases with every index published. Marmot refuses an index older than one it has already seen, so a replayed index cannot roll plugins back to a vulnerable version.
	Version   int64     `json:"version"`
	Generated time.Time `json:"generated"`
	// Expires bounds how long a captured index can be replayed to hide newer advisories. The index is republished well inside it.
	Expires time.Time         `json:"expires"`
	Plugins map[string]Plugin `json:"plugins"`
}

// Plugin is one plugin's published versions and the advisories against them.
type Plugin struct {
	Versions   []Version  `json:"versions"`
	Advisories []Advisory `json:"advisories,omitempty"`
}

// Version is one published version of a plugin.
type Version struct {
	Version string `json:"version"`
	// Digest is the sha256 of the version's OCI image index, which is what gets installed.
	Digest string `json:"digest"`
	// MarmotVersion is the semver constraint on the Marmot releases this version works with; empty means every release.
	MarmotVersion   string     `json:"marmot_version,omitempty"`
	ProtocolVersion int        `json:"protocol_version,omitempty"`
	Released        *time.Time `json:"released,omitempty"`
	ChangelogURL    string     `json:"changelog_url,omitempty"`
	// Yanked versions are never offered or installed, though a Marmot already running one keeps it until it moves.
	Yanked       bool   `json:"yanked,omitempty"`
	YankedReason string `json:"yanked_reason,omitempty"`
}

// Advisory is a security advisory against some versions of a plugin. It is only published once a fixed version is.
type Advisory struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	// Affected is the semver constraint on the plugin versions the advisory applies to, e.g. "<0.3.1".
	Affected  string     `json:"affected"`
	Fixed     string     `json:"fixed"`
	Summary   string     `json:"summary"`
	URL       string     `json:"url,omitempty"`
	Published *time.Time `json:"published,omitempty"`
} // @name PluginAdvisory

// Expired reports whether the index is past its expiry at now.
func (i *Index) Expired(now time.Time) bool {
	return !i.Expires.IsZero() && now.After(i.Expires)
}

// Find returns the named version of a plugin.
func (p Plugin) Find(version string) (Version, bool) {
	for _, v := range p.Versions {
		if v.Version == version {
			return v, true
		}
	}
	return Version{}, false
}

// Compatible reports whether this version works with the given Marmot release.
func (v Version) Compatible(marmotVersion string) bool {
	if !plugin.SupportsProtocol(v.ProtocolVersion) {
		return false
	}
	ok, err := plugin.SupportsMarmot(v.MarmotVersion, marmotVersion)
	return err == nil && ok
}

// Affecting returns the advisories that apply to a version of the plugin.
func (p Plugin) Affecting(version string) []Advisory {
	v, err := semver.NewVersion(version)
	if err != nil {
		return nil
	}
	var out []Advisory
	for _, a := range p.Advisories {
		c, err := semver.NewConstraint(a.Affected)
		if err != nil {
			continue
		}
		if c.Check(v) {
			out = append(out, a)
		}
	}
	return out
}

// Installable returns the versions that can be installed on the given Marmot release, newest first: compatible and not yanked.
func (p Plugin) Installable(marmotVersion string) []Version {
	var out []Version
	for _, v := range p.Versions {
		if v.Yanked || !v.Compatible(marmotVersion) {
			continue
		}
		if _, err := semver.NewVersion(v.Version); err != nil {
			continue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return Less(out[j].Version, out[i].Version) })
	return out
}

// Latest returns the newest installable version that no advisory applies to.
func (p Plugin) Latest(marmotVersion string) (Version, bool) {
	for _, v := range p.Installable(marmotVersion) {
		if len(p.Affecting(v.Version)) == 0 {
			return v, true
		}
	}
	return Version{}, false
}

// SecurityFix returns the smallest installable step up from current that no advisory applies to, the least disruptive way off a vulnerable version.
func (p Plugin) SecurityFix(current, marmotVersion string) (Version, bool) {
	candidates := p.Installable(marmotVersion)
	for i := len(candidates) - 1; i >= 0; i-- {
		v := candidates[i]
		if Less(current, v.Version) && len(p.Affecting(v.Version)) == 0 {
			return v, true
		}
	}
	return Version{}, false
}

// LatestPatch returns the newest installable version on the same major and minor as current, newer than it and without advisories.
func (p Plugin) LatestPatch(current, marmotVersion string) (Version, bool) {
	cur, err := semver.NewVersion(current)
	if err != nil {
		return Version{}, false
	}
	for _, v := range p.Installable(marmotVersion) {
		sv, _ := semver.NewVersion(v.Version)
		if sv.Major() == cur.Major() && sv.Minor() == cur.Minor() && sv.GreaterThan(cur) && len(p.Affecting(v.Version)) == 0 {
			return v, true
		}
	}
	return Version{}, false
}

// Less reports whether version a sorts before b. Versions that are not semver sort first, by string.
func Less(a, b string) bool {
	va, errA := semver.NewVersion(strings.TrimPrefix(a, "v"))
	vb, errB := semver.NewVersion(strings.TrimPrefix(b, "v"))
	switch {
	case errA != nil && errB != nil:
		return a < b
	case errA != nil:
		return true
	case errB != nil:
		return false
	}
	return va.LessThan(vb)
}
