package controllers

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRecording(t *testing.T) {
	// Create a new registry for testing to avoid conflicts
	registry := prometheus.NewRegistry()

	// Re-register metrics with test registry
	vmStatusUpdatesTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_status_updates_total",
			Help: "Total number of VM status updates processed by the controller",
		},
		[]string{"namespace", "vm_name", "old_status", "new_status", "result"},
	)

	vmStatusUpdateDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ssvirt_vm_status_update_duration_seconds",
			Help:    "Time taken to update VM status in database",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "vm_name"},
	)

	vmReconcileErrorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_reconcile_errors_total",
			Help: "Total number of VM reconciliation errors",
		},
		[]string{"namespace", "vm_name", "error_type"},
	)

	vmDeletionsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_deletions_total",
			Help: "Total number of VM deletions handled by the controller",
		},
		[]string{"namespace", "vm_name", "result"},
	)

	vmUpdatesSkippedTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_updates_skipped_total",
			Help: "Total number of VM updates skipped (not managed by SSVirt)",
		},
		[]string{"namespace", "vm_name", "reason"},
	)

	controllerHealthGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ssvirt_vm_controller_healthy",
			Help: "Whether the VM controller is healthy (1) or not (0)",
		},
	)

	registry.MustRegister(
		vmStatusUpdatesTotal,
		vmStatusUpdateDuration,
		vmReconcileErrorsTotal,
		vmDeletionsTotal,
		vmUpdatesSkippedTotal,
		controllerHealthGauge,
	)

	tests := []struct {
		name           string
		action         func()
		expectedMetric string
		expectedValue  float64
		metricType     string
	}{
		{
			name: "Record VM status update success",
			action: func() {
				vmStatusUpdatesTotal.WithLabelValues("test-ns", "test-vm", "POWERED_OFF", "POWERED_ON", "success").Inc()
				vmStatusUpdateDuration.WithLabelValues("test-ns", "test-vm").Observe(0.123)
			},
			expectedMetric: "ssvirt_vm_status_updates_total",
			expectedValue:  1,
			metricType:     "counter",
		},
		{
			name: "Record VM reconcile error",
			action: func() {
				vmReconcileErrorsTotal.WithLabelValues("test-ns", "test-vm", "database_error").Inc()
			},
			expectedMetric: "ssvirt_vm_reconcile_errors_total",
			expectedValue:  1,
			metricType:     "counter",
		},
		{
			name: "Record VM deletion",
			action: func() {
				vmDeletionsTotal.WithLabelValues("test-ns", "test-vm", "success").Inc()
			},
			expectedMetric: "ssvirt_vm_deletions_total",
			expectedValue:  1,
			metricType:     "counter",
		},
		{
			name: "Record skipped VM update",
			action: func() {
				vmUpdatesSkippedTotal.WithLabelValues("test-ns", "test-vm", "not_managed").Inc()
			},
			expectedMetric: "ssvirt_vm_updates_skipped_total",
			expectedValue:  1,
			metricType:     "counter",
		},
		{
			name: "Set controller health",
			action: func() {
				controllerHealthGauge.Set(1)
			},
			expectedMetric: "ssvirt_vm_controller_healthy",
			expectedValue:  1,
			metricType:     "gauge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metric
			tt.action()

			// Check if the metric was recorded
			metricFamilies, err := registry.Gather()
			assert.NoError(t, err)

			found := false
			for _, mf := range metricFamilies {
				if mf.GetName() == tt.expectedMetric {
					found = true

					switch tt.metricType {
					case "counter":
						assert.True(t, len(mf.GetMetric()) > 0)
						assert.Equal(t, tt.expectedValue, mf.GetMetric()[0].GetCounter().GetValue())
					case "gauge":
						assert.True(t, len(mf.GetMetric()) > 0)
						assert.Equal(t, tt.expectedValue, mf.GetMetric()[0].GetGauge().GetValue())
					case "histogram":
						assert.True(t, len(mf.GetMetric()) > 0)
						assert.True(t, mf.GetMetric()[0].GetHistogram().GetSampleCount() > 0)
					}
					break
				}
			}
			assert.True(t, found, "Metric %s not found", tt.expectedMetric)
		})
	}
}

func TestRecordVMStatusUpdateFunction(t *testing.T) {
	// Test the actual function used in the controller
	recordVMStatusUpdate("test-namespace", "test-vm", "POWERED_OFF", "POWERED_ON", "success", 0.123)

	// Verify the metric was recorded by checking if the counter increased
	// Note: This test relies on the global metrics being initialized
	value := testutil.ToFloat64(vmStatusUpdatesTotal.WithLabelValues("test-namespace", "test-vm", "POWERED_OFF", "POWERED_ON", "success"))
	assert.Greater(t, value, 0.0)
}

func TestRecordVMReconcileErrorFunction(t *testing.T) {
	recordVMReconcileError("test-namespace", "test-vm", "database_error")

	value := testutil.ToFloat64(vmReconcileErrorsTotal.WithLabelValues("test-namespace", "test-vm", "database_error"))
	assert.Greater(t, value, 0.0)
}

func TestRecordVMDeletionFunction(t *testing.T) {
	recordVMDeletion("test-namespace", "test-vm", "success")

	value := testutil.ToFloat64(vmDeletionsTotal.WithLabelValues("test-namespace", "test-vm", "success"))
	assert.Greater(t, value, 0.0)
}

func TestRecordVMSkippedFunction(t *testing.T) {
	recordVMSkipped("test-namespace", "test-vm", "not_managed")

	value := testutil.ToFloat64(vmUpdatesSkippedTotal.WithLabelValues("test-namespace", "test-vm", "not_managed"))
	assert.Greater(t, value, 0.0)
}

func TestSetControllerHealthFunction(t *testing.T) {
	// Test setting healthy
	setControllerHealth(true)
	value := testutil.ToFloat64(controllerHealthGauge)
	assert.Equal(t, 1.0, value)

	// Test setting unhealthy
	setControllerHealth(false)
	value = testutil.ToFloat64(controllerHealthGauge)
	assert.Equal(t, 0.0, value)
}
