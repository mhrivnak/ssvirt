package models

import (
	"time"

	"gorm.io/gorm"
)

// Role represents a role in the system following VMware Cloud Director API spec
type Role struct {
	ID          string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name        string         `gorm:"unique;not null;size:255" json:"name"`
	Description string         `json:"description"`
	BundleKey   string         `json:"bundleKey"`
	ReadOnly    bool           `gorm:"default:true;not null" json:"readOnly"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BeforeCreate sets the URN ID if not already set
func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = GenerateRoleURN()
	}
	return nil
}

// IsSystemAdmin checks if this role is the System Administrator role
func (r *Role) IsSystemAdmin() bool {
	return r.Name == RoleSystemAdmin
}

// IsOrgAdmin checks if this role is the Organization Administrator role
func (r *Role) IsOrgAdmin() bool {
	return r.Name == RoleOrgAdmin
}

// IsVAppUser checks if this role is the vApp User role
func (r *Role) IsVAppUser() bool {
	return r.Name == RoleVAppUser
}
