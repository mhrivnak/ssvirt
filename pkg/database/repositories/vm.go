package repositories

import (
	"context"
	"time"

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

func (r *VMRepository) GetByID(id string) (*models.VM, error) {
	var vm models.VM
	err := r.db.Where("id = ?", id).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) GetByVAppID(vappID string) ([]models.VM, error) {
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

func (r *VMRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.VM{}).Error
}

func (r *VMRepository) GetWithVApp(id string) (*models.VM, error) {
	var vm models.VM
	err := r.db.Preload("VApp").Where("id = ?", id).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) GetByOrganizationIDs(orgIDs []string) ([]models.VM, error) {
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

func (r *VMRepository) GetByOrganizationIDsWithFilters(orgIDs []string, vappID *string, status string, limit, offset int) ([]models.VM, int64, error) {
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

func (r *VMRepository) GetByVAppIDWithFilters(vappID string, status string, limit, offset int) ([]models.VM, int64, error) {
	// Build the base query for specific vApp
	query := r.db.Preload("VApp").Preload("VApp.VDC").Preload("VApp.VDC.Organization").
		Where("v_app_id = ?", vappID)

	// Apply status filter
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Count total records
	var total int64
	countQuery := r.db.Table("vms").Where("v_app_id = ?", vappID)
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
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

// Context-aware methods for VM API

// GetWithVAppContext retrieves a VM with its vApp and VDC information for access control
func (r *VMRepository) GetWithVAppContext(ctx context.Context, vmID string) (*models.VM, error) {
	var vm models.VM
	err := r.db.WithContext(ctx).
		Preload("VApp").
		Preload("VApp.VDC").
		Where("id = ?", vmID).
		First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

// Controller-specific methods for VM status synchronization

// GetByNamespaceAndVMName finds a VM by its namespace and VM name (for controller)
func (r *VMRepository) GetByNamespaceAndVMName(ctx context.Context, namespace, vmName string) (*models.VM, error) {
	var vm models.VM
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND vm_name = ?", namespace, vmName).
		First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

// GetByVAppAndVMName finds a VM by its vApp ID and VM name (for controller)
func (r *VMRepository) GetByVAppAndVMName(ctx context.Context, vappID, vmName string) (*models.VM, error) {
	var vm models.VM
	err := r.db.WithContext(ctx).
		Where("v_app_id = ? AND vm_name = ?", vappID, vmName).
		First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

// UpdateStatus updates only the status and updated_at fields of a VM (for controller)
func (r *VMRepository) UpdateStatus(ctx context.Context, vmID string, status string) error {
	result := r.db.WithContext(ctx).
		Model(&models.VM{}).
		Where("id = ?", vmID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CreateVM creates a new VM record (for controller)
func (r *VMRepository) CreateVM(ctx context.Context, vm *models.VM) error {
	return r.db.WithContext(ctx).Create(vm).Error
}
