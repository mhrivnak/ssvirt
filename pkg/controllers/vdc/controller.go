package vdc

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// VDCReconciler reconciles VDC database records with Kubernetes namespaces
type VDCReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	VDCRepo  *repositories.VDCRepository
	OrgRepo  *repositories.OrganizationRepository
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewVDCReconciler creates a new VDCReconciler
func NewVDCReconciler(client client.Client, scheme *runtime.Scheme, log logr.Logger, vdcRepo *repositories.VDCRepository, orgRepo *repositories.OrganizationRepository) *VDCReconciler {
	ctx, cancel := context.WithCancel(context.Background())
	return &VDCReconciler{
		Client:   client,
		Scheme:   scheme,
		Log:      log,
		VDCRepo:  vdcRepo,
		OrgRepo:  orgRepo,
		interval: 30 * time.Second, // Poll database every 30 seconds
		ctx:      ctx,
		cancel:   cancel,
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *VDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a controller builder and watch namespace changes
	err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Named("vdc").
		Complete(r)

	if err != nil {
		return err
	}

	// Start periodic reconciliation to sync with database
	go r.startPeriodicReconciliation(r.ctx)

	return nil
}

// Stop gracefully stops the VDC reconciler
func (r *VDCReconciler) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

// startPeriodicReconciliation runs periodic database sync and stops gracefully when context is cancelled
func (r *VDCReconciler) startPeriodicReconciliation(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.Log.Info("Stopping periodic VDC reconciliation due to context cancellation")
			return
		case <-ticker.C:
			r.Log.V(1).Info("Starting periodic VDC reconciliation")

			// Get all VDCs from database
			vdcs, err := r.VDCRepo.GetAll(ctx)
			if err != nil {
				r.Log.Error(err, "Failed to get VDCs from database")
				continue
			}

			// Reconcile each VDC
			for _, vdc := range vdcs {
				req := reconcile.Request{
					NamespacedName: client.ObjectKey{
						Name: fmt.Sprintf("vdc-%s", vdc.ID.String()),
					},
				}
				if _, err := r.Reconcile(ctx, req); err != nil {
					r.Log.Error(err, "Failed to reconcile VDC", "vdc_id", vdc.ID)
				}
			}
		}
	}
}

// Reconcile implements the reconciliation logic
func (r *VDCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("vdc", req.NamespacedName)

	// Handle periodic reconciliation requests (these have a specific name format)
	if req.Name != "" && len(req.Name) > 4 && req.Name[:4] == "vdc-" {
		vdcID := req.Name[4:] // Remove "vdc-" prefix
		return r.reconcileVDCByID(ctx, log, vdcID)
	}

	// Handle namespace-triggered reconciliation
	if req.Namespace == "" && req.Name != "" {
		return r.reconcileNamespace(ctx, log, req.Name)
	}

	return ctrl.Result{}, nil
}

// reconcileVDCByID reconciles a specific VDC by ID
func (r *VDCReconciler) reconcileVDCByID(ctx context.Context, log logr.Logger, vdcIDStr string) (ctrl.Result, error) {
	// Get VDC from database
	vdc, err := r.VDCRepo.GetByIDString(ctx, vdcIDStr)
	if err != nil {
		log.Error(err, "Failed to get VDC from database", "vdc_id", vdcIDStr)
		return ctrl.Result{RequeueAfter: r.interval}, nil
	}

	if vdc == nil {
		log.V(1).Info("VDC not found in database, may have been deleted", "vdc_id", vdcIDStr)
		return ctrl.Result{}, nil
	}

	return r.reconcileVDC(ctx, log, vdc)
}

// reconcileNamespace handles namespace events
func (r *VDCReconciler) reconcileNamespace(ctx context.Context, log logr.Logger, namespaceName string) (ctrl.Result, error) {
	// Check if this namespace belongs to a VDC (starts with vdc-)
	if len(namespaceName) < 4 || namespaceName[:4] != "vdc-" {
		// Not a VDC namespace, ignore
		return ctrl.Result{}, nil
	}

	// Check if this namespace exists in database
	vdc, err := r.VDCRepo.GetByNamespace(ctx, namespaceName)
	if err != nil {
		log.Error(err, "Failed to query VDC by namespace", "namespace", namespaceName)
		return ctrl.Result{}, nil
	}

	if vdc == nil {
		// VDC namespace not found in database, may need cleanup
		return r.handleOrphanedNamespace(ctx, log, namespaceName)
	}

	return r.reconcileVDC(ctx, log, vdc)
}

