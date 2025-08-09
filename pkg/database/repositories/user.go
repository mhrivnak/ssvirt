package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
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
	if limit <= 0 {
		limit = 25 // Default limit to ensure results are returned
	}
	if offset < 0 {
		offset = 0
	}
	if limit > 1000 { // reasonable maximum
		limit = 1000
	}
	var users []models.User
	err := r.db.Limit(limit).Offset(offset).Order("username ASC").Find(&users).Error
	return users, err
}

func (r *UserRepository) GetWithRoles(id string) (*models.User, error) {
	var user models.User
	err := r.db.Preload("UserRoles").Preload("UserRoles.Role").Preload("UserRoles.Organization").Preload("Organization").Where("id = ?", id).First(&user).Error
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

	// Populate role entity references from all user roles
	user.RoleEntityRefs = make([]models.EntityRef, 0)
	for _, userRole := range user.UserRoles {
		if userRole.Role != nil {
			user.RoleEntityRefs = append(user.RoleEntityRefs, models.EntityRef{
				Name: userRole.Role.Name,
				ID:   userRole.Role.ID,
			})
		}
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
	if limit <= 0 {
		limit = 25 // Default limit to ensure results are returned
	}
	if offset < 0 {
		offset = 0
	}
	if limit > 1000 { // reasonable maximum
		limit = 1000
	}

	var users []models.User
	err := r.db.Preload("UserRoles").Preload("UserRoles.Role").Preload("UserRoles.Organization").Preload("Organization").Limit(limit).Offset(offset).Order("username ASC").Find(&users).Error
	if err != nil {
		return nil, err
	}

	// Populate entity references for each user
	for i := range users {
		user := &users[i]

		// Populate role entity references from all user roles
		user.RoleEntityRefs = make([]models.EntityRef, 0)
		for _, userRole := range user.UserRoles {
			if userRole.Role != nil {
				user.RoleEntityRefs = append(user.RoleEntityRefs, models.EntityRef{
					Name: userRole.Role.Name,
					ID:   userRole.Role.ID,
				})
			}
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
