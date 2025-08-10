package models

import (
	"time"

	"gorm.io/gorm"
)

type VApp struct {
	ID          string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	VDCID       string         `gorm:"type:varchar(255);not null;index" json:"vdc_id"`
	TemplateID  *string        `gorm:"type:varchar(255);index" json:"template_id"`
	Status      string         `json:"status"` // RESOLVED, DEPLOYED, SUSPENDED, etc.
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	VDC      *VDC          `gorm:"foreignKey:VDCID;references:ID" json:"vdc,omitempty"`
	Template *VAppTemplate `gorm:"foreignKey:TemplateID;references:ID" json:"template,omitempty"`
	VMs      []VM          `gorm:"foreignKey:VAppID;references:ID" json:"vms,omitempty"`
}

func (va *VApp) BeforeCreate(tx *gorm.DB) error {
	if va.ID == "" {
		va.ID = GenerateVAppURN()
	}
	return nil
}
