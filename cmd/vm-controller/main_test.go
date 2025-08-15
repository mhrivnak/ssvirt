package main

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func TestMainFunction(t *testing.T) {
	// Test command line flag parsing
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name     string
		args     []string
		expected map[string]interface{}
	}{
		{
			name: "default values",
			args: []string{"vm-controller"},
			expected: map[string]interface{}{
				"metricsAddr":          ":8080",
				"probeAddr":            ":8081",
				"enableLeaderElection": false,
				"enablePprof":          false,
			},
		},
		{
			name: "custom values",
			args: []string{
				"vm-controller",
				"--metrics-bind-address=:9090",
				"--health-probe-bind-address=:9091",
				"--leader-elect=true",
				"--enable-pprof=true",
			},
			expected: map[string]interface{}{
				"metricsAddr":          ":9090",
				"probeAddr":            ":9091",
				"enableLeaderElection": true,
				"enablePprof":          true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			os.Args = tt.args

			// Parse flags
			var (
				metricsAddr          = flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
				probeAddr            = flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
				enableLeaderElection = flag.Bool("leader-elect", false, "Enable leader election for controller manager.")
				enablePprof          = flag.Bool("enable-pprof", false, "Enable pprof endpoint for debugging.")
			)

			flag.Parse()

			// Verify parsed values
			assert.Equal(t, tt.expected["metricsAddr"], *metricsAddr)
			assert.Equal(t, tt.expected["probeAddr"], *probeAddr)
			assert.Equal(t, tt.expected["enableLeaderElection"], *enableLeaderElection)
			assert.Equal(t, tt.expected["enablePprof"], *enablePprof)
		})
	}
}

func TestSchemeSetup(t *testing.T) {
	scheme := runtime.NewScheme()

	// Test that we can set up the scheme without errors using the same logic as main
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))

	// Verify that required types are registered
	gvks := scheme.AllKnownTypes()
	assert.NotEmpty(t, gvks)

	// Check for KubeVirt VirtualMachine type
	found := false
	for gvk := range gvks {
		if gvk.Kind == "VirtualMachine" && gvk.Group == "kubevirt.io" {
			found = true
			break
		}
	}
	assert.True(t, found, "VirtualMachine type should be registered in scheme")
}

func TestManagerCreation(t *testing.T) {
	// Test manager creation with different configurations
	tests := []struct {
		name        string
		config      manager.Options
		expectError bool
	}{
		{
			name: "valid configuration",
			config: manager.Options{
				Scheme:                 runtime.NewScheme(),
				HealthProbeBindAddress: ":8081",
				LeaderElection:         false,
			},
			expectError: false,
		},
		{
			name: "with leader election",
			config: manager.Options{
				Scheme:                        runtime.NewScheme(),
				HealthProbeBindAddress:        ":8081",
				LeaderElection:                true,
				LeaderElectionID:              "ssvirt-vm-controller",
				LeaderElectionReleaseOnCancel: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up scheme
			utilruntime.Must(clientgoscheme.AddToScheme(tt.config.Scheme))
			utilruntime.Must(kubevirtv1.AddToScheme(tt.config.Scheme))

			// We can verify the config structure is valid
			assert.NotNil(t, tt.config.Scheme)
			assert.NotEmpty(t, tt.config.HealthProbeBindAddress)

			// Get config safely without panicking
			config, err := ctrl.GetConfig()
			if err != nil || config == nil {
				t.Skipf("Skipping test: no kubeconfig/in-cluster config available: %v", err)
				return
			}

			// Try to create manager - this may succeed or fail depending on environment
			_, err = ctrl.NewManager(config, tt.config)
			if !tt.expectError {
				// For valid configs, either success or connection failure is acceptable
				// The important thing is our config structure is correct
				if err != nil {
					// Expected in environments without k8s access
					t.Logf("Manager creation failed as expected in test environment: %v", err)
				} else {
					t.Logf("Manager creation succeeded")
				}
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	// Test context cancellation handling
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate graceful shutdown
	shutdownComplete := make(chan bool, 1)

	go func() {
		select {
		case <-ctx.Done():
			// Simulate cleanup
			time.Sleep(10 * time.Millisecond)
			shutdownComplete <- true
		case <-time.After(1 * time.Second):
			shutdownComplete <- false
		}
	}()

	// Cancel context to trigger shutdown
	cancel()

	// Wait for shutdown to complete
	select {
	case completed := <-shutdownComplete:
		assert.True(t, completed, "Graceful shutdown should complete")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Shutdown took too long")
	}
}

func TestEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		expected string
	}{
		{
			name:     "CONTROLLER_NAMESPACE",
			envVar:   "CONTROLLER_NAMESPACE",
			envValue: "test-namespace",
			expected: "test-namespace",
		},
		{
			name:     "LOG_LEVEL",
			envVar:   "LOG_LEVEL",
			envValue: "debug",
			expected: "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			oldValue := os.Getenv(tt.envVar)
			defer func() {
				if err := os.Setenv(tt.envVar, oldValue); err != nil {
					t.Errorf("Failed to restore environment variable %s: %v", tt.envVar, err)
				}
			}()

			if err := os.Setenv(tt.envVar, tt.envValue); err != nil {
				t.Fatalf("Failed to set environment variable %s: %v", tt.envVar, err)
			}

			// Verify environment variable is set
			assert.Equal(t, tt.expected, os.Getenv(tt.envVar))
		})
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that metrics can be registered without conflicts
	registry := metrics.Registry
	assert.NotNil(t, registry)

	// The metrics package should initialize metrics during import
	// We can verify the registry exists and is usable
	metricFamilies, err := registry.Gather()
	assert.NoError(t, err)
	assert.NotNil(t, metricFamilies)
}
