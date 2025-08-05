package repositories

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type VDCRepository struct {
	db *gorm.DB
}

func NewVDCRepository(db *gorm.DB) *VDCRepository {
	return &VDCRepository{db: db}
}

func (r *VDCRepository) Create(vdc *models.VDC) error {
	if vdc == nil {
		return errors.New("VDC cannot be nil")
	}
	return r.db.Create(vdc).Error
}

func (r *VDCRepository) GetByID(id uuid.UUID) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Where("id = ?", id).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

func (r *VDCRepository) GetByOrganizationID(orgID uuid.UUID) ([]models.VDC, error) {
	var vdcs []models.VDC
	err := r.db.Where("organization_id = ?", orgID).Find(&vdcs).Error
	return vdcs, err
}

func (r *VDCRepository) List() ([]models.VDC, error) {
	var vdcs []models.VDC
	err := r.db.Find(&vdcs).Error
	return vdcs, err
}

func (r *VDCRepository) Update(vdc *models.VDC) error {
	if vdc == nil {
		return errors.New("VDC cannot be nil")
	}
	return r.db.Save(vdc).Error
}

func (r *VDCRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.VDC{}, id).Error
}

func (r *VDCRepository) GetWithVApps(id uuid.UUID) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Preload("VApps").Where("id = ?", id).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

func (r *VDCRepository) GetWithOrganization(id uuid.UUID) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Preload("Organization").Where("id = ?", id).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}
