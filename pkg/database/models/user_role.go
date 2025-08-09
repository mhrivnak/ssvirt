package models

import (
	"time"

	"gorm.io/gorm"
)

type UserRole struct {
	ID             string         `gorm:"type:varchar(255);primary_key" json:"id"`
	UserID         string         `gorm:"type:varchar(255);not null;index" json:"user_id"`
	OrganizationID string         `gorm:"type:varchar(255);not null;index" json:"organization_id"`
	RoleID         string         `gorm:"type:varchar(255);not null;index" json:"role_id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	User         *User         `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	Organization *Organization `gorm:"foreignKey:OrganizationID;references:ID" json:"organization,omitempty"`
	Role         *Role         `gorm:"foreignKey:RoleID;references:ID" json:"role,omitempty"`
}

func (ur *UserRole) BeforeCreate(tx *gorm.DB) error {
	if ur.ID == "" {
		ur.ID = GenerateUserURN() // Reuse user URN format for simplicity
	}
	return nil
}
