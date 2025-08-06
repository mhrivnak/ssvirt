package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
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

func (r *OrganizationRepository) GetByID(id uuid.UUID) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Where("id = ?", id).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// GetByIDWithContext retrieves organization by ID with context support
func (r *OrganizationRepository) GetByIDWithContext(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
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

func (r *OrganizationRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Organization{}, id).Error
}

func (r *OrganizationRepository) GetWithVDCs(id uuid.UUID) (*models.Organization, error) {
	var org models.Organization
	err := r.db.Preload("VDCs").Where("id = ?", id).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}


func (r *OrganizationRepository) GetByIDString(ctx context.Context, idStr string) (*models.Organization, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}

	var org models.Organization
	err = r.db.WithContext(ctx).Where("id = ?", id).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil org for not found
		}
		return nil, err
	}
	return &org, nil
}

func (r *OrganizationRepository) GetAll(ctx context.Context) ([]models.Organization, error) {
	var orgs []models.Organization
	err := r.db.WithContext(ctx).Find(&orgs).Error
	return orgs, err
}
