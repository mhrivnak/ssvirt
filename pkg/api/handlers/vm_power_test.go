package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// MockVMRepository mocks the VM repository
type MockVMRepository struct {
	mock.Mock
}

func (m *MockVMRepository) GetByID(id string) (*models.VM, error) {
	args := m.Called(id)
	if vm := args.Get(0); vm != nil {
		return vm.(*models.VM), args.Error(1)
	}
	return nil, args.Error(1)
}

func setupTest() (*gin.Engine, *MockVMRepository, client.Client) {
	gin.SetMode(gin.TestMode)

	// Create mock repository
	mockRepo := new(MockVMRepository)

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	_ = kubevirtv1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create handler
	logger := slog.Default()
	handler := NewPowerManagementHandler(mockRepo, fakeClient, logger)

	// Setup router
	router := gin.New()
	router.POST("/cloudapi/1.0.0/vms/:id/actions/powerOn", handler.PowerOn)
	router.POST("/cloudapi/1.0.0/vms/:id/actions/powerOff", handler.PowerOff)

	return router, mockRepo, fakeClient
}

func TestPowerOnHandler_Success(t *testing.T) {
	router, mockRepo, k8sClient := setupTest()

	// Create test VM
	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_OFF",
	}

	// Create VirtualMachine resource in fake client
	vmResource := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm",
			Namespace: "test-namespace",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &[]kubevirtv1.VirtualMachineRunStrategy{kubevirtv1.RunStrategyHalted}[0],
		},
	}
	err := k8sClient.Create(context.Background(), vmResource)
	assert.NoError(t, err)

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOn", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response PowerOperationResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("urn:vcloud:vm:%s", vmID), response.ID)
	assert.Equal(t, "test-vm", response.Name)
	assert.Equal(t, "POWERING_ON", response.Status)
	assert.Equal(t, "POWERING_ON", response.PowerState)

	mockRepo.AssertExpectations(t)
}

func TestPowerOnHandler_VMNotFound(t *testing.T) {
	router, mockRepo, _ := setupTest()

	vmID := uuid.New().String()

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(nil, gorm.ErrRecordNotFound)

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOn", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(404), response["code"])
	assert.Equal(t, "VM not found", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestPowerOnHandler_VMAlreadyPoweredOn(t *testing.T) {
	router, mockRepo, _ := setupTest()

	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_ON",
	}

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOn", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), response["code"])
	assert.Equal(t, "VM is already powered on or powering on", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestPowerOnHandler_ConflictingState(t *testing.T) {
	router, mockRepo, _ := setupTest()

	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "DELETING",
	}

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOn", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(409), response["code"])
	assert.Equal(t, "VM is in a conflicting state", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestPowerOnHandler_VirtualMachineNotFound(t *testing.T) {
	router, mockRepo, _ := setupTest()

	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_OFF",
	}

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request (VirtualMachine resource doesn't exist in fake client)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOn", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(404), response["code"])
	assert.Equal(t, "VirtualMachine resource not found in cluster", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestPowerOnHandler_InvalidUUID(t *testing.T) {
	router, _, _ := setupTest()

	// Make request with invalid UUID
	req, _ := http.NewRequest("POST", "/cloudapi/1.0.0/vms/invalid-uuid/actions/powerOn", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), response["code"])
	assert.Equal(t, "Invalid VM ID format", response["message"])
}

func TestPowerOnHandler_ValidURN(t *testing.T) {
	router, mockRepo, k8sClient := setupTest()

	// Create test VM
	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_OFF",
	}

	// Create VirtualMachine resource in fake client
	vmResource := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm",
			Namespace: "test-namespace",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &[]kubevirtv1.VirtualMachineRunStrategy{kubevirtv1.RunStrategyHalted}[0],
		},
	}
	err := k8sClient.Create(context.Background(), vmResource)
	assert.NoError(t, err)

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request with VM URN format
	vmURN := fmt.Sprintf("urn:vcloud:vm:%s", vmID)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOn", vmURN), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response PowerOperationResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("urn:vcloud:vm:%s", vmID), response.ID)
	assert.Equal(t, "test-vm", response.Name)
	assert.Equal(t, "POWERING_ON", response.Status)
	assert.Equal(t, "POWERING_ON", response.PowerState)
	assert.Equal(t, fmt.Sprintf("/cloudapi/1.0.0/vms/urn:vcloud:vm:%s", vmID), response.Href)

	mockRepo.AssertExpectations(t)
}

func TestPowerOffHandler_Success(t *testing.T) {
	router, mockRepo, k8sClient := setupTest()

	// Create test VM
	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_ON",
	}

	// Create VirtualMachine resource in fake client
	vmResource := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm",
			Namespace: "test-namespace",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			RunStrategy: &[]kubevirtv1.VirtualMachineRunStrategy{kubevirtv1.RunStrategyAlways}[0],
		},
	}
	err := k8sClient.Create(context.Background(), vmResource)
	assert.NoError(t, err)

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOff", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response PowerOperationResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("urn:vcloud:vm:%s", vmID), response.ID)
	assert.Equal(t, "test-vm", response.Name)
	assert.Equal(t, "POWERING_OFF", response.Status)
	assert.Equal(t, "POWERING_OFF", response.PowerState)

	mockRepo.AssertExpectations(t)
}

