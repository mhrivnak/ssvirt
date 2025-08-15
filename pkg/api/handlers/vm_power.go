package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// VMRepositoryInterface defines the interface for VM repository operations
type VMRepositoryInterface interface {
	GetByID(id string) (*models.VM, error)
}

// PowerManagementHandler handles VM power operations
type PowerManagementHandler struct {
	vmRepo    VMRepositoryInterface
	k8sClient client.Client
	logger    *slog.Logger
}

// NewPowerManagementHandler creates a new power management handler
func NewPowerManagementHandler(vmRepo VMRepositoryInterface, k8sClient client.Client, logger *slog.Logger) *PowerManagementHandler {
	return &PowerManagementHandler{
		vmRepo:    vmRepo,
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// PowerOperationResponse represents the response from power operations
type PowerOperationResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	PowerState string `json:"powerState"`
	Href       string `json:"href"`
}

// PowerOn handles VM power on requests
func (h *PowerManagementHandler) PowerOn(c *gin.Context) {
	ctx := c.Request.Context()
	vmIDParam := c.Param("vm_id")

	// Normalize VM ID parameter (URN, hyphenless, or regular UUID)
	vmID, err := parseVMIDParam(vmIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"error":   "Bad Request",
			"message": "Invalid VM ID format",
		})
		return
	}

	// Defensive check for Kubernetes client
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"error":   "Service Unavailable",
			"message": "Kubernetes client not initialized",
		})
		return
	}

	// Find VM record in database
	vm, err := h.vmRepo.GetByID(vmID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"error":   "Not Found",
				"message": "VM not found",
			})
			return
		}
		h.logger.Error("Failed to find VM", "vmID", vmID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Internal server error",
		})
		return
	}

	// Check if VM is already powered on
	if vm.Status == "POWERED_ON" || vm.Status == "POWERING_ON" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"error":   "Bad Request",
			"message": "VM is already powered on or powering on",
		})
		return
	}

	// Check for conflicting states
	if vm.Status == "DELETING" || vm.Status == "DELETED" {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"error":   "Conflict",
			"message": "VM is in a conflicting state",
		})
		return
	}

	// Get the VirtualMachine resource from Kubernetes
	vmResource := &kubevirtv1.VirtualMachine{}
	vmKey := types.NamespacedName{
		Name:      vm.VMName,
		Namespace: vm.Namespace,
	}

	err = h.k8sClient.Get(ctx, vmKey, vmResource)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"error":   "Not Found",
				"message": "VirtualMachine resource not found in cluster",
			})
			return
		}
		h.logger.Error("Failed to get VirtualMachine resource",
			"vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Failed to access VM resource",
		})
		return
	}

	// Patch the VirtualMachine spec to power on using strategic merge patch
	runStrategy := kubevirtv1.RunStrategyAlways

	// Create patch to update only the runStrategy field
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"runStrategy": runStrategy,
		},
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		h.logger.Error("Failed to marshal patch data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Failed to prepare VM update",
		})
		return
	}

	err = h.k8sClient.Patch(ctx, vmResource, client.RawPatch(types.MergePatchType, patchBytes))
	if err != nil {
		h.logger.Error("Failed to patch VirtualMachine run strategy",
			"vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Failed to power on VM",
		})
		return
	}

	h.logger.Info("VM power on initiated",
		"vmID", vmID, "vmName", vm.VMName, "namespace", vm.Namespace)

	// Return response (status will be updated by VM Status Controller)
	response := PowerOperationResponse{
		ID:         formatVMURN(vmID),
		Name:       vm.Name,
		Status:     "POWERING_ON",
		PowerState: "POWERING_ON",
		Href:       fmt.Sprintf("/cloudapi/1.0.0/vms/%s", formatVMURN(vmID)),
	}

	c.JSON(http.StatusAccepted, response)
}

