package repositories

import (
	"context"
	"errors"

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

func (r *VDCRepository) GetByID(id string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Where("id = ?", id).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

func (r *VDCRepository) GetByOrganizationID(orgID string) ([]models.VDC, error) {
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

func (r *VDCRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.VDC{}).Error
}

func (r *VDCRepository) GetWithVApps(id string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Preload("VApps").Where("id = ?", id).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

func (r *VDCRepository) GetWithOrganization(id string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Preload("Organization").Where("id = ?", id).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

// GetAll retrieves all VDCs from the database
func (r *VDCRepository) GetAll(ctx context.Context) ([]models.VDC, error) {
	var vdcs []models.VDC
	err := r.db.WithContext(ctx).Find(&vdcs).Error
	return vdcs, err
}

// GetByIDString retrieves a VDC by its ID string.
// Returns (nil, nil) when the record is not found.
func (r *VDCRepository) GetByIDString(ctx context.Context, idStr string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.WithContext(ctx).Where("id = ?", idStr).First(&vdc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &vdc, nil
}

// GetByNamespace retrieves a VDC by its namespace name.
// Returns (nil, nil) when the record is not found.
func (r *VDCRepository) GetByNamespace(ctx context.Context, namespaceName string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.WithContext(ctx).Where("namespace = ?", namespaceName).First(&vdc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &vdc, nil
}
