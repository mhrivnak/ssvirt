package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// VM URN validation regex - matches urn:vcloud:vm:UUID format
var vmURNRegex = regexp.MustCompile(`^urn:vcloud:vm:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// VMHandlers handles VM API endpoints
type VMHandlers struct {
	vmRepo   *repositories.VMRepository
	vappRepo *repositories.VAppRepository
	vdcRepo  *repositories.VDCRepository
}

// NewVMHandlers creates a new VMHandlers instance
func NewVMHandlers(vmRepo *repositories.VMRepository, vappRepo *repositories.VAppRepository, vdcRepo *repositories.VDCRepository) *VMHandlers {
	return &VMHandlers{
		vmRepo:   vmRepo,
		vappRepo: vappRepo,
		vdcRepo:  vdcRepo,
	}
}

// VMResponse represents the detailed response for VM information
type VMResponse struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Status             string              `json:"status"`
	VAppID             string              `json:"vappId"`
	TemplateID         string              `json:"templateId,omitempty"`
	CreatedAt          string              `json:"createdAt"`
	UpdatedAt          string              `json:"updatedAt"`
	GuestOS            string              `json:"guestOs"`
	VMTools            VMToolsInfo         `json:"vmTools"`
	Hardware           HardwareInfo        `json:"hardware"`
	StorageProfile     StorageProfileInfo  `json:"storageProfile"`
	NetworkConnections []NetworkConnection `json:"networkConnections"`
	Href               string              `json:"href"`
}

// VMToolsInfo represents VM tools information
type VMToolsInfo struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// HardwareInfo represents VM hardware configuration
type HardwareInfo struct {
	NumCPUs           int `json:"numCpus"`
	NumCoresPerSocket int `json:"numCoresPerSocket"`
	MemoryMB          int `json:"memoryMB"`
}

// StorageProfileInfo represents storage profile information
type StorageProfileInfo struct {
	Name string `json:"name"`
	Href string `json:"href"`
}

// NetworkConnection represents a VM network connection
type NetworkConnection struct {
	NetworkName string `json:"networkName"`
	IPAddress   string `json:"ipAddress"`
	MACAddress  string `json:"macAddress"`
	Connected   bool   `json:"connected"`
}

// GetVM handles GET /cloudapi/1.0.0/vms/{vm_id}
func (h *VMHandlers) GetVM(c *gin.Context) {
	// Extract user ID from JWT claims
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Authentication required",
		))
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, NewAPIError(
			http.StatusUnauthorized,
			"Unauthorized",
			"Invalid authentication token",
		))
		return
	}

	vmID := c.Param("vm_id")

	// Validate VM URN format
	if !vmURNRegex.MatchString(vmID) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid VM URN format",
		))
		return
	}

	// Validate VM access
	vm, err := h.validateVMAccess(c.Request.Context(), userClaims.UserID, vmID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"VM not found",
			))
		} else {
			c.JSON(http.StatusForbidden, NewAPIError(
				http.StatusForbidden,
				"Forbidden",
				"VM access denied",
			))
		}
		return
	}

	// Convert to response format
	response := h.toVMResponse(*vm)
	c.JSON(http.StatusOK, response)
}

// validateVMAccess validates that a user has access to a VM through vApp's VDC organization membership
func (h *VMHandlers) validateVMAccess(ctx context.Context, userID, vmID string) (*models.VM, error) {
	vm, err := h.vmRepo.GetWithVAppContext(ctx, vmID)
	if err != nil {
		return nil, err
	}

	// Check if user has access to the VDC containing this VM's vApp
	_, err = h.vdcRepo.GetAccessibleVDC(ctx, userID, vm.VApp.VDCID)
	if err != nil {
		return nil, fmt.Errorf("VDC access denied: %w", err)
	}

	return vm, nil
}

// toVMResponse converts a VM model to VCD-compliant response format
func (h *VMHandlers) toVMResponse(vm models.VM) VMResponse {
	// Extract template ID if available
	templateID := ""
	if vm.VApp != nil && vm.VApp.TemplateID != nil {
		templateID = *vm.VApp.TemplateID
	}

	// Default values for VM information
	// In a full implementation, these would be retrieved from OpenShift VirtualMachine resource
	hardware := HardwareInfo{
		NumCPUs:           2,
		NumCoresPerSocket: 1,
		MemoryMB:          4096,
	}

	if vm.CPUCount != nil {
		hardware.NumCPUs = *vm.CPUCount
	}
	if vm.MemoryMB != nil {
		hardware.MemoryMB = *vm.MemoryMB
	}

	guestOS := vm.GuestOS
	if guestOS == "" {
		guestOS = "Ubuntu Linux (64-bit)" // Default
	}

	description := vm.Description
	if description == "" {
		description = fmt.Sprintf("Virtual machine %s", vm.Name)
	}

	return VMResponse{
		ID:          vm.ID,
		Name:        vm.Name,
		Description: description,
		Status:      vm.Status,
		VAppID:      vm.VAppID,
		TemplateID:  templateID,
		CreatedAt:   vm.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   vm.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		GuestOS:     guestOS,
		VMTools: VMToolsInfo{
			Status:  "RUNNING",
			Version: "12.1.5",
		},
		Hardware: hardware,
		StorageProfile: StorageProfileInfo{
			Name: "default-storage-policy",
			Href: "/cloudapi/1.0.0/storageProfiles/default-storage-policy",
		},
		NetworkConnections: []NetworkConnection{
			{
				NetworkName: "default-network",
				IPAddress:   "192.168.1.100", // Would be retrieved from OpenShift
				MACAddress:  "00:50:56:12:34:56",
				Connected:   true,
			},
		},
		Href: fmt.Sprintf("/cloudapi/1.0.0/vms/%s", vm.ID),
	}
}