// PowerOff handles VM power off requests
func (h *PowerManagementHandler) PowerOff(c *gin.Context) {
	ctx := c.Request.Context()
	vmIDParam := c.Param("vm_id")

	// Normalize VM ID parameter (URN, hyphenless, or regular UUID)
	vmID, err := parseVMIDParam(vmIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"error":   "Bad Request",
			"message": "Invalid VM ID format",
		})
		return
	}

	// Defensive check for Kubernetes client
	if h.k8sClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"error":   "Service Unavailable",
			"message": "Kubernetes client not initialized",
		})
		return
	}

	// Find VM record in database
	vm, err := h.vmRepo.GetByID(vmID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"error":   "Not Found",
				"message": "VM not found",
			})
			return
		}
		h.logger.Error("Failed to find VM", "vmID", vmID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Internal server error",
		})
		return
	}

	// Check if VM is already powered off
	if vm.Status == "POWERED_OFF" || vm.Status == "POWERING_OFF" || vm.Status == "STOPPED" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"error":   "Bad Request",
			"message": "VM is already powered off or powering off",
		})
		return
	}

	// Check for conflicting states
	if vm.Status == "DELETING" || vm.Status == "DELETED" {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"error":   "Conflict",
			"message": "VM is in a conflicting state",
		})
		return
	}

	// Get the VirtualMachine resource from Kubernetes
	vmResource := &kubevirtv1.VirtualMachine{}
	vmKey := types.NamespacedName{
		Name:      vm.VMName,
		Namespace: vm.Namespace,
	}

	err = h.k8sClient.Get(ctx, vmKey, vmResource)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"error":   "Not Found",
				"message": "VirtualMachine resource not found in cluster",
			})
			return
		}
		h.logger.Error("Failed to get VirtualMachine resource",
			"vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Failed to access VM resource",
		})
		return
	}

	// Patch the VirtualMachine spec to power off using strategic merge patch
	runStrategy := kubevirtv1.RunStrategyHalted

	// Create patch to update only the runStrategy field
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"runStrategy": runStrategy,
		},
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		h.logger.Error("Failed to marshal patch data", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Failed to prepare VM update",
		})
		return
	}

	err = h.k8sClient.Patch(ctx, vmResource, client.RawPatch(types.MergePatchType, patchBytes))
	if err != nil {
		h.logger.Error("Failed to patch VirtualMachine run strategy",
			"vmName", vm.VMName, "namespace", vm.Namespace, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"error":   "Internal Server Error",
			"message": "Failed to power off VM",
		})
		return
	}

	h.logger.Info("VM power off initiated",
		"vmID", vmID, "vmName", vm.VMName, "namespace", vm.Namespace)

	// Return response (status will be updated by VM Status Controller)
	response := PowerOperationResponse{
		ID:         formatVMURN(vmID),
		Name:       vm.Name,
		Status:     "POWERING_OFF",
		PowerState: "POWERING_OFF",
		Href:       fmt.Sprintf("/cloudapi/1.0.0/vms/%s", formatVMURN(vmID)),
	}

	c.JSON(http.StatusAccepted, response)
}

// parseVMIDParam normalizes VM ID parameter from URN or hyphenless format to canonical UUID
func parseVMIDParam(param string) (string, error) {
	// Handle URN format: urn:vcloud:vm:{uuid}
	if strings.HasPrefix(param, "urn:vcloud:vm:") {
		uuidPart := strings.TrimPrefix(param, "urn:vcloud:vm:")
		_, err := uuid.Parse(uuidPart)
		if err != nil {
			return "", fmt.Errorf("invalid UUID in URN: %w", err)
		}
		return uuidPart, nil
	}

	// Handle hyphenless 32-hex format (32 characters)
	if len(param) == 32 && isHex(param) {
		// Insert hyphens to make it a valid UUID format
		formatted := fmt.Sprintf("%s-%s-%s-%s-%s",
			param[0:8], param[8:12], param[12:16], param[16:20], param[20:32])
		_, err := uuid.Parse(formatted)
		if err != nil {
			return "", fmt.Errorf("invalid hyphenless UUID: %w", err)
		}
		return formatted, nil
	}

	// Handle regular UUID format
	_, err := uuid.Parse(param)
	if err != nil {
		return "", fmt.Errorf("invalid UUID format: %w", err)
	}
	return param, nil
}

// isHex checks if a string contains only hexadecimal characters
func isHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// isValidUUID validates UUID format using parseVMIDParam
func isValidUUID(u string) bool {
	_, err := parseVMIDParam(u)
	return err == nil
}

// formatVMURN formats VM ID as VMware Cloud Director URN
func formatVMURN(vmID string) string {
	return fmt.Sprintf("urn:vcloud:vm:%s", vmID)
}
