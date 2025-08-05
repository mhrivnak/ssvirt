package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VM struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	VAppID    uuid.UUID `gorm:"type:uuid;not null" json:"vapp_id"`
	VMName    string    `json:"vm_name"`    // OpenShift VM resource name
	Namespace string    `json:"namespace"`  // OpenShift namespace
	Status    string    `json:"status"`
	CPUCount  *int      `gorm:"check:cpu_count > 0" json:"cpu_count"`
	MemoryMB  *int      `gorm:"check:memory_mb > 0" json:"memory_mb"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	VApp *VApp `gorm:"foreignKey:VAppID" json:"vapp,omitempty"`
}

func (vm *VM) BeforeCreate(tx *gorm.DB) error {
	if vm.ID == uuid.Nil {
		vm.ID = uuid.New()
	}
	return nil
}