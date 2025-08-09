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
	return r.db.Updates(role).Error
}

func (r *RoleRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.Role{}).Error
}

func (r *RoleRepository) List(limit, offset int) ([]models.Role, error) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	if limit > 1000 { // reasonable maximum
		limit = 1000
	}
	var roles []models.Role
	err := r.db.Limit(limit).Offset(offset).Find(&roles).Error
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

	for _, role := range roles {
		// Check if role already exists
		existing, err := r.GetByName(role.Name)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existing != nil {
			continue // Role already exists
		}

		// Create the role
		if err := r.Create(&role); err != nil {
			return err
		}
	}

	return nil
}
