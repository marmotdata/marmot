package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	v1 "github.com/marmotdata/marmot/internal/api/v1"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/staticfiles"
	"github.com/marmotdata/marmot/internal/store/postgres"
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

	mux := http.NewServeMux()
	server := v1.New(cfg, db)
	server.RegisterRoutes(mux)

	if cfg.Metrics.Enabled {
		go func() {
			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.Handler())

			metricsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Metrics.Port)
			log.Info().Str("address", metricsAddr).Msg("Metrics server started")

			if err := http.ListenAndServe(metricsAddr, metricsMux); err != nil {
				log.Error().Err(err).Msg("Metrics server failed")
			}
		}()
	}

	staticHandler := staticfiles.New()
	if err := staticHandler.SetupRoutes(mux); err != nil {
		return fmt.Errorf("failed to setup static file handler: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info().
		Str("address", addr).
		Str("swagger_ui", fmt.Sprintf("http://%s/swagger/index.html", addr)).
		Msg("Server started")

	return http.ListenAndServe(addr, mux)
}

func initializeDatabase(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.BuildDSN())
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.Database.MaxConns)
	poolConfig.MinConns = int32(cfg.Database.IdleConns)
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
