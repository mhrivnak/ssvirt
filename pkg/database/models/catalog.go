package models

import (
	"time"

	"gorm.io/gorm"
)

type Catalog struct {
	ID             string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name           string         `gorm:"not null" json:"name"`
	OrganizationID string         `gorm:"type:varchar(255);not null;index" json:"organization_id"`
	Description    string         `json:"description"`
	IsShared       bool           `gorm:"default:false" json:"is_shared"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Organization  *Organization  `gorm:"foreignKey:OrganizationID;references:ID" json:"organization,omitempty"`
	VAppTemplates []VAppTemplate `gorm:"foreignKey:CatalogID;references:ID" json:"vapp_templates,omitempty"`
}

func (c *Catalog) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = GenerateOrgURN() // Reuse org URN format for catalogs
	}
	return nil
}
