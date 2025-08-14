package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/pagination"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}
	return r.db.Create(user).Error
}

func (r *UserRepository) GetByID(id string) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}
	return r.db.Updates(user).Error
}

func (r *UserRepository) Delete(id string) error {
	result := r.db.Where("id = ?", id).Delete(&models.User{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepository) List(limit, offset int) ([]models.User, error) {
	// Sanitize and validate pagination parameters
	limit, offset = pagination.ClampPaginationParams(limit, offset)

	var users []models.User
	err := r.db.Limit(limit).Offset(offset).Order("username ASC").Find(&users).Error
	return users, err
}

func (r *UserRepository) GetWithRoles(id string) (*models.User, error) {
	var user models.User
	err := r.db.Preload("Roles").Preload("Organization").Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetWithEntityRefs gets a user and populates entity references for API responses
func (r *UserRepository) GetWithEntityRefs(id string) (*models.User, error) {
	user, err := r.GetWithRoles(id)
	if err != nil {
		return nil, err
	}

	// Populate role entity references from user's assigned roles
	user.RoleEntityRefs = make([]models.EntityRef, 0)
	for _, role := range user.Roles {
		user.RoleEntityRefs = append(user.RoleEntityRefs, models.EntityRef{
			Name: role.Name,
			ID:   role.ID,
		})
	}

	// Populate organization entity reference from user's primary organization
	if user.Organization != nil {
		user.OrgEntityRef = &models.EntityRef{
			Name: user.Organization.Name,
			ID:   user.Organization.ID,
		}
	}

	return user, nil
}

// ListWithEntityRefs gets users and populates entity references for API responses
func (r *UserRepository) ListWithEntityRefs(limit, offset int) ([]models.User, error) {
	// Sanitize and validate pagination parameters
	limit, offset = pagination.ClampPaginationParams(limit, offset)

	var users []models.User
	err := r.db.Preload("Roles").Preload("Organization").Limit(limit).Offset(offset).Order("username ASC").Find(&users).Error
	if err != nil {
		return nil, err
	}

	// Populate entity references for each user
	for i := range users {
		user := &users[i]

		// Populate role entity references from user's assigned roles
		user.RoleEntityRefs = make([]models.EntityRef, 0)
		for _, role := range user.Roles {
			user.RoleEntityRefs = append(user.RoleEntityRefs, models.EntityRef{
				Name: role.Name,
				ID:   role.ID,
			})
		}

		// Populate organization entity reference from user's primary organization
		if user.Organization != nil {
			user.OrgEntityRef = &models.EntityRef{
				Name: user.Organization.Name,
				ID:   user.Organization.ID,
			}
		}
	}

	return users, nil
}

// Count returns the total number of users
func (r *UserRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Count(&count).Error
	return count, err
}

// AssignRoles assigns roles to a user by role IDs
func (r *UserRepository) AssignRoles(userID string, roleIDs []string) error {
	if len(roleIDs) == 0 {
		return nil
	}

	user, err := r.GetByID(userID)
	if err != nil {
		return err
	}

	// Get the roles to assign
	var roles []models.Role
	err = r.db.Where("id IN ?", roleIDs).Find(&roles).Error
	if err != nil {
		return err
	}

	// Verify all requested roles were found
	if len(roles) != len(roleIDs) {
		return errors.New("one or more roles not found")
	}

	// Use association to assign roles (this replaces existing roles)
	return r.db.Model(user).Association("Roles").Replace(&roles)
}

// ClearRoles removes all role assignments from a user
func (r *UserRepository) ClearRoles(userID string) error {
	user, err := r.GetByID(userID)
	if err != nil {
		return err
	}

	return r.db.Model(user).Association("Roles").Clear()
}
