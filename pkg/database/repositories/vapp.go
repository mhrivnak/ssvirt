package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/pagination"
)

// ErrVAppHasRunningVMs is returned when attempting to delete a vApp that contains running VMs
var ErrVAppHasRunningVMs = errors.New("vApp contains running VMs")

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
	err := r.db.Preload("VDC").Preload("VDC.Organization").Preload("Template").Preload("VMs").Where("id = ?", id).First(&vapp).Error
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

// Context-aware methods for VM creation API

// CreateWithContext creates a new vApp with context support
func (r *VAppRepository) CreateWithContext(ctx context.Context, vapp *models.VApp) error {
	return r.db.WithContext(ctx).Create(vapp).Error
}

// UpdateWithContext updates an existing vApp with context support
func (r *VAppRepository) UpdateWithContext(ctx context.Context, vapp *models.VApp) error {
	return r.db.WithContext(ctx).Save(vapp).Error
}

// GetByIDString retrieves a vApp by its string ID
func (r *VAppRepository) GetByIDString(ctx context.Context, id string) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

// GetWithVDC retrieves a vApp with its VDC information for access control
func (r *VAppRepository) GetWithVDC(ctx context.Context, vappID string) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.WithContext(ctx).
		Preload("VDC").
		Where("id = ?", vappID).
		First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

// ExistsByNameInVDC checks if a vApp with the given name exists in the specified VDC
func (r *VAppRepository) ExistsByNameInVDC(ctx context.Context, vdcID, name string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.VApp{}).
		Where("vdc_id = ? AND name = ?", vdcID, name).
		Count(&count).Error
	return count > 0, err
}

// ListByVDCWithPagination retrieves vApps for a VDC with pagination, filtering, and sorting
func (r *VAppRepository) ListByVDCWithPagination(ctx context.Context, vdcID string, limit, offset int, filter, sortOrder string) ([]models.VApp, error) {
	var vapps []models.VApp
	query := r.db.WithContext(ctx).Preload("VMs").Where("vdc_id = ?", vdcID)

	// Apply filter if provided
	if filter != "" {
		query = r.applyFilter(query, filter)
	}

	// Sanitize and validate pagination parameters
	limit, offset = pagination.ClampPaginationParams(limit, offset)
	sortOrder = pagination.SanitizeSortOrder(sortOrder, pagination.VAppSortColumns, "created_at DESC, id DESC")

	err := query.Limit(limit).Offset(offset).Order(sortOrder).Find(&vapps).Error
	return vapps, err
}

// CountByVDC returns the total count of vApps in a VDC (for pagination)
func (r *VAppRepository) CountByVDC(ctx context.Context, vdcID string, filter string) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.VApp{}).Where("vdc_id = ?", vdcID)

	// Apply filter if provided
	if filter != "" {
		query = r.applyFilter(query, filter)
	}

	err := query.Count(&count).Error
	return count, err
}

// GetWithVMsString retrieves a vApp with its VMs using string ID
func (r *VAppRepository) GetWithVMsString(ctx context.Context, vappID string) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.WithContext(ctx).
		Preload("VMs").
		Where("id = ?", vappID).
		First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

// DeleteWithValidation deletes a vApp after checking for dependencies
func (r *VAppRepository) DeleteWithValidation(ctx context.Context, vappID string, force bool) error {
	// Use a transaction to ensure atomicity
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get vApp with VMs
		var vapp models.VApp
		err := tx.Preload("VMs").Where("id = ?", vappID).First(&vapp).Error
		if err != nil {
			return err
		}

		// Check if VMs are powered on (if force is false)
		hasRunningVMs := false
		for _, vm := range vapp.VMs {
			if vm.Status == "POWERED_ON" {
				hasRunningVMs = true
				break
			}
		}

		if hasRunningVMs && !force {
			return ErrVAppHasRunningVMs
		}

		// If force is true and there are running VMs, power them off first
		if hasRunningVMs && force {
			err = tx.Model(&models.VM{}).Where("vapp_id = ? AND status = ?", vappID, "POWERED_ON").Update("status", "POWERED_OFF").Error
			if err != nil {
				return fmt.Errorf("failed to power off VMs: %w", err)
			}
		}

		// Delete VMs first
		if len(vapp.VMs) > 0 {
			err = tx.Where("vapp_id = ?", vappID).Delete(&models.VM{}).Error
			if err != nil {
				return fmt.Errorf("failed to delete VMs: %w", err)
			}
		}

		// Delete the vApp
		return tx.Where("id = ?", vappID).Delete(&models.VApp{}).Error
	})
}

// applyFilter applies VMware Cloud Director API filter syntax to a query
// Supports 'attribute==value' syntax for exact matches
func (r *VAppRepository) applyFilter(query *gorm.DB, filter string) *gorm.DB {
	// Check if filter uses 'attribute==value' syntax
	if strings.Contains(filter, "==") {
		parts := strings.SplitN(filter, "==", 2)
		if len(parts) == 2 {
			attribute := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Validate allowed filter attributes
			switch attribute {
			case "name":
				return query.Where("name = ?", value)
			case "status":
				return query.Where("status = ?", value)
			case "description":
				return query.Where("description = ?", value)
			default:
				// Invalid attribute, fall back to name substring matching using the value part
				return query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", value))
			}
		}
	}

	// Fall back to simple name substring matching for backward compatibility
	return query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", filter))
}

// Controller-specific methods

// GetByNameInVDC finds a VApp by name within a specific VDC (for controller)
func (r *VAppRepository) GetByNameInVDC(ctx context.Context, vdcID, name string) (*models.VApp, error) {
	var vapp models.VApp
	err := r.db.WithContext(ctx).
		Where("vdc_id = ? AND name = ?", vdcID, name).
		First(&vapp).Error
	if err != nil {
		return nil, err
	}
	return &vapp, nil
}

// CreateVApp creates a new VApp record (for controller)
func (r *VAppRepository) CreateVApp(ctx context.Context, vapp *models.VApp) error {
	return r.db.WithContext(ctx).Create(vapp).Error
}

// UpdateStatus updates only the status field of a VApp (for controller)
func (r *VAppRepository) UpdateStatus(ctx context.Context, vappID string, status string) error {
	result := r.db.WithContext(ctx).
		Model(&models.VApp{}).
		Where("id = ?", vappID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// No rows affected could mean either:
		// 1. vApp doesn't exist, or
		// 2. vApp exists but status was unchanged
		// Perform existence check to distinguish between these cases
		var count int64
		err := r.db.WithContext(ctx).
			Model(&models.VApp{}).
			Where("id = ?", vappID).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		// vApp exists but status was unchanged - this is a no-op, return nil
		return nil
	}
	return nil
}
