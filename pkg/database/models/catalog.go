package models

import (
	"time"

	"gorm.io/gorm"
)

// Catalog represents a Virtual Data Center catalog in VMware Cloud Director format
type Catalog struct {
	// Core catalog fields
	ID             string `gorm:"type:varchar(255);primaryKey" json:"id"`
	Name           string `gorm:"not null" json:"name"`
	Description    string `json:"description"`
	OrganizationID string `gorm:"type:varchar(255);not null;index" json:"-"` // Hidden from JSON

	// VCD-specific fields
	IsPublished  bool   `gorm:"default:false" json:"isPublished"`
	IsSubscribed bool   `gorm:"default:false" json:"isSubscribed"`
	IsLocal      bool   `gorm:"default:true" json:"isLocal"`
	Version      int    `gorm:"default:1" json:"version"`
	OwnerID      string `gorm:"type:varchar(255)" json:"-"` // Hidden, part of owner object

	// Timestamps (hidden from JSON in VCD format)
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships (hidden from JSON)
	Organization  *Organization  `gorm:"foreignKey:OrganizationID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	VAppTemplates []VAppTemplate `gorm:"foreignKey:CatalogID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// OrgReference represents an organization reference for VCD compliance
type OrgReference struct {
	ID string `json:"id"`
}

// OwnerReference represents an owner reference for VCD compliance
type OwnerReference struct {
	ID string `json:"id"`
}

// PublishConfig represents the publish configuration
type PublishConfig struct {
	IsPublished bool `json:"isPublished"`
}

// SubscriptionConfig represents the subscription configuration
type SubscriptionConfig struct {
	IsSubscribed bool `json:"isSubscribed"`
}

// Org returns the VCD-compliant organization reference
func (c *Catalog) Org() OrgReference {
	return OrgReference{
		ID: c.OrganizationID,
	}
}

// Owner returns the VCD-compliant owner reference
func (c *Catalog) Owner() OwnerReference {
	return OwnerReference{
		ID: c.OwnerID,
	}
}

// PublishConfig returns the publish configuration
func (c *Catalog) PublishConfigObj() PublishConfig {
	return PublishConfig{
		IsPublished: c.IsPublished,
	}
}

// SubscriptionConfig returns the subscription configuration
func (c *Catalog) SubscriptionConfigObj() SubscriptionConfig {
	return SubscriptionConfig{
		IsSubscribed: c.IsSubscribed,
	}
}

// DistributedCatalogConfig returns empty object as specified
func (c *Catalog) DistributedCatalogConfig() interface{} {
	return map[string]interface{}{}
}

// CatalogStorageProfiles returns empty array as specified
func (c *Catalog) CatalogStorageProfiles() []interface{} {
	return []interface{}{}
}

// NumberOfVAppTemplates returns the count of vApp templates (computed)
func (c *Catalog) NumberOfVAppTemplates() int {
	return len(c.VAppTemplates)
}

// NumberOfMedia returns the count of media items (default 0 as no media support yet)
func (c *Catalog) NumberOfMedia() int {
	return 0
}

// CreationDate returns the creation date in ISO-8601 format
func (c *Catalog) CreationDate() string {
	return c.CreatedAt.Format(time.RFC3339)
}

// BeforeCreate sets up the catalog before database creation
func (c *Catalog) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = GenerateCatalogURN()
	}

	// Set default owner to empty if not provided
	if c.OwnerID == "" {
		c.OwnerID = ""
	}

	return nil
}
