package main

import (
	"flag"
	"os"

	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/controllers"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
	utilruntime.Must(templatev1.AddToScheme(scheme))
}

func main() {
	var configPath string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enablePprof bool

	flag.StringVar(&configPath, "config", "/etc/ssvirt/config.yaml", "Path to configuration file")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true, "Enable leader election for controller manager.")
	flag.BoolVar(&enablePprof, "enable-pprof", false, "Enable pprof endpoint for debugging.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Starting VM Status Controller",
		"config", configPath,
		"metrics-addr", metricsAddr,
		"probe-addr", probeAddr,
		"leader-election", enableLeaderElection,
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		setupLog.Error(err, "Unable to load configuration")
		os.Exit(1)
	}

	// Setup database connection
	db, err := database.NewConnection(cfg)
	if err != nil {
		setupLog.Error(err, "Unable to connect to database")
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			setupLog.Error(err, "Failed to close database connection")
		}
	}()

	// Create repositories
	vmRepo := repositories.NewVMRepository(db.DB)
	vappRepo := repositories.NewVAppRepository(db.DB)
	vdcRepo := repositories.NewVDCRepository(db.DB)

	// Setup controller manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ssvirt-vm-controller",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	// Setup VM Status Controller
	if err = controllers.SetupVMStatusController(mgr, vmRepo, vappRepo, vdcRepo); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "VMStatus")
		os.Exit(1)
	}

	// Add health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	// Enable pprof if requested (would need custom implementation)
	if enablePprof {
		setupLog.Info("pprof enabled - endpoints available at /debug/pprof/")
	}

	setupLog.Info("Starting manager with singleton leader election")
	ctx := ctrl.SetupSignalHandler()
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}
