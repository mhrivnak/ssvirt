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
