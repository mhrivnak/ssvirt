package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type UserRoleRepository struct {
	db *gorm.DB
}

func NewUserRoleRepository(db *gorm.DB) *UserRoleRepository {
	return &UserRoleRepository{db: db}
}

func (r *UserRoleRepository) Create(userRole *models.UserRole) error {
	return r.db.Create(userRole).Error
}

func (r *UserRoleRepository) GetByID(id uuid.UUID) (*models.UserRole, error) {
	var userRole models.UserRole
	err := r.db.Where("id = ?", id).First(&userRole).Error
	if err != nil {
		return nil, err
	}
	return &userRole, nil
}

func (r *UserRoleRepository) GetByUserID(userID string) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.Where("user_id = ?", userID).Find(&userRoles).Error
	return userRoles, err
}

func (r *UserRoleRepository) GetByOrganizationID(orgID uuid.UUID) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.Where("organization_id = ?", orgID).Find(&userRoles).Error
	return userRoles, err
}

func (r *UserRoleRepository) GetByUserAndOrganization(userID string, orgID uuid.UUID) ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.Where("user_id = ? AND organization_id = ?", userID, orgID).Find(&userRoles).Error
	return userRoles, err
}

func (r *UserRoleRepository) List() ([]models.UserRole, error) {
	var userRoles []models.UserRole
	err := r.db.Find(&userRoles).Error
	return userRoles, err
}

func (r *UserRoleRepository) Update(userRole *models.UserRole) error {
	return r.db.Save(userRole).Error
}

func (r *UserRoleRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.UserRole{}, id).Error
}

func (r *UserRoleRepository) DeleteByUserAndOrganization(userID string, orgID uuid.UUID) error {
	return r.db.Where("user_id = ? AND organization_id = ?", userID, orgID).Delete(&models.UserRole{}).Error
}