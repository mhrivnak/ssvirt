package models

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// AllocationModel represents the allocation model for VDCs
type AllocationModel string

const (
	PayAsYouGo      AllocationModel = "PayAsYouGo"
	AllocationPool  AllocationModel = "AllocationPool"
	ReservationPool AllocationModel = "ReservationPool"
)

// Valid checks if the allocation model is valid
func (am AllocationModel) Valid() bool {
	switch am {
	case PayAsYouGo, AllocationPool, ReservationPool:
		return true
	default:
		return false
	}
}

// String returns the string representation
func (am AllocationModel) String() string {
	return string(am)
}

// URN constants for VMware Cloud Director compatibility
const (
	URNPrefixUser = "urn:vcloud:user:"
	URNPrefixOrg  = "urn:vcloud:org:"
	URNPrefixRole = "urn:vcloud:role:"
)

// Role constants
const (
	RoleSystemAdmin = "System Administrator"
	RoleOrgAdmin    = "Organization Administrator"
	RoleVAppUser    = "vApp User"
)

// Default organization name
const (
	DefaultOrgName = "Provider"
)

// EntityRef represents a reference to another entity
type EntityRef struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// URN helper functions
func GenerateUserURN() string {
	return URNPrefixUser + uuid.New().String()
}

func GenerateOrgURN() string {
	return URNPrefixOrg + uuid.New().String()
}

func GenerateRoleURN() string {
	return URNPrefixRole + uuid.New().String()
}

// ParseURN extracts the UUID from a URN
func ParseURN(urn string) (string, error) {
	if urn == "" {
		return "", fmt.Errorf("empty URN")
	}
	
	// Check for valid URN prefixes
	var prefix string
	switch {
	case strings.HasPrefix(urn, URNPrefixUser):
		prefix = URNPrefixUser
	case strings.HasPrefix(urn, URNPrefixOrg):
		prefix = URNPrefixOrg
	case strings.HasPrefix(urn, URNPrefixRole):
		prefix = URNPrefixRole
	default:
		return "", fmt.Errorf("invalid URN prefix: %s", urn)
	}
	
	uuidStr := strings.TrimPrefix(urn, prefix)
	if _, err := uuid.Parse(uuidStr); err != nil {
		return "", fmt.Errorf("invalid UUID in URN: %s", uuidStr)
	}
	
	return uuidStr, nil
}

// GetURNType returns the type of entity from a URN
func GetURNType(urn string) (string, error) {
	switch {
	case strings.HasPrefix(urn, URNPrefixUser):
		return "user", nil
	case strings.HasPrefix(urn, URNPrefixOrg):
		return "org", nil
	case strings.HasPrefix(urn, URNPrefixRole):
		return "role", nil
	default:
		return "", fmt.Errorf("unknown URN type: %s", urn)
	}
}
