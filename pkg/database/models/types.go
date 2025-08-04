package models

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