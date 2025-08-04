package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VDC struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name            string    `gorm:"not null" json:"name"`
	OrganizationID  uuid.UUID `gorm:"type:uuid;not null" json:"organization_id"`
	AllocationModel AllocationModel `gorm:"type:varchar(20);check:allocation_model IN ('PayAsYouGo', 'AllocationPool', 'ReservationPool')" json:"allocation_model"`
	CPULimit        *int      `gorm:"check:cpu_limit > 0" json:"cpu_limit"`
	MemoryLimitMB   *int      `gorm:"check:memory_limit_mb > 0" json:"memory_limit_mb"`
	StorageLimitMB  *int      `gorm:"check:storage_limit_mb > 0" json:"storage_limit_mb"`
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	VApps        []VApp        `gorm:"foreignKey:VDCID" json:"vapps,omitempty"`
}

func (v *VDC) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}