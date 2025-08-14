package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/pagination"
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
	// Sanitize and validate pagination parameters
	limit, offset = pagination.ClampPaginationParams(limit, offset)

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
			// Rebind role variable to avoid range variable address issue
			currentRole := role

			// Check if role already exists using transaction
			var existing models.Role
			err := tx.Where("name = ?", currentRole.Name).First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if existing.ID != "" {
				continue // Role already exists
			}

			// Generate URN for the role
			currentRole.ID = models.GenerateRoleURN()

			// Create the role using transaction
			if err := tx.Create(&currentRole).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ExistsByIDs checks which of the provided role IDs exist in the database
// Returns a map where the key is the role ID and the value indicates if it exists
func (r *RoleRepository) ExistsByIDs(roleIDs []string) (map[string]bool, error) {
	if len(roleIDs) == 0 {
		return make(map[string]bool), nil
	}

	var roles []models.Role
	err := r.db.Select("id").Where("id IN ?", roleIDs).Find(&roles).Error
	if err != nil {
		return nil, err
	}

	// Build result map
	result := make(map[string]bool)
	for _, id := range roleIDs {
		result[id] = false
	}
	for _, role := range roles {
		result[role.ID] = true
	}

	return result, nil
}

// Count returns the total number of roles
func (r *RoleRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Role{}).Count(&count).Error
	return count, err
}
