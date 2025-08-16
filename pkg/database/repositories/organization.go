package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/pagination"
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
	// Sanitize and validate pagination parameters
	limit, offset = pagination.ClampPaginationParams(limit, offset)

	var orgs []models.Organization
	err := r.db.Limit(limit).Offset(offset).Order("name ASC").Find(&orgs).Error
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

// Count returns the total number of organizations
func (r *OrganizationRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Organization{}).Count(&count).Error
	return count, err
}

// Public API methods for user access control

// ListAccessibleOrgs retrieves organizations accessible to a user based on their role and organization membership with pagination
func (r *OrganizationRepository) ListAccessibleOrgs(ctx context.Context, userID string, limit, offset int) ([]models.Organization, error) {
	var orgs []models.Organization

	// Check if user is a system administrator - they have access to all organizations
	var isSystemAdmin bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS(
			SELECT 1 FROM users u
			JOIN user_roles ur ON u.id = ur.user_id
			JOIN roles r ON ur.role_id = r.id
			WHERE u.id = ? AND r.name = ? AND u.deleted_at IS NULL AND r.deleted_at IS NULL
		)`, userID, models.RoleSystemAdmin).Scan(&isSystemAdmin).Error
	if err != nil {
		return nil, err
	}

	if isSystemAdmin {
		// System administrators can access all organizations
		err := r.db.WithContext(ctx).
			Limit(limit).
			Offset(offset).
			Order("name ASC").
			Find(&orgs).Error
		if err != nil {
			return nil, err
		}
	} else {
		// For non-system administrators, return only their primary organization
		subquery := r.db.WithContext(ctx).Model(&models.User{}).Select("organization_id").Where("id = ? AND organization_id IS NOT NULL", userID)

		err = r.db.WithContext(ctx).Where("id IN (?)", subquery).
			Limit(limit).
			Offset(offset).
			Order("name ASC").
			Find(&orgs).Error
		if err != nil {
			return nil, err
		}
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

// CountAccessibleOrgs returns the total count of organizations accessible to a user
func (r *OrganizationRepository) CountAccessibleOrgs(ctx context.Context, userID string) (int64, error) {
	var count int64

	// Check if user is a system administrator - they have access to all organizations
	var isSystemAdmin bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS(
			SELECT 1 FROM users u
			JOIN user_roles ur ON u.id = ur.user_id
			JOIN roles r ON ur.role_id = r.id
			WHERE u.id = ? AND r.name = ? AND u.deleted_at IS NULL AND r.deleted_at IS NULL
		)`, userID, models.RoleSystemAdmin).Scan(&isSystemAdmin).Error
	if err != nil {
		return 0, err
	}

	if isSystemAdmin {
		// System administrators can access all organizations
		err := r.db.WithContext(ctx).Model(&models.Organization{}).Count(&count).Error
		return count, err
	} else {
		// For non-system administrators, count only their primary organization
		subquery := r.db.WithContext(ctx).Model(&models.User{}).Select("organization_id").Where("id = ? AND organization_id IS NOT NULL", userID)

		err = r.db.WithContext(ctx).Model(&models.Organization{}).Where("id IN (?)", subquery).Count(&count).Error
		return count, err
	}
}

// GetAccessibleOrg retrieves a specific organization if the user has access to it
func (r *OrganizationRepository) GetAccessibleOrg(ctx context.Context, userID, orgID string) (*models.Organization, error) {
	var org models.Organization

	// Check if user is a system administrator - they have access to all organizations
	var isSystemAdmin bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS(
			SELECT 1 FROM users u
			JOIN user_roles ur ON u.id = ur.user_id
			JOIN roles r ON ur.role_id = r.id
			WHERE u.id = ? AND r.name = ? AND u.deleted_at IS NULL AND r.deleted_at IS NULL
		)`, userID, models.RoleSystemAdmin).Scan(&isSystemAdmin).Error
	if err != nil {
		return nil, err
	}

	if isSystemAdmin {
		// System administrators can access any organization
		org, err := r.GetWithEntityRefs(orgID)
		return org, err
	}

	// For non-system administrators, check if the requested org is their primary organization
	subquery := r.db.WithContext(ctx).Model(&models.User{}).Select("organization_id").Where("id = ? AND organization_id IS NOT NULL", userID)

	err = r.db.WithContext(ctx).Where("id = ? AND id IN (?)", orgID, subquery).First(&org).Error
	if err != nil {
		return nil, err
	}

	// Check if organization was found
	if org.ID == "" {
		return nil, gorm.ErrRecordNotFound
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

	return &org, nil
}
