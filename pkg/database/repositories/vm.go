package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type VMRepository struct {
	db *gorm.DB
}

func NewVMRepository(db *gorm.DB) *VMRepository {
	return &VMRepository{db: db}
}

func (r *VMRepository) Create(vm *models.VM) error {
	return r.db.Create(vm).Error
}

func (r *VMRepository) GetByID(id uuid.UUID) (*models.VM, error) {
	var vm models.VM
	err := r.db.Where("id = ?", id).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) GetByVAppID(vappID uuid.UUID) ([]models.VM, error) {
	var vms []models.VM
	err := r.db.Where("vapp_id = ?", vappID).Find(&vms).Error
	return vms, err
}

func (r *VMRepository) GetByVMName(vmName, namespace string) (*models.VM, error) {
	var vm models.VM
	err := r.db.Where("vm_name = ? AND namespace = ?", vmName, namespace).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) List() ([]models.VM, error) {
	var vms []models.VM
	err := r.db.Find(&vms).Error
	return vms, err
}

func (r *VMRepository) Update(vm *models.VM) error {
	return r.db.Save(vm).Error
}

func (r *VMRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.VM{}, id).Error
}

func (r *VMRepository) GetWithVApp(id uuid.UUID) (*models.VM, error) {
	var vm models.VM
	err := r.db.Preload("VApp").Where("id = ?", id).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) GetByOrganizationIDs(orgIDs []uuid.UUID) ([]models.VM, error) {
	if len(orgIDs) == 0 {
		return []models.VM{}, nil
	}

	var vms []models.VM
	err := r.db.Preload("VApp").Preload("VApp.VDC").Preload("VApp.VDC.Organization").
		Joins("JOIN v_apps ON vms.v_app_id = v_apps.id").
		Joins("JOIN vdcs ON v_apps.vdc_id = vdcs.id").
		Where("vdcs.organization_id IN ?", orgIDs).
		Find(&vms).Error
	return vms, err
}

func (r *VMRepository) GetByOrganizationIDsWithFilters(orgIDs []uuid.UUID, vappID *uuid.UUID, status string, limit, offset int) ([]models.VM, int64, error) {
	if len(orgIDs) == 0 {
		return []models.VM{}, 0, nil
	}

	// Build the base query
	query := r.db.Preload("VApp").Preload("VApp.VDC").Preload("VApp.VDC.Organization").
		Joins("JOIN v_apps ON vms.v_app_id = v_apps.id").
		Joins("JOIN vdcs ON v_apps.vdc_id = vdcs.id").
		Where("vdcs.organization_id IN ?", orgIDs)

	// Apply filters
	if vappID != nil {
		query = query.Where("vms.v_app_id = ?", *vappID)
	}
	if status != "" {
		query = query.Where("vms.status = ?", status)
	}

	// Count total records
	var total int64
	countQuery := r.db.Table("vms").
		Joins("JOIN v_apps ON vms.v_app_id = v_apps.id").
		Joins("JOIN vdcs ON v_apps.vdc_id = vdcs.id").
		Where("vdcs.organization_id IN ?", orgIDs)

	if vappID != nil {
		countQuery = countQuery.Where("vms.v_app_id = ?", *vappID)
	}
	if status != "" {
		countQuery = countQuery.Where("vms.status = ?", status)
	}

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var vms []models.VM
	err := query.Find(&vms).Error
	return vms, total, err
}
