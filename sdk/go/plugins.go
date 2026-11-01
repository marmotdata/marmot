package marmot

import (
	"context"

	apiclient "github.com/marmotdata/marmot/sdk/go/internal/gen/client"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/client/plugins"
	"github.com/marmotdata/marmot/sdk/go/internal/gen/models"
)

// PluginUpdatesStatus reports every core plugin's version, available updates and the plugin index state.
type PluginUpdatesStatus = models.PluginUpdatesStatus

// PluginIndexStatus describes the stored plugin index and when it was last checked.
type PluginIndexStatus = models.PluginIndexStatus

// PluginVersionStatus is one core plugin's version and the versions it could move to.
type PluginVersionStatus = models.PluginVersionStatus

// PluginPublishedVersion is a published version of a plugin as it applies to this Marmot.
type PluginPublishedVersion = models.PluginPublishedVersion

// PluginAdvisory is a security advisory against a range of plugin versions.
type PluginAdvisory = models.PluginAdvisory

// PluginsService manages which version each core plugin runs at.
type PluginsService struct {
	gen *apiclient.Marmot
}

// Versions returns every core plugin's version, available updates and advisories.
func (s *PluginsService) Versions(ctx context.Context) (*PluginUpdatesStatus, error) {
	p := plugins.NewGetPluginVersionsParams().WithContext(ctx)
	resp, err := s.gen.Plugins.GetPluginVersions(p, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	return resp.Payload, nil
}

// Check checks the plugin index for new versions and advisories now, and reports whether a newer index was found.
func (s *PluginsService) Check(ctx context.Context) (bool, error) {
	p := plugins.NewCheckPluginVersionsParams().WithContext(ctx)
	resp, err := s.gen.Plugins.CheckPluginVersions(p, nil)
	if err != nil {
		return false, mapErr(err)
	}
	return resp.Payload.Updated, nil
}

// SetVersion moves a core plugin to a version from the plugin index.
func (s *PluginsService) SetVersion(ctx context.Context, id, version string) error {
	p := plugins.NewSetPluginVersionParams().WithContext(ctx).WithID(id).WithRequest(&models.SetPluginVersionRequest{Version: version})
	_, err := s.gen.Plugins.SetPluginVersion(p, nil)
	return mapErr(err)
}

// Reset returns a core plugin to the version this Marmot release pins.
func (s *PluginsService) Reset(ctx context.Context, id string) error {
	p := plugins.NewResetPluginVersionParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Plugins.ResetPluginVersion(p, nil)
	return mapErr(err)
}

// Pin pins or unpins a core plugin at the version it runs.
func (s *PluginsService) Pin(ctx context.Context, id string, pinned bool) error {
	p := plugins.NewPinPluginVersionParams().WithContext(ctx).WithID(id).WithRequest(&models.PinPluginVersionRequest{Pinned: pinned})
	_, err := s.gen.Plugins.PinPluginVersion(p, nil)
	return mapErr(err)
}

// Rollback rolls a core plugin back to the version it ran before its last change, and pins it.
func (s *PluginsService) Rollback(ctx context.Context, id string) error {
	p := plugins.NewRollbackPluginVersionParams().WithContext(ctx).WithID(id)
	_, err := s.gen.Plugins.RollbackPluginVersion(p, nil)
	return mapErr(err)
}
