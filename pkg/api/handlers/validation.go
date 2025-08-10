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

// URN validation regex patterns used across handlers for VMware Cloud Director compliance.
// These patterns ensure that resource identifiers match the expected VCD URN format.
var (
	// vdcURNRegex validates VDC (Virtual Data Center) URN format.
	// Pattern: urn:vcloud:vdc:UUID
	// Example: urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc
	vdcURNRegex = regexp.MustCompile(`^urn:vcloud:vdc:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)
