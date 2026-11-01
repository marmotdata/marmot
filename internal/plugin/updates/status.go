package updates

import (
	"context"
	"sort"
	"time"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/plugin/index"
	"github.com/marmotdata/marmot/internal/plugin/install"
)

// Status is what the plugin settings page and CLI show.
type Status struct {
	// Enabled reports whether Marmot checks the plugin index for updates.
	Enabled       bool           `json:"enabled"`
	AutoUpdate    string         `json:"auto_update"`
	MarmotVersion string         `json:"marmot_version"`
	Index         IndexStatus    `json:"index"`
	Plugins       []PluginStatus `json:"plugins"`
} // @name PluginUpdatesStatus

// IndexStatus describes the stored plugin index and the last check.
type IndexStatus struct {
	Ref       string     `json:"ref"`
	Version   int64      `json:"version,omitempty"`
	Generated *time.Time `json:"generated,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Expired   bool       `json:"expired"`
	FetchedAt *time.Time `json:"fetched_at,omitempty"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	Error     string     `json:"error,omitempty"`
} // @name PluginIndexStatus

// PluginStatus is one core plugin's version and the versions it could move to.
type PluginStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
	// Version is the version the plugin should run at; empty for a locally installed override.
	Version string `json:"version"`
	// Loaded reports whether this replica has the plugin registered.
	Loaded         bool   `json:"loaded"`
	ReleaseVersion string `json:"release_version"`
	// Origin is release, manual, auto or local.
	Origin          string     `json:"origin"`
	Pinned          bool       `json:"pinned"`
	PreviousVersion string     `json:"previous_version,omitempty"`
	UpdatedBy       string     `json:"updated_by,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	// Latest is the newest version that supports this Marmot and has no advisory against it.
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	// Advisories are those against the version the plugin runs.
	Advisories []index.Advisory `json:"advisories"`
	Versions   []VersionStatus  `json:"versions"`
	// Error is this replica's last failure switching the plugin's version.
	Error string `json:"error,omitempty"`
} // @name PluginVersionStatus

// VersionStatus is a published version of a plugin as it applies to this Marmot.
type VersionStatus struct {
	index.Version
	Compatible bool     `json:"compatible"`
	Advisories []string `json:"advisories"`
} // @name PluginPublishedVersion

// Status reports every core plugin's version, newest versions first.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	if err := m.Load(ctx); err != nil {
		return Status{}, err
	}

	host := plugin.HostVersion()
	m.mu.RLock()
	defer m.mu.RUnlock()

	st := Status{
		Enabled:       m.cfg.IndexEnabled,
		AutoUpdate:    string(m.cfg.AutoUpdate),
		MarmotVersion: host,
		Index: IndexStatus{
			Ref:       m.cfg.IndexRef,
			Version:   m.idxRow.Version,
			ExpiresAt: m.idxRow.ExpiresAt,
			FetchedAt: m.idxRow.FetchedAt,
			CheckedAt: m.idxRow.CheckedAt,
			Error:     m.idxRow.LastError,
		},
		Plugins: make([]PluginStatus, 0, len(m.manifest.Plugins)),
	}
	if m.idx != nil {
		generated := m.idx.Generated
		st.Index.Generated = &generated
		st.Index.Expired = m.idx.Expired(time.Now())
	}

	for name, base := range m.manifest.Plugins {
		ps := PluginStatus{
			ID:             name,
			Name:           name,
			ReleaseVersion: base.Version,
			Advisories:     []index.Advisory{},
			Versions:       []VersionStatus{},
			Error:          m.errs[name],
		}
		if entry, err := plugin.GetRegistry().Get(name); err == nil {
			ps.Loaded = true
			if entry.Meta.Name != "" {
				ps.Name = entry.Meta.Name
			}
			ps.Icon = entry.Meta.Icon
		}

		if install.HasLocalOverride(name) {
			ps.Origin = OriginLocal
			st.Plugins = append(st.Plugins, ps)
			continue
		}

		pin, origin := m.effectiveLocked(name)
		ps.Version = pin.Version
		ps.Origin = origin
		if r, ok := m.desired[name]; ok && !m.trustedLocked(name, r) && ps.Error == "" {
			ps.Error = "the stored version " + r.Version + " is not in the signed plugin index, so the release version runs instead"
		}
		if r, ok := m.desired[name]; ok {
			ps.Pinned = r.Pinned
			ps.PreviousVersion = r.PreviousVersion
			ps.UpdatedBy = r.UpdatedBy
			updated := r.UpdatedAt
			ps.UpdatedAt = &updated
		}

		if m.idx != nil {
			p := m.idx.Plugins[name]
			ps.Advisories = append(ps.Advisories, p.Affecting(ps.Version)...)
			if latest, ok := p.Latest(host); ok {
				ps.Latest = latest.Version
				ps.UpdateAvailable = index.Less(ps.Version, latest.Version)
			}
			for _, v := range p.Versions {
				vs := VersionStatus{Version: v, Compatible: v.Compatible(host), Advisories: []string{}}
				for _, a := range p.Affecting(v.Version) {
					vs.Advisories = append(vs.Advisories, a.ID)
				}
				ps.Versions = append(ps.Versions, vs)
			}
			sort.Slice(ps.Versions, func(i, j int) bool { return index.Less(ps.Versions[j].Version.Version, ps.Versions[i].Version.Version) })
		}
		st.Plugins = append(st.Plugins, ps)
	}
	sort.Slice(st.Plugins, func(i, j int) bool { return st.Plugins[i].ID < st.Plugins[j].ID })
	return st, nil
}
