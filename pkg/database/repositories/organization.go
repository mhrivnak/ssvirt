package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type OrganizationRepository struct {
	db *gorm.DB
}

func NewOrganizationRepository(db *gorm.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Create(org *models.Organization) error {
	if org == nil {
		return errors.New("organization cannot be nil")
	}
	return r.db.Create(org).Error
}

func (r *OrganizationRepository) GetByID(id string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Where("id = ?", id).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// GetByIDWithContext retrieves organization by ID with context support
func (r *OrganizationRepository) GetByIDWithContext(ctx context.Context, id string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil org for not found
		}
		return nil, err
	}
	return &org, nil
}

func (r *OrganizationRepository) GetByName(name string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Where("name = ?", name).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *OrganizationRepository) List() ([]models.Organization, error) {
	var orgs []models.Organization
	err := r.db.Find(&orgs).Error
	return orgs, err
}

func (r *OrganizationRepository) Update(org *models.Organization) error {
	if org == nil {
		return errors.New("organization cannot be nil")
	}
	return r.db.Save(org).Error
}

func (r *OrganizationRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.Organization{}).Error
}

func (r *OrganizationRepository) GetWithVDCs(id string) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Preload("VDCs").Where("id = ?", id).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *OrganizationRepository) GetAll(ctx context.Context) ([]models.Organization, error) {
	var orgs []models.Organization
	err := r.db.WithContext(ctx).Find(&orgs).Error
	return orgs, err
}

// GetWithEntityRefs gets organizations and populates entity references for API responses
func (r *OrganizationRepository) GetWithEntityRefs(id string) (*models.Organization, error) {
	org, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Populate computed count fields - for now set to 0, can be enhanced later
	org.OrgVdcCount = 0
	org.CatalogCount = 0
	org.VappCount = 0
	org.RunningVMCount = 0
	org.UserCount = 0
	org.DiskCount = 0
	org.DirectlyManagedOrgCount = 0

	// Set managedBy to nil for now - can be enhanced later
	org.ManagedBy = nil

	return org, nil
}

// ListWithEntityRefs gets organizations and populates entity references for API responses
func (r *OrganizationRepository) ListWithEntityRefs(limit, offset int) ([]models.Organization, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	if limit > 1000 { // reasonable maximum
		limit = 1000
	}

	var orgs []models.Organization
	err := r.db.Limit(limit).Offset(offset).Find(&orgs).Error
	if err != nil {
		return nil, err
	}

	// Populate computed fields for each organization
	for i := range orgs {
		org := &orgs[i]

		// Populate computed count fields - for now set to 0, can be enhanced later
		org.OrgVdcCount = 0
		org.CatalogCount = 0
		org.VappCount = 0
		org.RunningVMCount = 0
		org.UserCount = 0
		org.DiskCount = 0
		org.DirectlyManagedOrgCount = 0

		// Set managedBy to nil for now - can be enhanced later
		org.ManagedBy = nil
	}

	return orgs, nil
}

// CreateDefaultOrganization creates the default Provider organization
func (r *OrganizationRepository) CreateDefaultOrganization() (*models.Organization, error) {
	// Check if Provider organization already exists
	existing, err := r.GetByName(models.DefaultOrgName)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil // Already exists
	}

	// Create the Provider organization
	org := &models.Organization{
		Name:          models.DefaultOrgName,
		DisplayName:   "Provider Organization",
		Description:   "Default provider organization",
		IsEnabled:     true,
		CanManageOrgs: true,
		CanPublish:    false,
	}

	if err := r.Create(org); err != nil {
		return nil, err
	}

	return org, nil
}
