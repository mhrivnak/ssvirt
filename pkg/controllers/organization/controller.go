package organization

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// OrganizationReconciler reconciles Organization database records with Kubernetes namespaces
type OrganizationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	OrgRepo  *repositories.OrganizationRepository
	interval time.Duration
}

// NewOrganizationReconciler creates a new OrganizationReconciler
func NewOrganizationReconciler(client client.Client, scheme *runtime.Scheme, log logr.Logger, orgRepo *repositories.OrganizationRepository) *OrganizationReconciler {
	return &OrganizationReconciler{
		Client:   client,
		Scheme:   scheme,
		Log:      log,
		OrgRepo:  orgRepo,
		interval: 30 * time.Second, // Poll database every 30 seconds
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a controller builder and watch namespace changes
	err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Named("organization").
		Complete(r)

	if err != nil {
		return err
	}

	// Start periodic reconciliation to sync with database
	go r.startPeriodicReconciliation()

	return nil
}

// startPeriodicReconciliation runs periodic database sync
func (r *OrganizationReconciler) startPeriodicReconciliation() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for range ticker.C {
		r.Log.V(1).Info("Starting periodic organization reconciliation")

		// Get all organizations from database
		orgs, err := r.OrgRepo.GetAll(context.Background())
		if err != nil {
			r.Log.Error(err, "Failed to get organizations from database")
			continue
		}

		// Reconcile each organization
		for _, org := range orgs {
			req := reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: fmt.Sprintf("org-%s", org.ID.String()),
				},
			}
			if _, err := r.Reconcile(context.Background(), req); err != nil {
				r.Log.Error(err, "Failed to reconcile organization", "org_id", org.ID)
			}
		}
	}
}

// Reconcile implements the reconciliation logic
func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("organization", req.NamespacedName)

	// Handle periodic reconciliation requests (these have a specific name format)
	if req.Name != "" && len(req.Name) > 4 && req.Name[:4] == "org-" {
		orgID := req.Name[4:] // Remove "org-" prefix
		return r.reconcileOrganizationByID(ctx, log, orgID)
	}

	// Handle namespace-triggered reconciliation
	if req.Namespace == "" && req.Name != "" {
		return r.reconcileNamespace(ctx, log, req.Name)
	}

	return ctrl.Result{}, nil
}

// reconcileOrganizationByID reconciles a specific organization by ID
func (r *OrganizationReconciler) reconcileOrganizationByID(ctx context.Context, log logr.Logger, orgIDStr string) (ctrl.Result, error) {
	// Get organization from database
	org, err := r.OrgRepo.GetByIDString(ctx, orgIDStr)
	if err != nil {
		log.Error(err, "Failed to get organization from database", "org_id", orgIDStr)
		return ctrl.Result{RequeueAfter: r.interval}, nil
	}

	if org == nil {
		log.V(1).Info("Organization not found in database, may have been deleted", "org_id", orgIDStr)
		return ctrl.Result{}, nil
	}

	return r.reconcileOrganization(ctx, log, org)
}

// reconcileNamespace handles namespace events
func (r *OrganizationReconciler) reconcileNamespace(ctx context.Context, log logr.Logger, namespaceName string) (ctrl.Result, error) {
	// Check if this namespace belongs to an organization
	org, err := r.OrgRepo.GetByNamespace(ctx, namespaceName)
	if err != nil {
		log.Error(err, "Failed to query organization by namespace", "namespace", namespaceName)
		return ctrl.Result{}, nil
	}

	if org == nil {
		// Not an organization namespace, ignore
		return ctrl.Result{}, nil
	}

	return r.reconcileOrganization(ctx, log, org)
}

// reconcileOrganization performs the main reconciliation logic for an organization
func (r *OrganizationReconciler) reconcileOrganization(ctx context.Context, log logr.Logger, org *models.Organization) (ctrl.Result, error) {
	log = log.WithValues("org_id", org.ID, "org_name", org.Name, "namespace", org.Namespace)

	if org.DeletedAt.Valid {
		// Organization is marked for deletion
		return r.handleOrganizationDeletion(ctx, log, org)
	}

	if !org.Enabled {
		// Organization is disabled, ensure namespace is also disabled/removed
		return r.handleOrganizationDisabled(ctx, log, org)
	}

	// Ensure namespace exists and is properly configured
	return r.ensureNamespaceExists(ctx, log, org)
}

