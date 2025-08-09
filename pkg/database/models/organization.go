package models

import (
	"time"

	"gorm.io/gorm"
)

type Organization struct {
	ID                        string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name                      string         `gorm:"uniqueIndex;not null;size:255" json:"name"`
	DisplayName               string         `gorm:"size:255" json:"displayName"`
	Description               string         `json:"description"`
	IsEnabled                 bool           `gorm:"default:true;not null" json:"isEnabled"`
	OrgVdcCount              int            `gorm:"-" json:"orgVdcCount"`              // Computed field
	CatalogCount             int            `gorm:"-" json:"catalogCount"`             // Computed field
	VappCount                int            `gorm:"-" json:"vappCount"`                // Computed field
	RunningVMCount           int            `gorm:"-" json:"runningVMCount"`           // Computed field
	UserCount                int            `gorm:"-" json:"userCount"`                // Computed field
	DiskCount                int            `gorm:"-" json:"diskCount"`                // Computed field
	CanManageOrgs            bool           `gorm:"default:false;not null" json:"canManageOrgs"`
	CanPublish               bool           `gorm:"default:false;not null" json:"canPublish"`
	MaskedEventTaskUsername  string         `json:"maskedEventTaskUsername"`
	DirectlyManagedOrgCount  int            `gorm:"-" json:"directlyManagedOrgCount"`  // Computed field
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
	DeletedAt                gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Entity references (populated in API responses)
	ManagedBy *EntityRef `gorm:"-" json:"managedBy,omitempty"`

	// Relationships
	VDCs      []VDC      `gorm:"foreignKey:OrganizationID;references:ID" json:"vdcs,omitempty"`
	Catalogs  []Catalog  `gorm:"foreignKey:OrganizationID;references:ID" json:"catalogs,omitempty"`
	UserRoles []UserRole `gorm:"foreignKey:OrganizationID;references:ID" json:"user_roles,omitempty"`
}

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = GenerateOrgURN()
	}
	if o.DisplayName == "" {
		o.DisplayName = o.Name
	}
	return nil
}

// IsProvider checks if this is the default Provider organization
func (o *Organization) IsProvider() bool {
	return o.Name == DefaultOrgName
}
