package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	templatev1 "github.com/openshift/api/template/v1"
	"gorm.io/gorm"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// VAppStatusRepositoryInterface defines the interface for VApp repository operations
type VAppStatusRepositoryInterface interface {
	GetByNameInVDC(ctx context.Context, vdcID, name string) (*models.VApp, error)
	UpdateStatus(ctx context.Context, vappID string, status string) error
}

// VMStatusRepositoryInterface defines the interface for VM repository operations
type VMStatusRepositoryInterface interface {
	GetByVAppID(vappID string) ([]models.VM, error)
}

// VDCStatusRepositoryInterface defines the interface for VDC repository operations
type VDCStatusRepositoryInterface interface {
	GetByNamespace(ctx context.Context, namespaceName string) (*models.VDC, error)
}

// VAppStatusController reconciles vApp status based on TemplateInstance and VM states
type VAppStatusController struct {
	client.Client
	Scheme   *runtime.Scheme
	VAppRepo VAppStatusRepositoryInterface
	VMRepo   VMStatusRepositoryInterface
	VDCRepo  VDCStatusRepositoryInterface
}

// VAppStatusEvaluator evaluates vApp status based on multiple inputs
type VAppStatusEvaluator struct {
	templateInstanceReady  bool
	templateInstanceFailed bool
	vmStatuses             []string
	hasVMs                 bool
}

// EvaluateStatus determines the appropriate vApp status
func (e *VAppStatusEvaluator) EvaluateStatus() string {
	// Check for deletion state - handled elsewhere

	// Check TemplateInstance status for instantiation/failure
	if e.templateInstanceFailed {
		return models.VAppStatusFailed
	}

	// If TemplateInstance is not ready, still instantiating
	if !e.templateInstanceReady {
		return models.VAppStatusInstantiating
	}

	// If no VMs yet, still instantiating
	if !e.hasVMs {
		return models.VAppStatusInstantiating
	}

	// Check VM statuses - any VM still provisioning means vApp is instantiating
	for _, vmStatus := range e.vmStatuses {
		if vmStatus == "UNRESOLVED" || vmStatus == "" {
			return models.VAppStatusInstantiating
		}
		if vmStatus == "DELETING" || vmStatus == "DELETED" {
			return models.VAppStatusDeleting
		}
	}

	// All VMs are in stable states (POWERED_ON, POWERED_OFF, SUSPENDED), vApp is deployed
	return models.VAppStatusDeployed
}

// +kubebuilder:rbac:groups=template.openshift.io,resources=templateinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=template.openshift.io,resources=templateinstances/status,verbs=get
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines/status,verbs=get
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances/status,verbs=get

// Reconcile handles vApp status updates based on TemplateInstance changes
func (r *VAppStatusController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the TemplateInstance
	var templateInstance templatev1.TemplateInstance
	if err := r.Get(ctx, req.NamespacedName, &templateInstance); err != nil {
		if k8serrors.IsNotFound(err) {
			// TemplateInstance was deleted, ignore
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get TemplateInstance")
		return ctrl.Result{}, err
	}

	// Find corresponding vApp by name and namespace
	vdc, err := r.VDCRepo.GetByNamespace(ctx, templateInstance.Namespace)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No VDC found for this namespace, ignore
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get VDC for namespace", "namespace", templateInstance.Namespace)
		return ctrl.Result{}, err
	}

	vapp, err := r.VAppRepo.GetByNameInVDC(ctx, vdc.ID, templateInstance.Name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No vApp found for this TemplateInstance, ignore
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get vApp", "name", templateInstance.Name, "vdc", vdc.ID)
		return ctrl.Result{}, err
	}

	// Evaluate new status
	newStatus := r.evaluateVAppStatus(ctx, &templateInstance, vapp, logger)

	// Update status if changed
	if vapp.Status != newStatus {
		oldStatus := vapp.Status
		err := r.VAppRepo.UpdateStatus(ctx, vapp.ID, newStatus)
		if err != nil {
			logger.Error(err, "Failed to update vApp status", "vapp", vapp.ID, "oldStatus", oldStatus, "newStatus", newStatus)
			return ctrl.Result{}, err
		}

		logger.Info("Updated vApp status", "vapp", vapp.ID, "oldStatus", oldStatus, "newStatus", newStatus)
	}

	return ctrl.Result{}, nil
}

// evaluateVAppStatus evaluates the appropriate vApp status
func (r *VAppStatusController) evaluateVAppStatus(ctx context.Context, templateInstance *templatev1.TemplateInstance, vapp *models.VApp, logger logr.Logger) string {
	evaluator := &VAppStatusEvaluator{}

	// Evaluate TemplateInstance status
	evaluator.templateInstanceReady, evaluator.templateInstanceFailed = r.evaluateTemplateInstanceStatus(templateInstance)

	// Get VM statuses within the vApp
	vms, err := r.VMRepo.GetByVAppID(vapp.ID)
	if err != nil {
		logger.Error(err, "Failed to get VMs for vApp", "vapp", vapp.ID)
		// If we can't get VMs, keep current status
		return vapp.Status
	}

	evaluator.hasVMs = len(vms) > 0
	evaluator.vmStatuses = make([]string, len(vms))
	for i, vm := range vms {
		evaluator.vmStatuses[i] = vm.Status
	}

	return evaluator.EvaluateStatus()
}

// evaluateTemplateInstanceStatus checks TemplateInstance conditions
func (r *VAppStatusController) evaluateTemplateInstanceStatus(templateInstance *templatev1.TemplateInstance) (ready bool, failed bool) {
	for _, condition := range templateInstance.Status.Conditions {
		switch condition.Type {
		case templatev1.TemplateInstanceReady:
			if condition.Status == "True" {
				ready = true
			}
		case templatev1.TemplateInstanceInstantiateFailure:
			if condition.Status == "True" {
				failed = true
			}
		}
	}
	return ready, failed
}

// SetupWithManager sets up the controller with the Manager
func (r *VAppStatusController) SetupWithManager(mgr ctrl.Manager) error {
	// Watch TemplateInstance resources
	err := ctrl.NewControllerManagedBy(mgr).
		For(&templatev1.TemplateInstance{}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to setup TemplateInstance controller: %w", err)
	}

	return nil
}

// SetupVAppStatusController sets up the VApp status controller with the manager
func SetupVAppStatusController(mgr ctrl.Manager, vappRepo VAppStatusRepositoryInterface, vmRepo VMStatusRepositoryInterface, vdcRepo VDCStatusRepositoryInterface) error {
	return (&VAppStatusController{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		VAppRepo: vappRepo,
		VMRepo:   vmRepo,
		VDCRepo:  vdcRepo,
	}).SetupWithManager(mgr)
}
