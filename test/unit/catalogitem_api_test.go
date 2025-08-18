package unit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/services"
)

// MockCatalogItemRepository is a mock implementation of CatalogItemRepository
type MockCatalogItemRepository struct {
	mock.Mock
}

func (m *MockCatalogItemRepository) ListByCatalogID(ctx context.Context, catalogID string, limit, offset int) ([]models.CatalogItem, error) {
	args := m.Called(ctx, catalogID, limit, offset)
	return args.Get(0).([]models.CatalogItem), args.Error(1)
}

func (m *MockCatalogItemRepository) CountByCatalogID(ctx context.Context, catalogID string) (int64, error) {
	args := m.Called(ctx, catalogID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCatalogItemRepository) GetByID(ctx context.Context, catalogID, itemID string) (*models.CatalogItem, error) {
	args := m.Called(ctx, catalogID, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CatalogItem), args.Error(1)
}

func TestCatalogItemAPIEndpoints(t *testing.T) {
	server, db, jwtManager := setupTestAPIServer(t)
	router := server.GetRouter()

	// Create test organization
	org := &models.Organization{
		Name:        "Test Organization",
		DisplayName: "Test Organization Full Name",
		Description: "Test organization for catalog item API testing",
		IsEnabled:   true,
	}
	require.NoError(t, db.DB.Create(org).Error)

	// Create test catalog
	catalog := &models.Catalog{
		Name:           "Test Catalog",
		Description:    "Test catalog for catalog item testing",
		OrganizationID: org.ID,
		IsPublished:    false,
		IsSubscribed:   false,
		IsLocal:        true,
		Version:        1,
	}
	require.NoError(t, db.DB.Create(catalog).Error)

	// Create test user with vApp User role
	user := &models.User{
		Username: "testuser",
		Email:    "testuser@example.com",
		FullName: "Test User",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// Create vApp User role
	userRole := &models.Role{
		Name:        models.RoleVAppUser,
		Description: "vApp User role",
	}
	require.NoError(t, db.DB.Create(userRole).Error)

	// Generate token
	userToken, err := jwtManager.GenerateWithRole(user.ID, user.Username, org.ID, models.RoleVAppUser)
	require.NoError(t, err)

	t.Run("List Catalog Items Tests", func(t *testing.T) {
		t.Run("List catalog items with valid catalog returns paginated results", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems", catalog.ID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[models.CatalogItem]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Should have pagination structure
			assert.Equal(t, 1, response.Page)
			assert.Equal(t, 25, response.PageSize)
			assert.GreaterOrEqual(t, response.PageCount, 0)
			assert.GreaterOrEqual(t, response.ResultTotal, int64(0))
		})

		t.Run("List catalog items with pagination parameters", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems?page=1&pageSize=10", catalog.ID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[models.CatalogItem]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, 1, response.Page)
			assert.Equal(t, 10, response.PageSize)
		})

		t.Run("List catalog items returns results sorted alphabetically by name", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems", catalog.ID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response types.Page[models.CatalogItem]
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Verify that results are sorted alphabetically by name
			// If there are multiple items, check that they are in alphabetical order
			if len(response.Values) > 1 {
				for i := 1; i < len(response.Values); i++ {
					assert.LessOrEqual(t, response.Values[i-1].Name, response.Values[i].Name,
						"Catalog items should be sorted alphabetically by name")
				}
			}
		})

		t.Run("List catalog items with invalid catalog URN returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/cloudapi/1.0.0/catalogs/invalid-urn/catalogItems", nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "Bad Request", response["error"])
			assert.Contains(t, response["message"], "Invalid catalog ID format")
		})

		t.Run("List catalog items with non-existent catalog returns 404", func(t *testing.T) {
			nonExistentCatalogID := "urn:vcloud:catalog:00000000-0000-0000-0000-000000000000"
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems", nonExistentCatalogID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("List catalog items without authorization returns 401", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems", catalog.ID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})

	t.Run("Get Catalog Item Tests", func(t *testing.T) {
		t.Run("Get catalog item with invalid catalog URN returns 400", func(t *testing.T) {
			itemID := "urn:vcloud:catalogitem:12345678-1234-1234-1234-123456789abc"
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/invalid-urn/catalogItems/%s", itemID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "Bad Request", response["error"])
			assert.Contains(t, response["message"], "Invalid catalog ID format")
		})

		t.Run("Get catalog item with invalid item URN returns 400", func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems/invalid-urn", catalog.ID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "Bad Request", response["error"])
			assert.Contains(t, response["message"], "Invalid catalog item ID format")
		})

		t.Run("Get non-existent catalog item returns 404", func(t *testing.T) {
			nonExistentItemID := "urn:vcloud:catalogitem:00000000-0000-0000-0000-000000000000"
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems/%s", catalog.ID, nonExistentItemID), nil)
			req.Header.Set("Authorization", "Bearer "+userToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("Get catalog item without authorization returns 401", func(t *testing.T) {
			itemID := "urn:vcloud:catalogitem:12345678-1234-1234-1234-123456789abc"
			req, _ := http.NewRequest("GET", fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems/%s", catalog.ID, itemID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
}

func TestTemplateMapper(t *testing.T) {
	mapper := &services.TemplateMapper{}

	t.Run("ExtractVMCount", func(t *testing.T) {
		t.Run("Returns 1 for template with no objects", func(t *testing.T) {
			template := &templatev1.Template{}
			count := mapper.ExtractVMCount(template)
			assert.Equal(t, 1, count)
		})

		t.Run("Counts VirtualMachine objects correctly", func(t *testing.T) {
			template := &templatev1.Template{
				Objects: []runtime.RawExtension{
					{
						Raw: []byte(`{"kind": "VirtualMachine", "apiVersion": "kubevirt.io/v1"}`),
					},
					{
						Raw: []byte(`{"kind": "VirtualMachine", "apiVersion": "kubevirt.io/v1"}`),
					},
					{
						Raw: []byte(`{"kind": "Service", "apiVersion": "v1"}`),
					},
				},
			}
			count := mapper.ExtractVMCount(template)
			assert.Equal(t, 2, count)
		})
	})

	t.Run("ExtractResourceRequirements", func(t *testing.T) {
		t.Run("Returns default values for template with no parameters", func(t *testing.T) {
			template := &templatev1.Template{}
			cpus, memory, storage := mapper.ExtractResourceRequirements(template)
			assert.Equal(t, 1, cpus)
			assert.Equal(t, int64(1024*1024*1024), memory)
			assert.Equal(t, int64(10*1024*1024*1024), storage)
		})

		t.Run("Extracts resource values from parameters", func(t *testing.T) {
			template := &templatev1.Template{
				Parameters: []templatev1.Parameter{
					{
						Name:  "CPU",
						Value: "4",
					},
					{
						Name:  "MEMORY",
						Value: "8589934592", // 8GB
					},
					{
						Name:  "STORAGE",
						Value: "42949672960", // 40GB
					},
				},
			}
			cpus, memory, storage := mapper.ExtractResourceRequirements(template)
			assert.Equal(t, 4, cpus)
			assert.Equal(t, int64(8589934592), memory)
			assert.Equal(t, int64(42949672960), storage)
		})
	})

	t.Run("TemplateToCatalogItem", func(t *testing.T) {
		template := &templatev1.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-template",
				UID:  "12345678-1234-1234-1234-123456789abc",
				Annotations: map[string]string{
					"description": "Test template description",
				},
				Labels: map[string]string{
					"catalog.ssvirt.io/published": "true",
				},
				CreationTimestamp: metav1.NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
			},
			Parameters: []templatev1.Parameter{
				{
					Name:  "CPU",
					Value: "2",
				},
			},
		}

		catalogID := "urn:vcloud:catalog:test-catalog"
		catalogItem := mapper.TemplateToCatalogItem(template, catalogID)

		assert.Equal(t, "urn:vcloud:catalogitem:test-catalog:test-template", catalogItem.ID)
		assert.Equal(t, "test-template", catalogItem.Name)
		assert.Equal(t, "Test template description", catalogItem.Description)
		assert.Equal(t, catalogID, catalogItem.CatalogID)
		assert.True(t, catalogItem.IsPublished)
		assert.False(t, catalogItem.IsExpired)
		assert.Equal(t, "AVAILABLE", catalogItem.Status)
		assert.Equal(t, "2024-01-15T10:30:00Z", catalogItem.CreationDate)

		// Check entity
		assert.Equal(t, "test-template", catalogItem.Entity.Name)
		assert.Equal(t, "Test template description", catalogItem.Entity.Description)
		assert.Equal(t, "application/vnd.vmware.vcloud.vAppTemplate+xml", catalogItem.Entity.Type)
		assert.Equal(t, 1, catalogItem.Entity.NumberOfVMs)
		assert.Equal(t, 2, catalogItem.Entity.NumberOfCpus)

		// Check references
		assert.Equal(t, "System", catalogItem.Owner.Name)
		assert.Equal(t, "", catalogItem.Owner.ID)
		assert.Equal(t, "Templates", catalogItem.Catalog.Name)
		assert.Equal(t, catalogID, catalogItem.Catalog.ID)
	})
}