func TestPowerOffHandler_VMAlreadyPoweredOff(t *testing.T) {
	router, mockRepo, _ := setupTest()

	vmID := uuid.New().String()
	vm := &models.VM{
		ID:        vmID,
		Name:      "test-vm",
		VMName:    "test-vm",
		Namespace: "test-namespace",
		Status:    "POWERED_OFF",
	}

	// Setup mock expectations
	mockRepo.On("GetByID", vmID).Return(vm, nil)

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOff", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(400), response["code"])
	assert.Equal(t, "VM is already powered off or powering off", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestPowerOffHandler_DatabaseError(t *testing.T) {
	router, mockRepo, _ := setupTest()

	vmID := uuid.New().String()

	// Setup mock expectations - database error
	mockRepo.On("GetByID", vmID).Return(nil, errors.New("database connection failed"))

	// Make request
	req, _ := http.NewRequest("POST", fmt.Sprintf("/cloudapi/1.0.0/vms/%s/actions/powerOff", vmID), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(500), response["code"])
	assert.Equal(t, "Internal server error", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name     string
		uuid     string
		expected bool
	}{
		{
			name:     "valid UUID",
			uuid:     "12345678-1234-1234-1234-123456789abc",
			expected: true,
		},
		{
			name:     "invalid UUID",
			uuid:     "invalid-uuid",
			expected: false,
		},
		{
			name:     "empty string",
			uuid:     "",
			expected: false,
		},
		{
			name:     "UUID without hyphens",
			uuid:     "123456781234123412341234567890ab",
			expected: true,
		},
		{
			name:     "valid URN format",
			uuid:     "urn:vcloud:vm:12345678-1234-1234-1234-123456789abc",
			expected: true,
		},
		{
			name:     "invalid URN format",
			uuid:     "urn:vcloud:vm:invalid-uuid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidUUID(tt.uuid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseVMIDParam(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "valid UUID",
			input:       "12345678-1234-1234-1234-123456789abc",
			expected:    "12345678-1234-1234-1234-123456789abc",
			expectError: false,
		},
		{
			name:        "valid URN format",
			input:       "urn:vcloud:vm:12345678-1234-1234-1234-123456789abc",
			expected:    "12345678-1234-1234-1234-123456789abc",
			expectError: false,
		},
		{
			name:        "valid hyphenless UUID",
			input:       "123456781234123412341234567890ab",
			expected:    "12345678-1234-1234-1234-1234567890ab",
			expectError: false,
		},
		{
			name:        "invalid UUID",
			input:       "invalid-uuid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid URN",
			input:       "urn:vcloud:vm:invalid-uuid",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseVMIDParam(tt.input)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.expected, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatVMURN(t *testing.T) {
	vmID := "12345678-1234-1234-1234-123456789abc"
	expected := "urn:vcloud:vm:12345678-1234-1234-1234-123456789abc"

	result := formatVMURN(vmID)
	assert.Equal(t, expected, result)
}
