package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Counter for VM status updates processed by the controller
	vmStatusUpdatesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_status_updates_total",
			Help: "Total number of VM status updates processed by the controller",
		},
		[]string{"namespace", "vm_name", "old_status", "new_status", "result"},
	)

	// Histogram for time taken to update VM status in database
	vmStatusUpdateDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ssvirt_vm_status_update_duration_seconds",
			Help:    "Time taken to update VM status in database",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "vm_name"},
	)

	// Counter for VM reconciliation errors
	vmReconcileErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_reconcile_errors_total",
			Help: "Total number of VM reconciliation errors",
		},
		[]string{"namespace", "vm_name", "error_type"},
	)

	// Gauge for currently tracked VMs
	vmTrackedGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ssvirt_vm_tracked_total",
			Help: "Current number of VMs being tracked by the controller",
		},
		[]string{"namespace"},
	)

	// Counter for VM deletions handled
	vmDeletionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_deletions_total",
			Help: "Total number of VM deletions handled by the controller",
		},
		[]string{"namespace", "vm_name", "result"},
	)

	// Gauge for controller health status
	controllerHealthGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ssvirt_vm_controller_healthy",
			Help: "Whether the VM controller is healthy (1) or not (0)",
		},
	)

	// Counter for skipped VM updates (not managed by SSVirt)
	vmUpdatesSkippedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_updates_skipped_total",
			Help: "Total number of VM updates skipped (not managed by SSVirt)",
		},
		[]string{"namespace", "vm_name", "reason"},
	)

	// Counter for vapp.ssvirt label operations
	vmLabelOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_label_operations_total",
			Help: "Total number of vapp.ssvirt label operations processed",
		},
		[]string{"namespace", "vm_name", "operation", "result"},
	)

	// Counter for VM record creation operations
	vmCreationOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vm_creation_operations_total",
			Help: "Total number of VM record creation operations processed",
		},
		[]string{"namespace", "vm_name", "vapp_name", "result"},
	)

	// Counter for VApp creation operations
	vappCreationOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ssvirt_vapp_creation_operations_total",
			Help: "Total number of VApp record creation operations processed",
		},
		[]string{"namespace", "vdc_id", "vapp_name", "result"},
	)
)

func init() {
	// Register metrics with controller-runtime
	metrics.Registry.MustRegister(
		vmStatusUpdatesTotal,
		vmStatusUpdateDuration,
		vmReconcileErrorsTotal,
		vmTrackedGauge,
		vmDeletionsTotal,
		controllerHealthGauge,
		vmUpdatesSkippedTotal,
		vmLabelOperationsTotal,
		vmCreationOperationsTotal,
		vappCreationOperationsTotal,
	)

	// Initialize controller as healthy
	controllerHealthGauge.Set(1)
}

// recordVMStatusUpdate records metrics for a VM status update
func recordVMStatusUpdate(namespace, vmName, oldStatus, newStatus, result string, duration float64) {
	vmStatusUpdatesTotal.WithLabelValues(namespace, vmName, oldStatus, newStatus, result).Inc()
	vmStatusUpdateDuration.WithLabelValues(namespace, vmName).Observe(duration)
}

// recordVMReconcileError records metrics for a reconciliation error
func recordVMReconcileError(namespace, vmName, errorType string) {
	vmReconcileErrorsTotal.WithLabelValues(namespace, vmName, errorType).Inc()
}

// recordVMDeletion records metrics for a VM deletion
func recordVMDeletion(namespace, vmName, result string) {
	vmDeletionsTotal.WithLabelValues(namespace, vmName, result).Inc()
}

// recordVMSkipped records metrics for a skipped VM update
func recordVMSkipped(namespace, vmName, reason string) {
	vmUpdatesSkippedTotal.WithLabelValues(namespace, vmName, reason).Inc()
}

// recordVMLabelOperation records metrics for a vapp.ssvirt label operation
func recordVMLabelOperation(namespace, vmName, operation, result string) {
	vmLabelOperationsTotal.WithLabelValues(namespace, vmName, operation, result).Inc()
}

// recordVMCreationOperation records metrics for a VM record creation operation
func recordVMCreationOperation(namespace, vmName, vappName, result string) {
	vmCreationOperationsTotal.WithLabelValues(namespace, vmName, vappName, result).Inc()
}

// recordVAppCreationOperation records metrics for a VApp record creation operation
func recordVAppCreationOperation(namespace, vdcID, vappName, result string) {
	vappCreationOperationsTotal.WithLabelValues(namespace, vdcID, vappName, result).Inc()
}

// setControllerHealth sets the controller health metric
func setControllerHealth(healthy bool) {
	if healthy {
		controllerHealthGauge.Set(1)
	} else {
		controllerHealthGauge.Set(0)
	}
}
