package models

import (
	"time"

	"gorm.io/gorm"
)

type VM struct {
	ID        string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name      string         `gorm:"not null" json:"name"`
	VAppID    string         `gorm:"type:varchar(255);not null;index" json:"vapp_id"`
	VMName    string         `json:"vm_name"`   // OpenShift VM resource name
	Namespace string         `json:"namespace"` // OpenShift namespace
	Status    string         `json:"status"`
	CPUCount  *int           `gorm:"check:cpu_count > 0" json:"cpu_count"`
	MemoryMB  *int           `gorm:"check:memory_mb > 0" json:"memory_mb"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	VApp *VApp `gorm:"foreignKey:VAppID;references:ID" json:"vapp,omitempty"`
}

func (vm *VM) BeforeCreate(tx *gorm.DB) error {
	if vm.ID == "" {
		vm.ID = GenerateOrgURN() // Reuse org URN format for VMs
	}
	return nil
}
