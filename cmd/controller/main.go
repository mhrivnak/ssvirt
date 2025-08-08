package main

import (
	"fmt"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/controllers/vdc"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	var enableLeaderElection bool

	// Setup logging
	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	cfg, err := config.Load()
	if err != nil {
		setupLog.Error(err, "unable to load configuration")
		os.Exit(1)
	}

	// Initialize database connection
	db, err := database.NewConnection(cfg)
	if err != nil {
		setupLog.Error(err, "unable to connect to database")
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			setupLog.Error(err, "failed to close database connection")
		}
	}()

	// Create runtime scheme
	runtimeScheme := runtime.NewScheme()
	if err := scheme.AddToScheme(runtimeScheme); err != nil {
		setupLog.Error(err, "unable to add core resources to scheme")
		os.Exit(1)
	}

	// Setup manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 runtimeScheme,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ssvirt-controller-leader-election",
		HealthProbeBindAddress: ":8081",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup repositories
	orgRepo := repositories.NewOrganizationRepository(db.DB)
	vdcRepo := repositories.NewVDCRepository(db.DB)

	// Setup VDC controller (manages Kubernetes namespaces)
	vdcController := vdc.NewVDCReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrl.Log.WithName("controllers").WithName("VDC"),
		vdcRepo,
		orgRepo,
	)
	if err = vdcController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VDC")
		os.Exit(1)
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	
	// Custom readiness check that includes database connectivity
	readinessCheck := func(req *http.Request) error {
		// Check database connection
		sqlDB, err := db.DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get database connection: %w", err)
		}
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
		return nil
	}
	
	if err := mgr.AddReadyzCheck("readyz", readinessCheck); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
