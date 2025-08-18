package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mhrivnak/ssvirt/pkg/api/handlers"
	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
	"github.com/mhrivnak/ssvirt/pkg/services"
)

// MockKubernetesService is a mock implementation of KubernetesService
type MockKubernetesService struct {
	mock.Mock
}

func (m *MockKubernetesService) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockKubernetesService) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockKubernetesService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error {
	args := m.Called(ctx, vdc, org)
	return args.Error(0)
}

func (m *MockKubernetesService) UpdateNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error {
	args := m.Called(ctx, vdc, org)
	return args.Error(0)
}

func (m *MockKubernetesService) DeleteNamespaceForVDC(ctx context.Context, vdc *models.VDC) error {
	args := m.Called(ctx, vdc)
	return args.Error(0)
}

func (m *MockKubernetesService) EnsureNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error {
	args := m.Called(ctx, vdc, org)
	return args.Error(0)
}

func (m *MockKubernetesService) GetTemplate(ctx context.Context, name string) (*services.TemplateInfo, error) {
	args := m.Called(ctx, name)
	if template := args.Get(0); template != nil {
		return template.(*services.TemplateInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockKubernetesService) CreateTemplateInstance(ctx context.Context, req *services.TemplateInstanceRequest) (*services.TemplateInstanceResult, error) {
	args := m.Called(ctx, req)
	if result := args.Get(0); result != nil {
		return result.(*services.TemplateInstanceResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockKubernetesService) GetTemplateInstance(ctx context.Context, namespace, name string) (*services.TemplateInstanceStatus, error) {
	args := m.Called(ctx, namespace, name)
	if status := args.Get(0); status != nil {
		return status.(*services.TemplateInstanceStatus), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockKubernetesService) DeleteTemplateInstance(ctx context.Context, namespace, name string) error {
	args := m.Called(ctx, namespace, name)
	return args.Error(0)
}

func (m *MockKubernetesService) EnsureNamespaceResources(ctx context.Context, namespace string, vdc *models.VDC) error {
	args := m.Called(ctx, namespace, vdc)
	return args.Error(0)
}

func (m *MockKubernetesService) GetClient() client.Client {
	args := m.Called()
	if clientVal := args.Get(0); clientVal != nil {
		return clientVal.(client.Client)
	}
	return nil
}

func TestVAppDeletion_CleansUpTemplateInstance(t *testing.T) {
	// Setup test infrastructure
	_, db, jwtManager := setupTestAPIServer(t)

	// Create mock Kubernetes service
	mockK8sService := &MockKubernetesService{}

	// Setup repositories
	orgRepo := repositories.NewOrganizationRepository(db.DB)
	vdcRepo := repositories.NewVDCRepository(db.DB)
	vappRepo := repositories.NewVAppRepository(db.DB)
	vmRepo := repositories.NewVMRepository(db.DB)

	// Create VApp handlers with mock K8s service
	vappHandlers := handlers.NewVAppHandlers(vappRepo, vdcRepo, vmRepo, mockK8sService)

	// Create test data
	// 1. Create organization
	org := &models.Organization{
		Name:        "TestOrg",
		DisplayName: "Test Organization",
		IsEnabled:   true,
	}
	require.NoError(t, orgRepo.Create(org))

	// 2. Create VDC
	vdc := &models.VDC{
		Name:            "TestVDC",
		Description:     "Test VDC for vApp deletion",
		OrganizationID:  org.ID,
		AllocationModel: models.PayAsYouGo,
		Namespace:       "test-namespace",
		IsEnabled:       true,
	}
	require.NoError(t, vdcRepo.Create(vdc))

	// 3. Create vApp
	vapp := &models.VApp{
		Name:        "test-vapp",
		Description: "Test vApp for deletion",
		VDCID:       vdc.ID,
		Status:      models.VAppStatusDeployed,
	}
	require.NoError(t, vappRepo.CreateWithContext(context.Background(), vapp))

	// 4. Create user
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// 5. Associate user with organization
	user.OrganizationID = &org.ID
	require.NoError(t, db.DB.Save(user).Error)

	// Setup mock expectations - DeleteTemplateInstance should be called
	mockK8sService.On("DeleteTemplateInstance", mock.Anything, vdc.Namespace, vapp.Name).Return(nil)

	// Generate JWT token
	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	// Setup gin in test mode
	gin.SetMode(gin.TestMode)

	// Create test request
	router := gin.New()
	router.DELETE("/cloudapi/1.0.0/vapps/:vapp_id", func(c *gin.Context) {
		// Set mock claims
		claims := &auth.Claims{
			UserID: user.ID,
		}
		c.Set(auth.ClaimsContextKey, claims)
		vappHandlers.DeleteVApp(c)
	})

	req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/"+vapp.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify the response
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify that DeleteTemplateInstance was called with correct parameters
	mockK8sService.AssertCalled(t, "DeleteTemplateInstance", mock.Anything, vdc.Namespace, vapp.Name)

	// Verify vApp was deleted from database
	var deletedVApp models.VApp
	err = db.DB.Where("id = ?", vapp.ID).First(&deletedVApp).Error
	assert.Error(t, err) // Should not be found
}

func TestVAppDeletion_HandlesKubernetesError(t *testing.T) {
	// Setup test infrastructure
	_, db, jwtManager := setupTestAPIServer(t)

	// Create mock Kubernetes service
	mockK8sService := &MockKubernetesService{}

	// Setup repositories
	orgRepo := repositories.NewOrganizationRepository(db.DB)
	vdcRepo := repositories.NewVDCRepository(db.DB)
	vappRepo := repositories.NewVAppRepository(db.DB)
	vmRepo := repositories.NewVMRepository(db.DB)

	// Create VApp handlers with mock K8s service
	vappHandlers := handlers.NewVAppHandlers(vappRepo, vdcRepo, vmRepo, mockK8sService)

	// Create test data
	// 1. Create organization
	org := &models.Organization{
		Name:        "TestOrg",
		DisplayName: "Test Organization",
		IsEnabled:   true,
	}
	require.NoError(t, orgRepo.Create(org))

	// 2. Create VDC
	vdc := &models.VDC{
		Name:            "TestVDC",
		Description:     "Test VDC for vApp deletion",
		OrganizationID:  org.ID,
		AllocationModel: models.PayAsYouGo,
		Namespace:       "test-namespace",
		IsEnabled:       true,
	}
	require.NoError(t, vdcRepo.Create(vdc))

	// 3. Create vApp
	vapp := &models.VApp{
		Name:        "test-vapp",
		Description: "Test vApp for deletion",
		VDCID:       vdc.ID,
		Status:      models.VAppStatusDeployed,
	}
	require.NoError(t, vappRepo.CreateWithContext(context.Background(), vapp))

	// 4. Create user
	user := &models.User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
		Enabled:  true,
	}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.DB.Create(user).Error)

	// 5. Associate user with organization
	user.OrganizationID = &org.ID
	require.NoError(t, db.DB.Save(user).Error)

	// Setup mock expectations - K8s service returns error but vApp deletion continues
	mockK8sService.On("DeleteTemplateInstance", mock.Anything, vdc.Namespace, vapp.Name).Return(assert.AnError)

	// Generate JWT token
	token, err := jwtManager.Generate(user.ID, user.Username)
	require.NoError(t, err)

	// Setup gin in test mode
	gin.SetMode(gin.TestMode)

	// Create test request
	router := gin.New()
	router.DELETE("/cloudapi/1.0.0/vapps/:vapp_id", func(c *gin.Context) {
		// Set mock claims
		claims := &auth.Claims{
			UserID: user.ID,
		}
		c.Set(auth.ClaimsContextKey, claims)
		vappHandlers.DeleteVApp(c)
	})

	req, _ := http.NewRequest("DELETE", "/cloudapi/1.0.0/vapps/"+vapp.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify the response - should still succeed despite K8s error
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify that DeleteTemplateInstance was called
	mockK8sService.AssertCalled(t, "DeleteTemplateInstance", mock.Anything, vdc.Namespace, vapp.Name)

	// Verify vApp was still deleted from database despite K8s error
	var deletedVApp models.VApp
	err = db.DB.Where("id = ?", vapp.ID).First(&deletedVApp).Error
	assert.Error(t, err) // Should not be found
}
