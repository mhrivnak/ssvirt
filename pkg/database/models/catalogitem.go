package models

// CatalogItem represents a VCD-compliant catalog item backed by OpenShift Templates
type CatalogItem struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	CatalogID    string            `json:"catalogId"`
	IsPublished  bool              `json:"isPublished"`
	IsExpired    bool              `json:"isExpired"`
	CreationDate string            `json:"creationDate"`
	Size         int64             `json:"size"`
	Status       string            `json:"status"`
	Entity       CatalogItemEntity `json:"entity"`
	Owner        EntityRef         `json:"owner"`
	Catalog      EntityRef         `json:"catalog"`
}

// CatalogItemEntity represents the detailed entity information for a catalog item
type CatalogItemEntity struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	Type              string `json:"type"`
	NumberOfVMs       int    `json:"numberOfVMs"`
	NumberOfCpus      int    `json:"numberOfCpus"`
	MemoryAllocation  int64  `json:"memoryAllocation"`
	StorageAllocation int64  `json:"storageAllocation"`
}
