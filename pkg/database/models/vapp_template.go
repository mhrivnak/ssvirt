package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VAppTemplate struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name           string    `gorm:"not null" json:"name"`
	CatalogID      uuid.UUID `gorm:"type:uuid;not null" json:"catalog_id"`
	Description    string    `json:"description"`
	VMInstanceType string    `json:"vm_instance_type"` // OpenShift VirtualMachineInstanceType
	OSType         string    `json:"os_type"`
	CPUCount       *int      `json:"cpu_count"`
	MemoryMB       *int      `json:"memory_mb"`
	DiskSizeGB     *int      `json:"disk_size_gb"`
	TemplateData   string    `gorm:"type:jsonb" json:"template_data"` // Template configuration as JSON
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Catalog *Catalog `gorm:"foreignKey:CatalogID" json:"catalog,omitempty"`
	VApps   []VApp   `gorm:"foreignKey:TemplateID" json:"vapps,omitempty"`
}

func (vt *VAppTemplate) BeforeCreate(tx *gorm.DB) error {
	if vt.ID == uuid.Nil {
		vt.ID = uuid.New()
	}
	return nil
}