package types

// Page represents a paginated response following VMware Cloud Director API specification
type Page[T any] struct {
	ResultTotal  int64 `json:"resultTotal"`
	PageCount    int   `json:"pageCount"`
	Page         int   `json:"page"`
	PageSize     int   `json:"pageSize"`
	Associations []any `json:"associations"`
	Values       []T   `json:"values"`
}

// NewPage creates a new paginated response
func NewPage[T any](values []T, page, pageSize int, totalCount int64) *Page[T] {
	// Validate and normalize parameters to prevent division-by-zero and ensure valid pagination
	if pageSize <= 0 {
		pageSize = 1
	}
	if page < 1 {
		page = 1
	}
	if totalCount < 0 {
		totalCount = 0
	}

	var pageCount int
	if totalCount > 0 {
		pageCount = int((totalCount + int64(pageSize) - 1) / int64(pageSize)) // Ceiling division
	}

	return &Page[T]{
		ResultTotal:  totalCount,
		PageCount:    pageCount,
		Page:         page,
		PageSize:     pageSize,
		Associations: []any{}, // Always empty array for VMware Cloud Director compliance
		Values:       values,
	}
}
