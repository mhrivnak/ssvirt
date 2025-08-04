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
	AllocationModel string    `json:"allocation_model"` // PayAsYouGo, AllocationPool, ReservationPool
	CPULimit        *int      `json:"cpu_limit"`
	MemoryLimitMB   *int      `json:"memory_limit_mb"`
	StorageLimitMB  *int      `json:"storage_limit_mb"`
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