// reconcileVDC performs the main reconciliation logic for a VDC
func (r *VDCReconciler) reconcileVDC(ctx context.Context, log logr.Logger, vdc *models.VDC) (ctrl.Result, error) {
	// Load organization for context
	org, err := r.OrgRepo.GetByIDWithContext(ctx, vdc.OrganizationID)
	if err != nil {
		log.Error(err, "Failed to load organization for VDC", "vdc_id", vdc.ID, "org_id", vdc.OrganizationID)
		return ctrl.Result{RequeueAfter: r.interval}, nil
	}

	if org == nil {
		log.Error(nil, "Organization not found for VDC", "vdc_id", vdc.ID, "org_id", vdc.OrganizationID)
		return ctrl.Result{RequeueAfter: r.interval}, nil
	}

	log = log.WithValues("vdc_id", vdc.ID, "vdc_name", vdc.Name, "org_name", org.Name, "namespace", vdc.NamespaceName)

	if vdc.DeletedAt.Valid {
		// VDC is marked for deletion
		return r.handleVDCDeletion(ctx, log, vdc)
	}

	if !vdc.Enabled {
		// VDC is disabled, ensure namespace is also disabled/removed
		return r.handleVDCDisabled(ctx, log, vdc)
	}

	// Ensure namespace exists and is properly configured
	return r.ensureNamespaceExists(ctx, log, vdc, org)
}

// handleVDCDeletion removes the associated namespace
func (r *VDCReconciler) handleVDCDeletion(ctx context.Context, log logr.Logger, vdc *models.VDC) (ctrl.Result, error) {
	if vdc.NamespaceName == "" {
		// No namespace to clean up
		return ctrl.Result{}, nil
	}

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: vdc.NamespaceName}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace already deleted
			log.Info("Namespace already deleted for VDC")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}

	// Delete the namespace
	log.Info("Deleting namespace for deleted VDC")
	err = r.Delete(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to delete namespace")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleVDCDisabled removes or marks the namespace as disabled
func (r *VDCReconciler) handleVDCDisabled(ctx context.Context, log logr.Logger, vdc *models.VDC) (ctrl.Result, error) {
	if vdc.NamespaceName == "" {
		// No namespace to handle
		return ctrl.Result{}, nil
	}

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: vdc.NamespaceName}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace doesn't exist, which is fine for disabled VDC
			log.Info("Namespace does not exist for disabled VDC")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}

	// Add disabled annotation to namespace
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	if namespace.Annotations["ssvirt.io/vdc-disabled"] != "true" {
		namespace.Annotations["ssvirt.io/vdc-disabled"] = "true"
		log.Info("Marking namespace as disabled for disabled VDC")

		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to update namespace with disabled annotation")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ensureNamespaceExists creates or updates the namespace for an enabled VDC
func (r *VDCReconciler) ensureNamespaceExists(ctx context.Context, log logr.Logger, vdc *models.VDC, org *models.Organization) (ctrl.Result, error) {
	if vdc.NamespaceName == "" {
		log.Error(nil, "VDC has empty namespace_name field")
		return ctrl.Result{}, fmt.Errorf("VDC %s has empty namespace_name field", vdc.ID)
	}

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: vdc.NamespaceName}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create the namespace
			return r.createNamespace(ctx, log, vdc, org)
		}
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}

	// Namespace exists, ensure it's properly configured
	return r.updateNamespace(ctx, log, vdc, org, namespace)
}

