package organization

import (
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// OrganizationReconciler handles Organization database operations (no Kubernetes resources)
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
	// Organizations are now database-only, no Kubernetes resource watching needed
	r.Log.Info("Organization controller setup - organizations are database-only entities")
	return nil
}

// Organizations are now database-only entities, no Kubernetes resource management needed
