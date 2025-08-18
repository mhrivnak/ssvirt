package models

import (
	"time"

	"gorm.io/gorm"
)

// VApp status constants
const (
	VAppStatusInstantiating = "INSTANTIATING"
	VAppStatusDeployed      = "DEPLOYED"
	VAppStatusFailed        = "FAILED"
	VAppStatusDeleting      = "DELETING"
	VAppStatusDeleted       = "DELETED"
	VAppStatusPoweringOn    = "POWERING_ON"
	VAppStatusPoweringOff   = "POWERING_OFF"
)

// ValidVAppStatuses contains all valid vApp status values
var ValidVAppStatuses = []string{
	VAppStatusInstantiating,
	VAppStatusDeployed,
	VAppStatusFailed,
	VAppStatusDeleting,
	VAppStatusDeleted,
	VAppStatusPoweringOn,
	VAppStatusPoweringOff,
}

// IsValidVAppStatus checks if a status is valid
func IsValidVAppStatus(status string) bool {
	for _, validStatus := range ValidVAppStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

type VApp struct {
	ID          string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name        string         `gorm:"not null;uniqueIndex:idx_vapp_vdc_name" json:"name"`
	VDCID       string         `gorm:"type:varchar(255);not null;index;uniqueIndex:idx_vapp_vdc_name" json:"vdc_id"`
	TemplateID  *string        `gorm:"type:varchar(255);index" json:"template_id"`
	Status      string         `json:"status"` // INSTANTIATING, DEPLOYED, FAILED, DELETING, DELETED, etc.
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
