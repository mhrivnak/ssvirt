package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// ErrCatalogHasDependencies is returned when attempting to delete a catalog that has dependent vApp templates
var ErrCatalogHasDependencies = errors.New("catalog has dependent vApp templates")

type CatalogRepository struct {
	db *gorm.DB
}

func NewCatalogRepository(db *gorm.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) Create(catalog *models.Catalog) error {
	if catalog == nil {
		return errors.New("catalog cannot be nil")
	}
	return r.db.Create(catalog).Error
}

func (r *CatalogRepository) GetByID(id string) (*models.Catalog, error) {
	var catalog models.Catalog
	err := r.db.Where("id = ?", id).First(&catalog).Error
	if err != nil {
		return nil, err
	}
	return &catalog, nil
}

func (r *CatalogRepository) GetByOrganizationID(orgID string) ([]models.Catalog, error) {
	var catalogs []models.Catalog
	err := r.db.Where("(organization_id = ? OR is_published = true)", orgID).Find(&catalogs).Error
	return catalogs, err
}

func (r *CatalogRepository) GetByOrganizationIDs(orgIDs []string) ([]models.Catalog, error) {
	var catalogs []models.Catalog
	if len(orgIDs) == 0 {
		// If user has no organization access, only return published catalogs
		err := r.db.Where("is_published = true").Find(&catalogs).Error
		return catalogs, err
	}
	err := r.db.Where("(organization_id IN ? OR is_published = true)", orgIDs).Find(&catalogs).Error
	return catalogs, err
}

func (r *CatalogRepository) List() ([]models.Catalog, error) {
	var catalogs []models.Catalog
	err := r.db.Find(&catalogs).Error
	return catalogs, err
}

func (r *CatalogRepository) Update(catalog *models.Catalog) error {
	if catalog == nil {
		return errors.New("catalog cannot be nil")
	}
	return r.db.Save(catalog).Error
}

func (r *CatalogRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.Catalog{}).Error
}

func (r *CatalogRepository) GetWithTemplates(id string) (*models.Catalog, error) {
	var catalog models.Catalog
	err := r.db.Preload("VAppTemplates").Where("id = ?", id).First(&catalog).Error
	if err != nil {
		return nil, err
	}
	return &catalog, nil
}

// VCD-compliant repository methods

// ListWithPagination retrieves catalogs with pagination
func (r *CatalogRepository) ListWithPagination(limit, offset int) ([]models.Catalog, error) {
	var catalogs []models.Catalog
	err := r.db.Preload("VAppTemplates").
		Limit(limit).
		Offset(offset).
		Order("created_at DESC, id DESC").
		Find(&catalogs).Error
	return catalogs, err
}

// CountAll returns the total count of catalogs
func (r *CatalogRepository) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&models.Catalog{}).Count(&count).Error
	return count, err
}

// GetByURN retrieves a catalog by its URN
func (r *CatalogRepository) GetByURN(urn string) (*models.Catalog, error) {
	var catalog models.Catalog
	err := r.db.Preload("VAppTemplates").Where("id = ?", urn).First(&catalog).Error
	if err != nil {
		return nil, err
	}
	return &catalog, nil
}

// GetWithCounts retrieves a catalog by ID with template counts preloaded
func (r *CatalogRepository) GetWithCounts(id string) (*models.Catalog, error) {
	var catalog models.Catalog
	err := r.db.Preload("VAppTemplates").Where("id = ?", id).First(&catalog).Error
	if err != nil {
		return nil, err
	}
	return &catalog, nil
}

// HasDependentTemplates checks if a catalog has dependent vApp templates
func (r *CatalogRepository) HasDependentTemplates(catalogID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.VAppTemplate{}).Where("catalog_id = ?", catalogID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteWithValidation deletes a catalog after checking for dependencies atomically
func (r *CatalogRepository) DeleteWithValidation(urn string) error {
	// Use a transaction to ensure atomicity
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check for dependent vApp templates within the transaction
		var count int64
		err := tx.Model(&models.VAppTemplate{}).Where("catalog_id = ?", urn).Count(&count).Error
		if err != nil {
			return err
		}

		if count > 0 {
			return ErrCatalogHasDependencies
		}

		// Delete the catalog within the same transaction
		return tx.Where("id = ?", urn).Delete(&models.Catalog{}).Error
	})
}

// ValidateUserCatalogAccess checks if a user has access to any catalogs for template instantiation
func (r *CatalogRepository) ValidateUserCatalogAccess(ctx context.Context, userID string) error {
	// Get the user's organization ID using a subquery approach similar to VDC access
	subquery := r.db.WithContext(ctx).Model(&models.User{}).Select("organization_id").Where("id = ? AND organization_id IS NOT NULL", userID)

	// Check if user has access to any catalogs in their organization or published catalogs
	var catalogCount int64
	err := r.db.WithContext(ctx).
		Model(&models.Catalog{}).
		Where("organization_id IN (?) OR is_published = true", subquery).
		Count(&catalogCount).Error
	if err != nil {
		return err
	}

	if catalogCount == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