// handleOrganizationDeletion removes the associated namespace
func (r *OrganizationReconciler) handleOrganizationDeletion(ctx context.Context, log logr.Logger, org *models.Organization) (ctrl.Result, error) {
	if org.Namespace == "" {
		// No namespace to clean up
		return ctrl.Result{}, nil
	}

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: org.Namespace}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace already deleted
			log.Info("Namespace already deleted for organization")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}

	// Delete the namespace
	log.Info("Deleting namespace for deleted organization")
	err = r.Delete(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to delete namespace")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleOrganizationDisabled removes or marks the namespace as disabled
func (r *OrganizationReconciler) handleOrganizationDisabled(ctx context.Context, log logr.Logger, org *models.Organization) (ctrl.Result, error) {
	if org.Namespace == "" {
		// No namespace to handle
		return ctrl.Result{}, nil
	}

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: org.Namespace}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace doesn't exist, which is fine for disabled org
			log.Info("Namespace does not exist for disabled organization")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}

	// Add disabled annotation to namespace
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	if namespace.Annotations["ssvirt.io/organization-disabled"] != "true" {
		namespace.Annotations["ssvirt.io/organization-disabled"] = "true"
		log.Info("Marking namespace as disabled for disabled organization")

		err = r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to update namespace with disabled annotation")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ensureNamespaceExists creates or updates the namespace for an enabled organization
func (r *OrganizationReconciler) ensureNamespaceExists(ctx context.Context, log logr.Logger, org *models.Organization) (ctrl.Result, error) {
	if org.Namespace == "" {
		log.Error(nil, "Organization has empty namespace field")
		return ctrl.Result{}, fmt.Errorf("organization %s has empty namespace field", org.ID)
	}

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: org.Namespace}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create the namespace
			return r.createNamespace(ctx, log, org)
		}
		log.Error(err, "Failed to get namespace")
		return ctrl.Result{}, err
	}

	// Namespace exists, ensure it's properly configured
	return r.updateNamespace(ctx, log, org, namespace)
}

// createNamespace creates a new namespace for the organization
func (r *OrganizationReconciler) createNamespace(ctx context.Context, log logr.Logger, org *models.Organization) (ctrl.Result, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: org.Namespace,
			Labels: map[string]string{
				"ssvirt.io/organization-id":   org.ID.String(),
				"ssvirt.io/organization-name": org.Name,
				"ssvirt.io/managed-by":        "ssvirt-controller",
			},
			Annotations: map[string]string{
				"ssvirt.io/organization-display-name": org.DisplayName,
				"ssvirt.io/organization-description":  org.Description,
			},
		},
	}

	log.Info("Creating namespace for organization")
	err := r.Create(ctx, namespace)
	if err != nil {
		log.Error(err, "Failed to create namespace")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created namespace for organization")
	return ctrl.Result{}, nil
}

// updateNamespace updates an existing namespace with current organization metadata
func (r *OrganizationReconciler) updateNamespace(ctx context.Context, log logr.Logger, org *models.Organization, namespace *corev1.Namespace) (ctrl.Result, error) {
	updated := false

	// Ensure labels are set correctly
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	expectedLabels := map[string]string{
		"ssvirt.io/organization-id":   org.ID.String(),
		"ssvirt.io/organization-name": org.Name,
		"ssvirt.io/managed-by":        "ssvirt-controller",
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

	// Remove disabled annotation if organization is enabled
	if namespace.Annotations["ssvirt.io/organization-disabled"] == "true" {
		delete(namespace.Annotations, "ssvirt.io/organization-disabled")
		updated = true
	}

	if updated {
		log.Info("Updating namespace metadata for organization")
		err := r.Update(ctx, namespace)
		if err != nil {
			log.Error(err, "Failed to update namespace")
			return ctrl.Result{}, err
		}
		log.Info("Successfully updated namespace for organization")
	}

	return ctrl.Result{}, nil
}
