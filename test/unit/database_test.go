package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.Role{},
		&models.VDC{},
		&models.Catalog{},
		&models.VAppTemplate{},
		&models.VApp{},
		&models.VM{},
	)
	require.NoError(t, err)

	return db
}

func TestOrganizationRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewOrganizationRepository(db)

	// Test Create
	org := &models.Organization{
		Name:        "test-org",
		DisplayName: "Test Organization",
		Description: "Test organization for unit tests",
	}

	err := repo.Create(org)
	require.NoError(t, err)
	assert.NotEmpty(t, org.ID)
	assert.Contains(t, org.ID, "urn:vcloud:org:")

	// Test GetByID
	retrieved, err := repo.GetByID(org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.Name, retrieved.Name)
	assert.Equal(t, org.DisplayName, retrieved.DisplayName)

	// Test GetByName
	retrieved, err = repo.GetByName("test-org")
	require.NoError(t, err)
	assert.Equal(t, org.ID, retrieved.ID)

	// Test List
	orgs, err := repo.List()
	require.NoError(t, err)
	assert.Len(t, orgs, 1)

	// Test Update
	org.DisplayName = "Updated Test Organization"
	err = repo.Update(org)
	require.NoError(t, err)

	retrieved, err = repo.GetByID(org.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Test Organization", retrieved.DisplayName)
}

func TestVDCRepository(t *testing.T) {
	db := setupTestDB(t)
	orgRepo := repositories.NewOrganizationRepository(db)
	vdcRepo := repositories.NewVDCRepository(db)

	// Create organization first
	org := &models.Organization{
		Name: "test-org",
	}
	err := orgRepo.Create(org)
	require.NoError(t, err)

	// Test Create VDC
	vdc := &models.VDC{
		Name:            "test-vdc",
		OrganizationID:  org.ID,
		AllocationModel: "PayAsYouGo",
		CPULimit:        intPtr(100),
		MemoryLimitMB:   intPtr(8192),
	}

	err = vdcRepo.Create(vdc)
	require.NoError(t, err)
	assert.NotEmpty(t, vdc.ID)

	// Test GetByID
	retrieved, err := vdcRepo.GetByID(vdc.ID)
	require.NoError(t, err)
	assert.Equal(t, vdc.Name, retrieved.Name)
	assert.Equal(t, vdc.OrganizationID, retrieved.OrganizationID)

	// Test GetByOrganizationID
	vdcs, err := vdcRepo.GetByOrganizationID(org.ID)
	require.NoError(t, err)
	assert.Len(t, vdcs, 1)
}

func TestCatalogRepository(t *testing.T) {
	db := setupTestDB(t)
	orgRepo := repositories.NewOrganizationRepository(db)
	catalogRepo := repositories.NewCatalogRepository(db)

	// Create organization first
	org := &models.Organization{
		Name: "test-org",
	}
	err := orgRepo.Create(org)
	require.NoError(t, err)

	// Test Create Catalog
	catalog := &models.Catalog{
		Name:           "test-catalog",
		OrganizationID: org.ID,
		Description:    "Test catalog",
		IsShared:       false,
	}

	err = catalogRepo.Create(catalog)
	require.NoError(t, err)
	assert.NotEmpty(t, catalog.ID)

	// Test GetByID
	retrieved, err := catalogRepo.GetByID(catalog.ID)
	require.NoError(t, err)
	assert.Equal(t, catalog.Name, retrieved.Name)

	// Test GetByOrganizationID
	catalogs, err := catalogRepo.GetByOrganizationID(org.ID)
	require.NoError(t, err)
	assert.Len(t, catalogs, 1)
}

func intPtr(i int) *int {
	return &i
}
