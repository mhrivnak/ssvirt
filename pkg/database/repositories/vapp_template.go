package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type VAppTemplateRepository struct {
	db *gorm.DB
}

func NewVAppTemplateRepository(db *gorm.DB) *VAppTemplateRepository {
	return &VAppTemplateRepository{db: db}
}

func (r *VAppTemplateRepository) Create(template *models.VAppTemplate) error {
	return r.db.Create(template).Error
}

func (r *VAppTemplateRepository) GetByID(id uuid.UUID) (*models.VAppTemplate, error) {
	var template models.VAppTemplate
	err := r.db.Where("id = ?", id).First(&template).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *VAppTemplateRepository) GetByCatalogID(catalogID uuid.UUID) ([]models.VAppTemplate, error) {
	var templates []models.VAppTemplate
	err := r.db.Where("catalog_id = ?", catalogID).Find(&templates).Error
	return templates, err
}

func (r *VAppTemplateRepository) List() ([]models.VAppTemplate, error) {
	var templates []models.VAppTemplate
	err := r.db.Find(&templates).Error
	return templates, err
}

func (r *VAppTemplateRepository) Update(template *models.VAppTemplate) error {
	return r.db.Save(template).Error
}

func (r *VAppTemplateRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.VAppTemplate{}, id).Error
}

func (r *VAppTemplateRepository) GetWithCatalog(id uuid.UUID) (*models.VAppTemplate, error) {
	var template models.VAppTemplate
	err := r.db.Preload("Catalog").Where("id = ?", id).First(&template).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}
