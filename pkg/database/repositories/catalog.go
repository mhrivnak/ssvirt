package repositories

import (
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type CatalogRepository struct {
	db *gorm.DB
}

func NewCatalogRepository(db *gorm.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) Create(catalog *models.Catalog) error {
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
	err := r.db.Where("organization_id = ? OR is_shared = true", orgID).Find(&catalogs).Error
	return catalogs, err
}

func (r *CatalogRepository) GetByOrganizationIDs(orgIDs []string) ([]models.Catalog, error) {
	var catalogs []models.Catalog
	if len(orgIDs) == 0 {
		// If user has no organization access, only return shared catalogs
		err := r.db.Where("is_shared = true").Find(&catalogs).Error
		return catalogs, err
	}
	err := r.db.Where("organization_id IN ? OR is_shared = true", orgIDs).Find(&catalogs).Error
	return catalogs, err
}

func (r *CatalogRepository) List() ([]models.Catalog, error) {
	var catalogs []models.Catalog
	err := r.db.Find(&catalogs).Error
	return catalogs, err
}

func (r *CatalogRepository) Update(catalog *models.Catalog) error {
	return r.db.Save(catalog).Error
}

func (r *CatalogRepository) Delete(id string) error {
	return r.db.Delete(&models.Catalog{}, id).Error
}

func (r *CatalogRepository) GetWithTemplates(id string) (*models.Catalog, error) {
	var catalog models.Catalog
	err := r.db.Preload("VAppTemplates").Where("id = ?", id).First(&catalog).Error
	if err != nil {
		return nil, err
	}
	return &catalog, nil
}
