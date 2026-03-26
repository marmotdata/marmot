package cmd

import (
	"fmt"
	"time"

	"github.com/marmotdata/marmot/client/client/metrics"
	"github.com/marmotdata/marmot/internal/cmd/output"
	"github.com/spf13/cobra"
)

const defaultMetricsRange = 30 * 24 * time.Hour // 30 days

func parseTimeRange(cmd *cobra.Command) (time.Time, time.Time, error) {
	startStr, _ := cmd.Flags().GetString("start")
	endStr, _ := cmd.Flags().GetString("end")

	var end time.Time
	if endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --end format, use RFC3339 (e.g. 2026-01-01T00:00:00Z): %w", err)
		}
	} else {
		end = time.Now().UTC()
	}

	var start time.Time
	if startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --start format, use RFC3339 (e.g. 2026-01-01T00:00:00Z): %w", err)
		}
	} else {
		start = end.Add(-defaultMetricsRange)
	}

	return start, end, nil
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "View catalog metrics",
}

var metricsSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show combined metrics summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c := newSwaggerClient()

		totalResp, err := c.Metrics.GetAPIV1MetricsAssetsTotal(metrics.NewGetAPIV1MetricsAssetsTotalParams())
		if err != nil {
			return err
		}
		byTypeResp, err := c.Metrics.GetAPIV1MetricsAssetsByType(metrics.NewGetAPIV1MetricsAssetsByTypeParams())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			combined := map[string]interface{}{
				"total_assets":   totalResp.Payload,
				"assets_by_type": byTypeResp.Payload,
			}
			return p.PrintJSON(combined)
		}

		fmt.Printf("Total Assets: %d\n\n", totalResp.Payload.Count)

		if len(byTypeResp.Payload.Assets) > 0 {
			t := output.NewTable("TYPE", "COUNT")
			for k, v := range byTypeResp.Payload.Assets {
				t.AddRow(k, fmt.Sprintf("%d", v))
			}
			p.PrintTable(t)
		}
		return nil
	},
}

var metricsByTypeCmd = &cobra.Command{
	Use:   "by-type",
	Short: "Show assets grouped by type",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c := newSwaggerClient()

		resp, err := c.Metrics.GetAPIV1MetricsAssetsByType(metrics.NewGetAPIV1MetricsAssetsByTypeParams())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("TYPE", "COUNT")
		for k, v := range resp.Payload.Assets {
			t.AddRow(k, fmt.Sprintf("%d", v))
		}
		p.PrintTable(t)
		return nil
	},
}

var metricsByProviderCmd = &cobra.Command{
	Use:   "by-provider",
	Short: "Show assets grouped by provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getPrinter()
		c := newSwaggerClient()

		resp, err := c.Metrics.GetAPIV1MetricsAssetsByProvider(metrics.NewGetAPIV1MetricsAssetsByProviderParams())
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("PROVIDER", "COUNT")
		for k, v := range resp.Payload.Assets {
			t.AddRow(k, fmt.Sprintf("%d", v))
		}
		p.PrintTable(t)
		return nil
	},
}

var metricsTopAssetsCmd = &cobra.Command{
	Use:   "top-assets",
	Short: "Show most viewed assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		start, end, err := parseTimeRange(cmd)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		p := getPrinter()
		c := newSwaggerClient()

		params := metrics.NewGetAPIV1MetricsTopAssetsParams()
		params.SetStart(start.Format(time.RFC3339))
		params.SetEnd(end.Format(time.RFC3339))
		params.SetLimit(int64Ptr(limit))

		resp, err := c.Metrics.GetAPIV1MetricsTopAssets(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("NAME", "TYPE", "VIEWS")
		for _, a := range resp.Payload {
			t.AddRow(a.AssetName, a.AssetType, fmt.Sprintf("%d", a.Count))
		}
		p.PrintTable(t)
		return nil
	},
}

var metricsTopQueriesCmd = &cobra.Command{
	Use:   "top-queries",
	Short: "Show most popular search queries",
	RunE: func(cmd *cobra.Command, args []string) error {
		start, end, err := parseTimeRange(cmd)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		p := getPrinter()
		c := newSwaggerClient()

		params := metrics.NewGetAPIV1MetricsTopQueriesParams()
		params.SetStart(start.Format(time.RFC3339))
		params.SetEnd(end.Format(time.RFC3339))
		params.SetLimit(int64Ptr(limit))

		resp, err := c.Metrics.GetAPIV1MetricsTopQueries(params)
		if err != nil {
			return err
		}

		if p.IsRaw() {
			data, err := marshalPayload(resp.Payload)
			if err != nil {
				return err
			}
			return p.PrintRaw(data)
		}

		t := output.NewTable("QUERY", "COUNT")
		for _, q := range resp.Payload {
			t.AddRow(q.Query, fmt.Sprintf("%d", q.Count))
		}
		p.PrintTable(t)
		return nil
	},
}

func init() {
	metricsTopAssetsCmd.Flags().Int("limit", 10, "Maximum number of results")
	metricsTopAssetsCmd.Flags().String("start", "", "Start time in RFC3339 format (default: 30 days ago)")
	metricsTopAssetsCmd.Flags().String("end", "", "End time in RFC3339 format (default: now)")

	metricsTopQueriesCmd.Flags().Int("limit", 10, "Maximum number of results")
	metricsTopQueriesCmd.Flags().String("start", "", "Start time in RFC3339 format (default: 30 days ago)")
	metricsTopQueriesCmd.Flags().String("end", "", "End time in RFC3339 format (default: now)")

	metricsCmd.AddCommand(metricsSummaryCmd)
	metricsCmd.AddCommand(metricsByTypeCmd)
	metricsCmd.AddCommand(metricsByProviderCmd)
	metricsCmd.AddCommand(metricsTopAssetsCmd)
	metricsCmd.AddCommand(metricsTopQueriesCmd)
	rootCmd.AddCommand(metricsCmd)
}
