package repositories

import (
	"context"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/services"
)

// CatalogItemRepository provides access to catalog items backed by OpenShift Templates
type CatalogItemRepository struct {
	templateService services.TemplateServiceInterface
	catalogRepo     *CatalogRepository
}

// NewCatalogItemRepository creates a new CatalogItemRepository
func NewCatalogItemRepository(templateService services.TemplateServiceInterface, catalogRepo *CatalogRepository) *CatalogItemRepository {
	return &CatalogItemRepository{
		templateService: templateService,
		catalogRepo:     catalogRepo,
	}
}

// ListByCatalogID returns paginated catalog items for the specified catalog
func (r *CatalogItemRepository) ListByCatalogID(ctx context.Context, catalogID string, limit, offset int) ([]models.CatalogItem, error) {
	// Verify the catalog exists first
	_, err := r.catalogRepo.GetByID(catalogID)
	if err != nil {
		return nil, err
	}

	// Get catalog items from template service
	return r.templateService.ListCatalogItems(ctx, catalogID, limit, offset)
}

// CountByCatalogID returns the total count of catalog items for the specified catalog
func (r *CatalogItemRepository) CountByCatalogID(ctx context.Context, catalogID string) (int64, error) {
	// Verify the catalog exists first
	_, err := r.catalogRepo.GetByID(catalogID)
	if err != nil {
		return 0, err
	}

	// Get count from template service
	return r.templateService.CountCatalogItems(ctx, catalogID)
}

// GetByID returns a specific catalog item by ID within the specified catalog
func (r *CatalogItemRepository) GetByID(ctx context.Context, catalogID, itemID string) (*models.CatalogItem, error) {
	// Verify the catalog exists first
	_, err := r.catalogRepo.GetByID(catalogID)
	if err != nil {
		return nil, err
	}

	// Get catalog item from template service
	return r.templateService.GetCatalogItem(ctx, catalogID, itemID)
}
