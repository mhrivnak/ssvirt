package types

// Page represents a paginated response following VMware Cloud Director API specification
type Page[T any] struct {
	ResultTotal  int   `json:"resultTotal"`
	PageCount    int   `json:"pageCount"`
	Page         int   `json:"page"`
	PageSize     int   `json:"pageSize"`
	Associations []any `json:"associations"`
	Values       []T   `json:"values"`
}

// NewPage creates a new paginated response
func NewPage[T any](values []T, page, pageSize, totalCount int) *Page[T] {
	var pageCount int
	if totalCount > 0 {
		pageCount = (totalCount + pageSize - 1) / pageSize // Ceiling division
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
