package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Payload is the anonymous telemetry payload sent to the ingest endpoint.
type Payload struct {
	InstallID       string         `json:"install_id"`
	Version         string         `json:"version"`
	GoVersion       string         `json:"go_version"`
	OS              string         `json:"os"`
	Arch            string         `json:"arch"`
	DeploymentMode  string         `json:"deployment_mode"`
	UptimeSeconds   int64          `json:"uptime_seconds"`
	Timestamp       time.Time      `json:"timestamp"`
	AssetCount      int            `json:"asset_count"`
	UserCount       int            `json:"user_count"`
	LineageEdges    int            `json:"lineage_edges"`
	ConnectorCounts map[string]int `json:"connector_counts"`
}

// CollectorConfig holds configuration for the telemetry collector.
type CollectorConfig struct {
	Enabled  bool
	Endpoint string
	Interval time.Duration
	Version  string
}

// Collector gathers anonymous usage data and sends it periodically.
type Collector struct {
	db        *pgxpool.Pool
	config    CollectorConfig
	startedAt time.Time
}

// NewCollector creates a new telemetry collector.
func NewCollector(db *pgxpool.Pool, cfg CollectorConfig) *Collector {
	return &Collector{
		db:        db,
		config:    cfg,
		startedAt: time.Now(),
	}
}

// Run starts the telemetry collection loop. Returns immediately if disabled.
func (c *Collector) Run(ctx context.Context) {
	if !c.config.Enabled {
		return
	}

	installID, err := c.getOrCreateInstallID(ctx)
	if err != nil {
		log.Trace().Err(err).Msg("telemetry: failed to get install ID")
		return
	}

	payload := c.buildPayload(ctx, installID)
	if err := c.send(ctx, payload); err != nil {
		log.Trace().Err(err).Msg("telemetry: failed to send payload")
	}

	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p := c.buildPayload(ctx, installID)
			if err := c.send(ctx, p); err != nil {
				log.Trace().Err(err).Msg("telemetry: failed to send payload")
			}
		}
	}
}

func (c *Collector) getOrCreateInstallID(ctx context.Context) (string, error) {
	var id string
	err := c.db.QueryRow(ctx, "SELECT id FROM telemetry_install LIMIT 1").Scan(&id)
	if err == nil {
		return id, nil
	}

	err = c.db.QueryRow(ctx,
		"INSERT INTO telemetry_install DEFAULT VALUES RETURNING id",
	).Scan(&id)
	if err != nil {
		err2 := c.db.QueryRow(ctx, "SELECT id FROM telemetry_install LIMIT 1").Scan(&id)
		if err2 != nil {
			return "", err2
		}
	}
	return id, nil
}

func (c *Collector) buildPayload(ctx context.Context, installID string) Payload {
	p := Payload{
		InstallID:      installID,
		Version:        c.config.Version,
		GoVersion:      runtime.Version(),
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		DeploymentMode: detectDeploymentMode(),
		UptimeSeconds:  int64(time.Since(c.startedAt).Seconds()),
		Timestamp:      time.Now().UTC(),
	}

	_ = c.db.QueryRow(ctx, "SELECT COUNT(*) FROM assets").Scan(&p.AssetCount)
	_ = c.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&p.UserCount)
	_ = c.db.QueryRow(ctx, "SELECT COUNT(*) FROM lineage_edges").Scan(&p.LineageEdges)

	rows, err := c.db.Query(ctx, "SELECT source_name, COUNT(*) FROM runs GROUP BY source_name")
	if err == nil {
		defer rows.Close()
		p.ConnectorCounts = make(map[string]int)
		for rows.Next() {
			var name string
			var count int
			if rows.Scan(&name, &count) == nil {
				p.ConnectorCounts[name] = count
			}
		}
	}

	return p
}

func (c *Collector) send(ctx context.Context, payload Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func detectDeploymentMode() string {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return "kubernetes"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return "docker"
	}
	return "binary"
}
