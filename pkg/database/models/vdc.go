package models

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// VDC represents a Virtual Data Center in VMware Cloud Director format
type VDC struct {
	// Core VDC fields
	ID              string          `gorm:"type:varchar(255);primaryKey" json:"id"`
	Name            string          `gorm:"not null" json:"name"`
	Description     string          `json:"description"`
	OrganizationID  string          `gorm:"type:varchar(255);not null;index" json:"-"` // Hidden from JSON
	AllocationModel AllocationModel `gorm:"type:varchar(20);check:allocation_model IN ('PayAsYouGo', 'AllocationPool', 'ReservationPool', 'Flex')" json:"allocationModel"`

	// Compute capacity fields
	CPUAllocated    int    `gorm:"default:0" json:"-"`     // Hidden, part of computeCapacity
	CPULimit        int    `gorm:"default:0" json:"-"`     // Hidden, part of computeCapacity
	CPUUnits        string `gorm:"default:'MHz'" json:"-"` // Hidden, part of computeCapacity
	MemoryAllocated int    `gorm:"default:0" json:"-"`     // Hidden, part of computeCapacity (MB)
	MemoryLimit     int    `gorm:"default:0" json:"-"`     // Hidden, part of computeCapacity (MB)
	MemoryUnits     string `gorm:"default:'MB'" json:"-"`  // Hidden, part of computeCapacity

	// Provider VDC reference (stored as separate fields for GORM)
	ProviderVdcID   string `gorm:"type:varchar(255)" json:"-"` // Hidden, part of providerVdc
	ProviderVdcName string `json:"-"`                          // Hidden, part of providerVdc

	// VDC quotas and settings
	NicQuota        int  `gorm:"default:100" json:"nicQuota"`
	NetworkQuota    int  `gorm:"default:50" json:"networkQuota"`
	IsThinProvision bool `gorm:"default:false" json:"isThinProvision"`
	IsEnabled       bool `gorm:"default:true" json:"isEnabled"`

	// Kubernetes integration (hidden from JSON)
	Namespace string `gorm:"uniqueIndex;size:253" json:"-"` // Kubernetes namespace for this VDC

	// Timestamps (hidden from JSON in VCD format)
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships (hidden from JSON)
	Organization *Organization `gorm:"foreignKey:OrganizationID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	VApps        []VApp        `gorm:"foreignKey:VDCID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// ComputeCapacity represents the compute capacity structure for VCD compliance
type ComputeCapacity struct {
	CPU    ComputeResource `json:"cpu"`
	Memory ComputeResource `json:"memory"`
}

// ComputeResource represents a compute resource with allocation, limit and units
type ComputeResource struct {
	Allocated int    `json:"allocated"`
	Limit     int    `json:"limit"`
	Units     string `json:"units"`
}

// ProviderVdc represents a provider VDC reference
type ProviderVdc struct {
	ID string `json:"id"`
}

// VdcStorageProfiles represents storage profiles (empty for now as specified)
type VdcStorageProfiles struct {
	// Empty for now as specified in requirements
}

// ComputeCapacity returns the VCD-compliant compute capacity structure
func (v *VDC) ComputeCapacity() ComputeCapacity {
	return ComputeCapacity{
		CPU: ComputeResource{
			Allocated: v.CPUAllocated,
			Limit:     v.CPULimit,
			Units:     v.CPUUnits,
		},
		Memory: ComputeResource{
			Allocated: v.MemoryAllocated,
			Limit:     v.MemoryLimit,
			Units:     v.MemoryUnits,
		},
	}
}

// SetComputeCapacity sets the compute capacity from VCD structure
func (v *VDC) SetComputeCapacity(cc ComputeCapacity) {
	v.CPUAllocated = cc.CPU.Allocated
	v.CPULimit = cc.CPU.Limit
	if cc.CPU.Units != "" {
		v.CPUUnits = cc.CPU.Units
	}
	v.MemoryAllocated = cc.Memory.Allocated
	v.MemoryLimit = cc.Memory.Limit
	if cc.Memory.Units != "" {
		v.MemoryUnits = cc.Memory.Units
	}
}

// ProviderVdc returns the provider VDC reference
func (v *VDC) ProviderVdc() ProviderVdc {
	return ProviderVdc{
		ID: v.ProviderVdcID,
	}
}

// SetProviderVdc sets the provider VDC reference
func (v *VDC) SetProviderVdc(pv ProviderVdc) {
	v.ProviderVdcID = pv.ID
}

// VdcStorageProfiles returns empty storage profiles as specified
func (v *VDC) VdcStorageProfiles() VdcStorageProfiles {
	return VdcStorageProfiles{}
}

// BeforeCreate sets up the VDC before database creation
func (v *VDC) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = GenerateVDCURN()
	}

	// Generate namespace name if not set
	if v.Namespace == "" {
		// Load organization to get name
		var org Organization
		if err := tx.Where("id = ?", v.OrganizationID).First(&org).Error; err != nil {
			return fmt.Errorf("failed to load organization: %w", err)
		}
		
		// Generate unique namespace name
		namespace, err := generateUniqueNamespaceName(tx, org.Name, v.Name)
		if err != nil {
			return fmt.Errorf("failed to generate unique namespace: %w", err)
		}
		v.Namespace = namespace
	}

	// Set default units if not provided
	if v.CPUUnits == "" {
		v.CPUUnits = "MHz"
	}
	if v.MemoryUnits == "" {
		v.MemoryUnits = "MB"
	}

	return nil
}

// generateUniqueNamespaceName creates a unique Kubernetes-compliant namespace name
func generateUniqueNamespaceName(tx *gorm.DB, orgName, vdcName string) (string, error) {
	// Start with the base name
	baseName := generateNamespaceName(orgName, vdcName)
	
	// Check if the base name is available
	var existingVDC VDC
	err := tx.Where("namespace = ?", baseName).First(&existingVDC).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Base name is available
			return baseName, nil
		}
		// Database error
		return "", err
	}
	
	// Base name is taken, try with incremental suffixes
	for i := 1; i <= 999; i++ {
		candidateName := fmt.Sprintf("%s-%d", baseName, i)
		
		err := tx.Where("namespace = ?", candidateName).First(&existingVDC).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// This candidate is available
				return candidateName, nil
			}
			// Database error
			return "", err
		}
		// This candidate is also taken, try the next one
	}
	
	// If we get here, we couldn't find a unique name after 999 attempts
	return "", fmt.Errorf("unable to generate unique namespace name for org '%s' and VDC '%s'", orgName, vdcName)
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
