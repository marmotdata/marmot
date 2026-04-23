package cmd

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/marmotdata/marmot/internal/api/v1"
	"github.com/marmotdata/marmot/internal/staticfiles"
	"github.com/marmotdata/marmot/internal/store/postgres"
	"github.com/marmotdata/marmot/internal/telemetry"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start an instance of the Marmot Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMarmot(cmd)
	},
}

func runMarmot(_ *cobra.Command) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	level, err := zerolog.ParseLevel(cfg.Logging.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	zerolog.SetGlobalLevel(level)

	if cfg.Logging.Format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	println("Starting Marmot...")
	ctx := context.Background()

	db, err := initializeDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Telemetry
	if cfg.Telemetry.Enabled {
		log.Info().Msg("Anonymous telemetry enabled — learn more: https://marmotdata.io/docs/configure/telemetry — disable with telemetry.enabled: false or MARMOT_TELEMETRY_ENABLED=false")
	}
	telemetryCfg := telemetry.CollectorConfig{
		Enabled:  cfg.Telemetry.Enabled,
		Endpoint: cfg.Telemetry.Endpoint,
		Interval: time.Duration(cfg.Telemetry.Interval) * time.Second,
		Version:  ServerVersion,
	}
	collector := telemetry.NewCollector(db, telemetryCfg)
	go collector.Run(ctx)

	mux := http.NewServeMux()
	server := v1.New(cfg, db)
	server.RegisterRoutes(mux)

	if cfg.Metrics.Enabled {
		go func() {
			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.Handler())

			metricsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Metrics.Port)
			log.Info().Str("address", metricsAddr).Msg("Metrics server started")

			metricsSrv := &http.Server{
				Addr:              metricsAddr,
				Handler:           metricsMux,
				ReadHeaderTimeout: 10 * time.Second,
			}
			if err := metricsSrv.ListenAndServe(); err != nil {
				log.Error().Err(err).Msg("Metrics server failed")
			}
		}()
	}

	staticHandler := staticfiles.New()
	if err := staticHandler.SetupRoutes(mux); err != nil {
		return fmt.Errorf("failed to setup static file handler: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	handler := securityHeaders(mux, cfg.Server.CustomResponseHeaders)

	if cfg.Server.TLS != nil {
		tlsCfg, err := cfg.Server.TLS.ToServerTLSConfig()
		if err != nil {
			return fmt.Errorf("configuring server TLS: %w", err)
		}

		log.Info().
			Str("address", addr).
			Str("swagger_ui", fmt.Sprintf("https://%s/swagger/index.html", addr)).
			Msg("Server started (TLS)")

		srv := &http.Server{
			Addr:              addr,
			Handler:           handler,
			TLSConfig:         tlsCfg,
			ReadHeaderTimeout: 10 * time.Second,
		}
		return srv.ListenAndServeTLS("", "")
	}

	log.Info().
		Str("address", addr).
		Str("swagger_ui", fmt.Sprintf("http://%s/swagger/index.html", addr)).
		Msg("Server started")

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// securityHeaders wraps an http.Handler to set default security headers on every response.
func securityHeaders(next http.Handler, custom map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; frame-ancestors 'self'")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		for key, value := range custom {
			w.Header().Set(key, value)
		}

		next.ServeHTTP(w, r)
	})
}

func initializeDatabase(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.BuildDSN())
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	poolConfig.MaxConns = safeInt32(cfg.Database.MaxConns)
	poolConfig.MinConns = safeInt32(cfg.Database.IdleConns)
	poolConfig.MaxConnLifetime = time.Duration(cfg.Database.ConnLifetime) * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	setup := postgres.NewSetup(pool)
	if err := setup.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initializing database: %w", err)
	}

	return pool, nil
}

func safeInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < 0 {
		return 0
	}
	return int32(v) //nolint:gosec // G115: bounds checked above
}
