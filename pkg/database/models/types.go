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
	Flex            AllocationModel = "Flex"
)

// Valid checks if the allocation model is valid
func (am AllocationModel) Valid() bool {
	switch am {
	case PayAsYouGo, AllocationPool, ReservationPool, Flex:
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
	URNPrefixUser    = "urn:vcloud:user:"
	URNPrefixOrg     = "urn:vcloud:org:"
	URNPrefixRole    = "urn:vcloud:role:"
	URNPrefixSession = "urn:vcloud:session:"
	URNPrefixVDC     = "urn:vcloud:vdc:"
	URNPrefixCatalog = "urn:vcloud:catalog:"
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

func GenerateSessionURN() string {
	return URNPrefixSession + uuid.New().String()
}

func GenerateVDCURN() string {
	return URNPrefixVDC + uuid.New().String()
}

func GenerateCatalogURN() string {
	return URNPrefixCatalog + uuid.New().String()
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
	case strings.HasPrefix(urn, URNPrefixSession):
		prefix = URNPrefixSession
	case strings.HasPrefix(urn, URNPrefixVDC):
		prefix = URNPrefixVDC
	case strings.HasPrefix(urn, URNPrefixCatalog):
		prefix = URNPrefixCatalog
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
	case strings.HasPrefix(urn, URNPrefixSession):
		return "session", nil
	case strings.HasPrefix(urn, URNPrefixVDC):
		return "vdc", nil
	case strings.HasPrefix(urn, URNPrefixCatalog):
		return "catalog", nil
	default:
		return "", fmt.Errorf("unknown URN type: %s", urn)
	}
}
