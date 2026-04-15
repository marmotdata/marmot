package operator

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	runsv1alpha1 "github.com/marmotdata/marmot/internal/operator/api/v1alpha1"
	"github.com/marmotdata/marmot/internal/operator/controller"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(runsv1alpha1.AddToScheme(scheme))
}

// NewCommand returns the `marmot operator` cobra command.
func NewCommand() *cobra.Command {
	var (
		metricsAddr          string
		probeAddr            string
		leaderElect          bool
		namespace            string
		marmotURL            string
		ingestServiceAccount string
		image                string
	)

	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run the Marmot Kubernetes operator",
		Long:  "Start the Marmot operator controller manager which watches Run CRDs and reconciles them into K8s Jobs/CronJobs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

			ctrl.SetLogger(crlog.New(crlog.UseDevMode(true)))

			opts := ctrl.Options{
				Scheme: scheme,
				Metrics: metricsserver.Options{
					BindAddress: metricsAddr,
				},
				HealthProbeBindAddress: probeAddr,
				LeaderElection:         leaderElect,
				LeaderElectionID:       "marmot-operator.marmotdata.io",
			}

			if namespace != "" {
				opts.Cache.DefaultNamespaces = map[string]cache.Config{namespace: {}}
			}

			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
			if err != nil {
				return fmt.Errorf("creating manager: %w", err)
			}

			cfg := controller.OperatorConfig{
				MarmotURL:            marmotURL,
				IngestServiceAccount: ingestServiceAccount,
				Image:                image,
			}

			if err := (&controller.RunReconciler{
				Client: mgr.GetClient(),
				Scheme: mgr.GetScheme(),
				Config: cfg,
			}).SetupWithManager(mgr); err != nil {
				return fmt.Errorf("setting up controller: %w", err)
			}

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				return fmt.Errorf("setting up health check: %w", err)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				return fmt.Errorf("setting up ready check: %w", err)
			}

			log.Info().Msg("Starting operator controller manager")
			return mgr.Start(ctrl.SetupSignalHandler())
		},
	}

	cmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8090", "Address the metrics endpoint binds to")
	cmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8091", "Address the health probe endpoint binds to")
	cmd.Flags().BoolVar(&leaderElect, "leader-elect", true, "Enable leader election for controller manager")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace to watch (empty = all namespaces)")
	cmd.Flags().StringVar(&marmotURL, "marmot-url", "http://marmot:8080", "Marmot API URL for Job pods")
	cmd.Flags().StringVar(&ingestServiceAccount, "ingest-service-account", "marmot-ingest", "ServiceAccount name for ingestion Job pods")
	cmd.Flags().StringVar(&image, "image", "", "Container image for Job pods (defaults to operator image)")

	return cmd
}
