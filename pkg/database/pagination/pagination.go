// Package pagination provides shared utilities for secure pagination and sorting in database repositories.
//
// This package implements security measures to prevent SQL injection attacks and abuse of pagination
// parameters by validating sort columns against whitelists and clamping limit/offset values.
package pagination

import (
	"strings"
)

// Pagination constants for security
const (
	MaxPageSize   = 100
	MaxOffset     = 100000
	DefaultLimit  = 25
	DefaultOffset = 0
)

// SanitizeSortOrder validates and sanitizes the sort order to prevent SQL injection
// columnWhitelist should contain valid column names for the specific entity
func SanitizeSortOrder(sortOrder string, columnWhitelist map[string]bool, defaultSort string) string {
	if sortOrder == "" {
		return defaultSort
	}

	// Parse the sort order string (could be multiple columns)
	parts := strings.Split(sortOrder, ",")
	var validParts []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split column and direction
		tokens := strings.Fields(part)
		if len(tokens) == 0 {
			continue
		}

		column := strings.ToLower(strings.TrimSpace(tokens[0]))
		direction := "ASC" // Default direction

		if len(tokens) > 1 {
			dir := strings.ToUpper(strings.TrimSpace(tokens[1]))
			if dir == "DESC" || dir == "ASC" {
				direction = dir
			}
		}

		// Validate column name against whitelist
		if columnWhitelist[column] {
			validParts = append(validParts, column+" "+direction)
		}
	}

	if len(validParts) == 0 {
		return defaultSort // Safe default
	}

	return strings.Join(validParts, ", ")
}

// ClampPaginationParams ensures limit and offset are within safe bounds
func ClampPaginationParams(limit, offset int) (int, int) {
	// Clamp limit
	if limit <= 0 {
		limit = DefaultLimit
	} else if limit > MaxPageSize {
		limit = MaxPageSize
	}

	// Clamp offset
	if offset < 0 {
		offset = DefaultOffset
	} else if offset > MaxOffset {
		offset = MaxOffset
	}

	return limit, offset
}

// Common column whitelists for different entities

// VAppSortColumns defines valid sort columns for vApp entities
var VAppSortColumns = map[string]bool{
	"id":          true,
	"name":        true,
	"status":      true,
	"created_at":  true,
	"updated_at":  true,
	"description": true,
}

// VDCSortColumns defines valid sort columns for VDC entities
var VDCSortColumns = map[string]bool{
	"id":                true,
	"name":              true,
	"created_at":        true,
	"updated_at":        true,
	"description":       true,
	"allocation_model":  true,
	"provider_vdc_name": true,
}

// CatalogSortColumns defines valid sort columns for Catalog entities
var CatalogSortColumns = map[string]bool{
	"id":          true,
	"name":        true,
	"created_at":  true,
	"updated_at":  true,
	"description": true,
}

// UserSortColumns defines valid sort columns for User entities
var UserSortColumns = map[string]bool{
	"id":         true,
	"username":   true,
	"email":      true,
	"full_name":  true,
	"created_at": true,
	"updated_at": true,
}

// OrganizationSortColumns defines valid sort columns for Organization entities
var OrganizationSortColumns = map[string]bool{
	"id":           true,
	"name":         true,
	"display_name": true,
	"created_at":   true,
	"updated_at":   true,
	"description":  true,
}

// RoleSortColumns defines valid sort columns for Role entities
var RoleSortColumns = map[string]bool{
	"id":          true,
	"name":        true,
	"created_at":  true,
	"updated_at":  true,
	"description": true,
}
