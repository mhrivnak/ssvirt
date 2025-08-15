// Package handlers provides shared validation patterns for VMware Cloud Director API handlers.
//
// This file contains common validation regex patterns used across multiple handler files
// to ensure consistent URN format validation throughout the API implementation.
//
// VMware Cloud Director uses Uniform Resource Names (URNs) to uniquely identify resources
// across the system. These URNs follow specific patterns that must be validated to ensure
// API compliance and security.
package handlers

import "regexp"

// Input validation patterns for non-URN fields used across handlers.
// URN validation is now centralized in models.ParseURN and models.GetURNType.
var (
	// dns1123LabelRegex validates DNS-1123 label format for Kubernetes compatibility.
	// Requirements:
	// - Must be lowercase
	// - Must contain only lowercase letters, numbers, and hyphens
	// - Must start and end with alphanumeric characters
	// - Must be 1-63 characters long
	dns1123LabelRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

	// catalogItemURNRegex validates catalog item URN suffix format.
	// Allows alphanumeric characters, hyphens, underscores, and colons.
	// Supports both legacy 4-part format (item-name) and 5-part format (catalog-id:item-name)
	catalogItemURNRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_:]+$`)
)
