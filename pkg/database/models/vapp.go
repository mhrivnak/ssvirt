package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VApp struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Name        string     `gorm:"not null" json:"name"`
	VDCID       uuid.UUID  `gorm:"type:uuid;not null" json:"vdc_id"`
	TemplateID  *uuid.UUID `gorm:"type:uuid" json:"template_id"`
	Status      string     `json:"status"` // RESOLVED, DEPLOYED, SUSPENDED, etc.
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	VDC      *VDC         `gorm:"foreignKey:VDCID" json:"vdc,omitempty"`
	Template *VAppTemplate `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	VMs      []VM         `gorm:"foreignKey:VAppID" json:"vms,omitempty"`
}

func (va *VApp) BeforeCreate(tx *gorm.DB) error {
	if va.ID == uuid.Nil {
		va.ID = uuid.New()
	}
	return nil
}