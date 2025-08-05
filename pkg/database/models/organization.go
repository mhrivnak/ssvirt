package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Organization struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null;size:255" json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	Namespace   string    `gorm:"uniqueIndex;size:63" json:"namespace"` // Kubernetes namespace max length
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	VDCs    []VDC    `gorm:"foreignKey:OrganizationID" json:"vdcs,omitempty"`
	Catalogs []Catalog `gorm:"foreignKey:OrganizationID" json:"catalogs,omitempty"`
	UserRoles []UserRole `gorm:"foreignKey:OrganizationID" json:"user_roles,omitempty"`
}

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}