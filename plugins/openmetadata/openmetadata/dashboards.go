package openmetadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	dashboardFields = "owners,tags,domains,dataProducts,charts,dataModels"
	chartFields     = "owners,tags,domains,dataProducts"
	dataModelFields = "owners,tags,domains,dataProducts,columns"
)

// discoverDashboards catalogues BI dashboards, the charts on them and
// the data models behind them, and links each dashboard to its charts.
func (c *collector) discoverDashboards(ctx context.Context, client *client) error {
	dashboards, err := listAll[dashboard](ctx, client, "/v1/dashboards", dashboardFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing dashboards: %w", err)
	}

	charts, err := listAll[chart](ctx, client, "/v1/charts", chartFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing charts: %w", err)
	}

	models, _, err := listOptional[dashboardDataModel](ctx, client, "/v1/dashboard/datamodels", dataModelFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing dashboard data models: %w", err)
	}

	chartMRNs := make(map[string]string, len(charts))
	for _, ch := range charts {
		if !c.wanted(ch.entityBase) {
			continue
		}
		p := projectionFor(ch.ServiceType)
		name := strings.Join(fqnBelowService(ch.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "chart_type", ch.ChartType)

		asset := c.newAsset(ch.entityBase, "chart", "Chart", p, c.mrnName(name, ch.FullyQualifiedName), metadata)
		c.add(ch.ID, asset)
		chartMRNs[ch.ID] = *asset.MRN
	}

	modelMRNs := make(map[string]string, len(models))
	for _, dm := range models {
		if !c.wanted(dm.entityBase) {
			continue
		}
		p := projectionFor(dm.ServiceType)
		name := strings.Join(fqnBelowService(dm.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "data_model_type", dm.DataModelType)
		putIf(metadata, "project", dm.Project)

		asset := c.newAsset(dm.entityBase, "dashboardDataModel", "Data Model Object", p, c.mrnName(name, dm.FullyQualifiedName), metadata)
		if c.config.IncludeColumns {
			setColumns(&asset, dm.Columns)
		}
		if sql := strings.TrimSpace(dm.SQL); sql != "" {
			language := "SQL"
			asset.Query = &sql
			asset.QueryLanguage = &language
		}

		c.add(dm.ID, asset)
		modelMRNs[dm.ID] = *asset.MRN
	}

	discovered := 0
	for _, d := range dashboards {
		if !c.wanted(d.entityBase) {
			continue
		}
		p := projectionFor(d.ServiceType)
		name := strings.Join(fqnBelowService(d.FullyQualifiedName), ".")
		if name == "" {
			continue
		}

		metadata := map[string]interface{}{}
		putIf(metadata, "dashboard_type", d.DashboardType)
		putIf(metadata, "project", d.Project)
		putIf(metadata, "chart_count", len(d.Charts))

		asset := c.newAsset(d.entityBase, "dashboard", "Dashboard", p, c.mrnName(name, d.FullyQualifiedName), metadata)
		c.add(d.ID, asset)
		discovered++

		for _, ref := range d.Charts {
			if chartMRN, ok := chartMRNs[ref.ID]; ok {
				c.link(*asset.MRN, chartMRN, "CONTAINS")
			}
		}
		for _, ref := range d.DataModels {
			if modelMRN, ok := modelMRNs[ref.ID]; ok {
				c.link(modelMRN, *asset.MRN, "FEEDS")
			}
		}
	}

	log.Debug().
		Int("dashboards", discovered).
		Int("charts", len(chartMRNs)).
		Int("data_models", len(modelMRNs)).
		Msg("Discovered dashboards")
	return nil
}
