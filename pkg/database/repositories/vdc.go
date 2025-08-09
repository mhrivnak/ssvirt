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

// VCD-compliant repository methods

// ListByOrgWithPagination retrieves VDCs for an organization with pagination
func (r *VDCRepository) ListByOrgWithPagination(orgID string, limit, offset int) ([]models.VDC, error) {
	var vdcs []models.VDC
	err := r.db.Where("organization_id = ?", orgID).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC, id DESC").
		Find(&vdcs).Error
	return vdcs, err
}

// CountByOrganization returns the total count of VDCs in an organization
func (r *VDCRepository) CountByOrganization(orgID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.VDC{}).Where("organization_id = ?", orgID).Count(&count).Error
	return count, err
}

// GetByURN retrieves a VDC by its URN ID
func (r *VDCRepository) GetByURN(urn string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Where("id = ?", urn).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

// GetByOrgAndVDCURN retrieves a VDC by organization URN and VDC URN
func (r *VDCRepository) GetByOrgAndVDCURN(orgURN, vdcURN string) (*models.VDC, error) {
	var vdc models.VDC
	err := r.db.Where("organization_id = ? AND id = ?", orgURN, vdcURN).First(&vdc).Error
	if err != nil {
		return nil, err
	}
	return &vdc, nil
}

// HasDependentVApps checks if a VDC has dependent vApps
func (r *VDCRepository) HasDependentVApps(vdcID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.VApp{}).Where("vdc_id = ?", vdcID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteWithValidation deletes a VDC after checking for dependencies atomically
func (r *VDCRepository) DeleteWithValidation(id string) error {
	// Use a transaction to ensure atomicity
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check for dependent vApps within the transaction
		var count int64
		err := tx.Model(&models.VApp{}).Where("vdc_id = ?", id).Count(&count).Error
		if err != nil {
			return err
		}

		if count > 0 {
			return errors.New("cannot delete VDC with dependent vApps")
		}

		// Delete the VDC within the same transaction
		return tx.Where("id = ?", id).Delete(&models.VDC{}).Error
	})
}