// createNamespace creates a new namespace for the VDC
func (r *VDCReconciler) createNamespace(ctx context.Context, log logr.Logger, vdc *models.VDC, org *models.Organization) (ctrl.Result, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: vdc.NamespaceName,
			Labels: map[string]string{
				"ssvirt.io/organization":                   org.Name,
				"ssvirt.io/organization-id":                org.ID.String(),
				"ssvirt.io/vdc":                            vdc.Name,
				"ssvirt.io/vdc-id":                         vdc.ID.String(),
				"k8s.ovn.org/primary-user-defined-network": "",
				"app.kubernetes.io/managed-by":             "ssvirt",
			},
			Annotations: map[string]string{
				"ssvirt.io/organization-display-name": org.DisplayName,
				"ssvirt.io/organization-description":  org.Description,
			},
		},
	}

	log.Info("Creating namespace for VDC")
	err := r.Create(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to create namespace")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created namespace for VDC")

	// Create resource quota for the VDC
	if err := r.createResourceQuota(ctx, log, vdc, namespace); err != nil {
		log.Error(err, "Failed to create resource quota")
		return ctrl.Result{}, err
	}

	// Create network policies for isolation
	if err := r.createNetworkPolicies(ctx, log, vdc, namespace); err != nil {
		log.Error(err, "Failed to create network policies")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateNamespace updates an existing namespace with current VDC metadata
func (r *VDCReconciler) updateNamespace(ctx context.Context, log logr.Logger, vdc *models.VDC, org *models.Organization, namespace *corev1.Namespace) (ctrl.Result, error) {
	updated := false

	// Ensure labels are set correctly
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	expectedLabels := map[string]string{
		"ssvirt.io/organization":                   org.Name,
		"ssvirt.io/organization-id":                org.ID.String(),
		"ssvirt.io/vdc":                            vdc.Name,
		"ssvirt.io/vdc-id":                         vdc.ID.String(),
		"k8s.ovn.org/primary-user-defined-network": "",
		"app.kubernetes.io/managed-by":             "ssvirt",
	}

	for key, value := range expectedLabels {
		if namespace.Labels[key] != value {
			namespace.Labels[key] = value
			updated = true
		}
	}

	// Ensure annotations are set correctly
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	expectedAnnotations := map[string]string{
		"ssvirt.io/organization-display-name": org.DisplayName,
		"ssvirt.io/organization-description":  org.Description,
	}

	for key, value := range expectedAnnotations {
		if namespace.Annotations[key] != value {
			namespace.Annotations[key] = value
			updated = true
		}
	}

	// Remove disabled annotation if VDC is enabled
	if namespace.Annotations["ssvirt.io/vdc-disabled"] == "true" {
		delete(namespace.Annotations, "ssvirt.io/vdc-disabled")
		updated = true
	}

	if updated {
		log.Info("Updating namespace metadata for VDC")
		err := r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to update namespace")
			return ctrl.Result{}, err
		}
		log.Info("Successfully updated namespace for VDC")
	}

	// Ensure resource quota is up to date
	if err := r.reconcileResourceQuota(ctx, log, vdc, namespace); err != nil {
		log.Error(err, "Failed to reconcile resource quota")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// createResourceQuota creates a resource quota for the VDC namespace
func (r *VDCReconciler) createResourceQuota(ctx context.Context, log logr.Logger, vdc *models.VDC, namespace *corev1.Namespace) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-quota",
			Namespace: namespace.Name,
			Labels: map[string]string{
				"ssvirt.io/vdc-id":             vdc.ID.String(),
				"app.kubernetes.io/managed-by": "ssvirt",
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{},
		},
	}

	// Set CPU limit if specified
	if vdc.CPULimit != nil && *vdc.CPULimit > 0 {
		quota.Spec.Hard[corev1.ResourceRequestsCPU] = resource.MustParse(fmt.Sprintf("%d", *vdc.CPULimit))
		quota.Spec.Hard[corev1.ResourceLimitsCPU] = resource.MustParse(fmt.Sprintf("%d", *vdc.CPULimit))
	}

	// Set memory limit if specified
	if vdc.MemoryLimitMB != nil && *vdc.MemoryLimitMB > 0 {
		memoryLimit := fmt.Sprintf("%dMi", *vdc.MemoryLimitMB)
		quota.Spec.Hard[corev1.ResourceRequestsMemory] = resource.MustParse(memoryLimit)
		quota.Spec.Hard[corev1.ResourceLimitsMemory] = resource.MustParse(memoryLimit)
	}

	// Set storage limit if specified
	if vdc.StorageLimitMB != nil && *vdc.StorageLimitMB > 0 {
		storageLimit := fmt.Sprintf("%dMi", *vdc.StorageLimitMB)
		quota.Spec.Hard[corev1.ResourceRequestsStorage] = resource.MustParse(storageLimit)
		quota.Spec.Hard[corev1.ResourcePersistentVolumeClaims] = resource.MustParse("20") // Allow up to 20 PVCs
	}

	log.Info("Creating resource quota for VDC namespace")
	return r.Create(ctx, quota)
}

