package models

import (
	"time"

	"gorm.io/gorm"
)

type VAppTemplate struct {
	ID             string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Name           string         `gorm:"not null" json:"name"`
	CatalogID      string         `gorm:"type:varchar(255);not null;index" json:"catalog_id"`
	Description    string         `json:"description"`
	VMInstanceType string         `json:"vm_instance_type"` // OpenShift VirtualMachineInstanceType
	OSType         string         `json:"os_type"`
	CPUCount       *int           `gorm:"check:cpu_count > 0" json:"cpu_count"`
	MemoryMB       *int           `gorm:"check:memory_mb > 0" json:"memory_mb"`
	DiskSizeGB     *int           `gorm:"check:disk_size_gb > 0" json:"disk_size_gb"`
	TemplateData   string         `gorm:"type:jsonb" json:"template_data"` // Template configuration as JSON
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Catalog *Catalog `gorm:"foreignKey:CatalogID;references:ID" json:"catalog,omitempty"`
	VApps   []VApp   `gorm:"foreignKey:TemplateID;references:ID" json:"vapps,omitempty"`
}

func (vt *VAppTemplate) BeforeCreate(tx *gorm.DB) error {
	if vt.ID == "" {
		vt.ID = GenerateOrgURN() // Reuse org URN format for templates
	}
	return nil
}
