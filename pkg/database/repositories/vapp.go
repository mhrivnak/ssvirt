package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type VAppRepository struct {
	db *gorm.DB
}

func NewVAppRepository(db *gorm.DB) *VAppRepository {
	return &VAppRepository{db: db}
}

func (r *VAppRepository) Create(vapp *models.VApp) error {
	return r.db.Create(vapp).Error
}

func (r *VAppRepository) GetByID(id uuid.UUID) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.Where("id = ?", id).First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

func (r *VAppRepository) GetByVDCID(vdcID uuid.UUID) ([]models.VApp, error) {
	var vapps []models.VApp
	err := r.db.Where("vdc_id = ?", vdcID).Find(&vapps).Error
	return vapps, err
}

func (r *VAppRepository) List() ([]models.VApp, error) {
	var vapps []models.VApp
	err := r.db.Find(&vapps).Error
	return vapps, err
}

func (r *VAppRepository) Update(vapp *models.VApp) error {
	return r.db.Save(vapp).Error
}

func (r *VAppRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.VApp{}, id).Error
}

func (r *VAppRepository) GetWithVMs(id uuid.UUID) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.Preload("VMs").Where("id = ?", id).First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

func (r *VAppRepository) GetWithAll(id uuid.UUID) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.Preload("VDC").Preload("Template").Preload("VMs").Where("id = ?", id).First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

func (r *VAppRepository) GetByOrganizationIDs(orgIDs []uuid.UUID) ([]models.VApp, error) {
	if len(orgIDs) == 0 {
		return []models.VApp{}, nil
	}

	var vapps []models.VApp
	err := r.db.Preload("VDC").Preload("Template").Preload("VMs").
		Joins("JOIN vdcs ON v_apps.vdc_id = vdcs.id").
		Where("vdcs.organization_id IN ?", orgIDs).
		Find(&vapps).Error
	return vapps, err
}