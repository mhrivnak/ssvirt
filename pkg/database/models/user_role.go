package models

import (
	"time"

	"gorm.io/gorm"
)

// UserRole represents the many-to-many relationship between users and roles
type UserRole struct {
	ID             string         `gorm:"type:varchar(255);primaryKey" json:"id"`
	UserID         string         `gorm:"type:varchar(255);not null;index;uniqueIndex:idx_user_org_role" json:"user_id"`
	OrganizationID string         `gorm:"type:varchar(255);not null;index;uniqueIndex:idx_user_org_role" json:"organization_id"`
	RoleID         string         `gorm:"type:varchar(255);not null;index;uniqueIndex:idx_user_org_role" json:"role_id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships with cascading deletes
	User         *User         `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Organization *Organization `gorm:"foreignKey:OrganizationID;references:ID;constraint:OnDelete:CASCADE" json:"organization,omitempty"`
	Role         *Role         `gorm:"foreignKey:RoleID;references:ID;constraint:OnDelete:CASCADE" json:"role,omitempty"`
}

func (ur *UserRole) BeforeCreate(tx *gorm.DB) error {
	if ur.ID == "" {
		ur.ID = GenerateUserURN() // Reuse user URN format for simplicity
	}
	return nil
}
