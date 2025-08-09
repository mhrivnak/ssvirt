package models

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type VDC struct {
	ID              string          `gorm:"type:varchar(255);primary_key" json:"id"`
	Name            string          `gorm:"not null" json:"name"`
	OrganizationID  string          `gorm:"type:varchar(255);not null;index" json:"organization_id"`
	AllocationModel AllocationModel `gorm:"type:varchar(20);check:allocation_model IN ('PayAsYouGo', 'AllocationPool', 'ReservationPool')" json:"allocation_model"`
	CPULimit        *int            `gorm:"check:cpu_limit > 0" json:"cpu_limit"`
	MemoryLimitMB   *int            `gorm:"check:memory_limit_mb > 0" json:"memory_limit_mb"`
	StorageLimitMB  *int            `gorm:"check:storage_limit_mb > 0" json:"storage_limit_mb"`
	Namespace       string          `gorm:"uniqueIndex;size:253" json:"namespace"` // Kubernetes namespace for this VDC
	Enabled         bool            `gorm:"default:true" json:"enabled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	DeletedAt       gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID;references:ID" json:"organization,omitempty"`
	VApps        []VApp        `gorm:"foreignKey:VDCID;references:ID" json:"vapps,omitempty"`
}

func (v *VDC) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = GenerateOrgURN() // Reuse org URN format for VDCs
	}

	// Generate namespace name if not set
	if v.Namespace == "" {
		// Load organization to get name
		var org Organization
		if err := tx.Where("id = ?", v.OrganizationID).First(&org).Error; err != nil {
			return fmt.Errorf("failed to load organization: %w", err)
		}
		v.Namespace = generateNamespaceName(org.Name, v.Name)
	}

	return nil
}

// generateNamespaceName creates a Kubernetes-compliant namespace name
func generateNamespaceName(orgName, vdcName string) string {
	// Convert to lowercase and replace invalid characters
	orgSafe := strings.ToLower(strings.ReplaceAll(orgName, "_", "-"))
	vdcSafe := strings.ToLower(strings.ReplaceAll(vdcName, "_", "-"))

	// Remove any characters that aren't alphanumeric or hyphen
	orgSafe = sanitizeKubernetesName(orgSafe)
	vdcSafe = sanitizeKubernetesName(vdcSafe)

	return fmt.Sprintf("vdc-%s-%s", orgSafe, vdcSafe)
}

// sanitizeKubernetesName ensures the name is valid for Kubernetes
func sanitizeKubernetesName(name string) string {
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result += string(r)
		}
	}
	// Ensure it doesn't start or end with hyphen
	result = strings.Trim(result, "-")
	if result == "" {
		result = "default"
	}
	return result
}