// reconcileResourceQuota updates the resource quota for the VDC
func (r *VDCReconciler) reconcileResourceQuota(ctx context.Context, log logr.Logger, vdc *models.VDC, namespace *corev1.Namespace) error {
	quota := &corev1.ResourceQuota{}
	err := r.Get(ctx, client.ObjectKey{Name: "vdc-quota", Namespace: namespace.Name}, quota)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create the quota
			return r.createResourceQuota(ctx, log, vdc, namespace)
		}
		return err
	}

	// Update quota specifications
	updated := false
	newHard := corev1.ResourceList{}

	// Set CPU limit if specified
	if vdc.CPULimit != nil && *vdc.CPULimit > 0 {
		cpuQuantity := resource.MustParse(fmt.Sprintf("%d", *vdc.CPULimit))
		newHard[corev1.ResourceRequestsCPU] = cpuQuantity
		newHard[corev1.ResourceLimitsCPU] = cpuQuantity
	}

	// Set memory limit if specified
	if vdc.MemoryLimitMB != nil && *vdc.MemoryLimitMB > 0 {
		memoryQuantity := resource.MustParse(fmt.Sprintf("%dMi", *vdc.MemoryLimitMB))
		newHard[corev1.ResourceRequestsMemory] = memoryQuantity
		newHard[corev1.ResourceLimitsMemory] = memoryQuantity
	}

	// Set storage limit if specified
	if vdc.StorageLimitMB != nil && *vdc.StorageLimitMB > 0 {
		storageQuantity := resource.MustParse(fmt.Sprintf("%dMi", *vdc.StorageLimitMB))
		newHard[corev1.ResourceRequestsStorage] = storageQuantity
		newHard[corev1.ResourcePersistentVolumeClaims] = resource.MustParse("20")
	}

	// Check if quota needs updating
	for resource, quantity := range newHard {
		if existing, exists := quota.Spec.Hard[resource]; !exists || !existing.Equal(quantity) {
			updated = true
			break
		}
	}

	if updated {
		quota.Spec.Hard = newHard
		log.Info("Updating resource quota for VDC namespace")
		return r.Update(ctx, quota)
	}

	return nil
}

// createNetworkPolicies creates network policies for VDC isolation
func (r *VDCReconciler) createNetworkPolicies(ctx context.Context, log logr.Logger, vdc *models.VDC, namespace *corev1.Namespace) error {
	// Create a network policy that denies all ingress and egress by default
	// This provides VDC-level isolation
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-isolation",
			Namespace: namespace.Name,
			Labels: map[string]string{
				"ssvirt.io/vdc-id":             vdc.ID.String(),
				"app.kubernetes.io/managed-by": "ssvirt",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			PodSelector: metav1.LabelSelector{}, // Apply to all pods in namespace
			// Empty ingress and egress rules means deny all by default
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow DNS resolution
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"name": "openshift-dns",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
					},
				},
			},
		},
	}

	log.Info("Creating network policy for VDC namespace")
	return r.Create(ctx, policy)
}

// handleOrphanedNamespace handles namespaces that exist in Kubernetes but not in database
func (r *VDCReconciler) handleOrphanedNamespace(ctx context.Context, log logr.Logger, namespaceName string) (ctrl.Result, error) {
	// Check if this is actually a VDC namespace that should be managed
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace already deleted
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get orphaned namespace")
		return ctrl.Result{}, err
	}

	// Check if this namespace is managed by ssvirt
	if managedBy, exists := namespace.Labels["app.kubernetes.io/managed-by"]; !exists || managedBy != "ssvirt" {
		// Not managed by ssvirt, ignore
		return ctrl.Result{}, nil
	}

	// This is an orphaned ssvirt VDC namespace, mark for cleanup
	log.Info("Found orphaned VDC namespace, marking for cleanup", "namespace", namespaceName)

	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	namespace.Annotations["ssvirt.io/orphaned"] = "true"
	namespace.Annotations["ssvirt.io/orphaned-timestamp"] = time.Now().Format(time.RFC3339)

	err = r.Update(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to mark namespace as orphaned")
		return ctrl.Result{}, err
	}

	// TODO: Implement cleanup logic - for now just mark it
	// In a future iteration, we could delete orphaned namespaces after a grace period

	return ctrl.Result{}, nil
}
