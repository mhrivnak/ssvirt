package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(role *models.Role) error {
	if role == nil {
		return errors.New("role cannot be nil")
	}
	return r.db.Create(role).Error
}

func (r *RoleRepository) GetByID(id string) (*models.Role, error) {
	var role models.Role
	err := r.db.Where("id = ?", id).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) GetByName(name string) (*models.Role, error) {
	var role models.Role
	err := r.db.Where("name = ?", name).First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) Update(role *models.Role) error {
	if role == nil {
		return errors.New("role cannot be nil")
	}
	if role.ID == "" {
		return errors.New("role ID cannot be empty")
	}
	return r.db.Save(role).Error
}

func (r *RoleRepository) Delete(id string) error {
	// First check if the role exists and is not a system role
	var role models.Role
	err := r.db.Where("id = ?", id).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		return err
	}

	// Prevent deletion of read-only system roles
	if role.ReadOnly {
		return errors.New("cannot delete read-only system role")
	}

	// Attempt deletion and check if any rows were affected
	result := r.db.Where("id = ?", id).Delete(&models.Role{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *RoleRepository) List(limit, offset int) ([]models.Role, error) {
	if limit <= 0 {
		limit = 25 // Default limit to ensure results are returned
	}
	if offset < 0 {
		offset = 0
	}
	if limit > 1000 { // reasonable maximum
		limit = 1000
	}
	var roles []models.Role
	err := r.db.Limit(limit).Offset(offset).Order("name ASC").Find(&roles).Error
	return roles, err
}

// GetSystemAdminRole gets the System Administrator role
func (r *RoleRepository) GetSystemAdminRole() (*models.Role, error) {
	return r.GetByName(models.RoleSystemAdmin)
}

// GetOrgAdminRole gets the Organization Administrator role
func (r *RoleRepository) GetOrgAdminRole() (*models.Role, error) {
	return r.GetByName(models.RoleOrgAdmin)
}

// GetVAppUserRole gets the vApp User role
func (r *RoleRepository) GetVAppUserRole() (*models.Role, error) {
	return r.GetByName(models.RoleVAppUser)
}

// CreateDefaultRoles creates the default system roles
func (r *RoleRepository) CreateDefaultRoles() error {
	roles := []models.Role{
		{
			Name:        models.RoleSystemAdmin,
			Description: "Full system administrator access",
			BundleKey:   "",
			ReadOnly:    true,
		},
		{
			Name:        models.RoleOrgAdmin,
			Description: "Organization administrator access",
			BundleKey:   "",
			ReadOnly:    true,
		},
		{
			Name:        models.RoleVAppUser,
			Description: "Basic vApp user access",
			BundleKey:   "",
			ReadOnly:    true,
		},
	}

	// Use transaction to ensure atomicity
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, role := range roles {
			// Check if role already exists using transaction
			var existing models.Role
			err := tx.Where("name = ?", role.Name).First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if existing.ID != "" {
				continue // Role already exists
			}

			// Generate URN for the role
			role.ID = models.GenerateRoleURN()

			// Create the role using transaction
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Count returns the total number of roles
func (r *RoleRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Role{}).Count(&count).Error
	return count, err
}
