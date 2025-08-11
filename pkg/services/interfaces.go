package services

import (
	"context"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// TemplateServiceInterface defines the interface for template service operations
type TemplateServiceInterface interface {
	ListCatalogItems(ctx context.Context, catalogID string, limit, offset int) ([]models.CatalogItem, error)
	CountCatalogItems(ctx context.Context, catalogID string) (int64, error)
	GetCatalogItem(ctx context.Context, catalogID, itemID string) (*models.CatalogItem, error)
	Start(ctx context.Context) error
}

// KubernetesServiceInterface defines the interface for Kubernetes operations
type KubernetesServiceInterface interface {
	KubernetesService
}
