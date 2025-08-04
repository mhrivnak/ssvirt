package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Catalog struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name           string    `gorm:"not null" json:"name"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null" json:"organization_id"`
	Description    string    `json:"description"`
	IsShared       bool      `gorm:"default:false" json:"is_shared"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Organization   *Organization   `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	VAppTemplates  []VAppTemplate  `gorm:"foreignKey:CatalogID" json:"vapp_templates,omitempty"`
}

func (c *Catalog) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}