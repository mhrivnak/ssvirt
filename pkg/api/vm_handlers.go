package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// VMResponse represents a VM response
type VMResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	VAppID    string `json:"vapp_id"`
	VAppName  string `json:"vapp_name,omitempty"`
	VMName    string `json:"vm_name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	CPUCount  *int   `json:"cpu_count"`
	MemoryMB  *int   `json:"memory_mb"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	VDCName   string `json:"vdc_name,omitempty"`
	OrgName   string `json:"org_name,omitempty"`
}

// VMQueryResponse represents a VM query response
type VMQueryResponse struct {
	VMs   []VMResponse `json:"vms"`
	Total int          `json:"total"`
}

// CreateVMRequest represents a VM creation request
type CreateVMRequest struct {
	Name     string `json:"name" binding:"required"`
	VMName   string `json:"vm_name,omitempty"`
	CPUCount *int   `json:"cpu_count,omitempty"`
	MemoryMB *int   `json:"memory_mb,omitempty"`
}

// UpdateVMRequest represents a VM update request
type UpdateVMRequest struct {
	Name     string `json:"name,omitempty"`
	CPUCount *int   `json:"cpu_count,omitempty"`
	MemoryMB *int   `json:"memory_mb,omitempty"`
	Status   string `json:"status,omitempty"`
}

// vappVMsQueryHandler handles GET /api/vApp/{vapp-id}/vms/query - list VMs in a specific vApp
func (s *Server) vappVMsQueryHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse vApp ID from URL parameter
	vappIDStr := c.Param("vapp-id")
	vappID, err := uuid.Parse(vappIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid vApp ID format"))
		return
	}

	// Parse query parameters
	status := c.Query("status")
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")

	// Get user with their organization roles
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Extract organization IDs from user roles
	orgIDs := make([]uuid.UUID, 0, len(user.UserRoles))
	for _, role := range user.UserRoles {
		orgIDs = append(orgIDs, role.OrganizationID)
	}

	// Check if user has access to the vApp's organization
	vapp, err := s.vappRepo.GetWithAll(vappID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "vApp not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp"))
		}
		return
	}

	hasAccess := false
	if vapp.VDC != nil {
		for _, role := range user.UserRoles {
			if role.OrganizationID == vapp.VDC.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to access this vApp"))
		return
	}

	// vApp ID is already validated from URL parameter

	// Parse pagination
	var limit, offset int
	if limitStr != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitStr); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	if offsetStr != "" {
		if parsedOffset, parseErr := strconv.Atoi(offsetStr); parseErr == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Get VMs from the specific vApp with filters
	vms, total, err := s.vmRepo.GetByVAppIDWithFilters(vappID, status, limit, offset)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve VMs"))
		return
	}

	// Convert to response format
	vmResponses := make([]VMResponse, len(vms))
	for i, vm := range vms {
		vmResponse := VMResponse{
			ID:        vm.ID.String(),
			Name:      vm.Name,
			VAppID:    vm.VAppID.String(),
			VMName:    vm.VMName,
			Namespace: vm.Namespace,
			Status:    vm.Status,
			CPUCount:  vm.CPUCount,
			MemoryMB:  vm.MemoryMB,
			CreatedAt: vm.CreatedAt.Format(time.RFC3339),
			UpdatedAt: vm.UpdatedAt.Format(time.RFC3339),
		}

		// Add vApp name if preloaded
		if vm.VApp != nil {
			vmResponse.VAppName = vm.VApp.Name
			// Add VDC and organization names if preloaded
			if vm.VApp.VDC != nil {
				vmResponse.VDCName = vm.VApp.VDC.Name
				if vm.VApp.VDC.Organization != nil {
					vmResponse.OrgName = vm.VApp.VDC.Organization.Name
				}
			}
		}

		vmResponses[i] = vmResponse
	}

	response := VMQueryResponse{
		VMs:   vmResponses,
		Total: int(total),
	}

	SendSuccess(c, http.StatusOK, response)
}

// vmHandler handles GET /api/vm/{vm-id} - get specific VM with details
func (s *Server) vmHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse VM ID
	vmIDStr := c.Param("vm-id")
	vmID, err := uuid.Parse(vmIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid VM ID format"))
		return
	}

	// Get VM with all related data
	vm, err := s.vmRepo.GetWithVApp(vmID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "VM not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve VM"))
		}
		return
	}

	// Check if user has access to this VM's organization
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Get VDC and organization info for access control
	vapp, err := s.vappRepo.GetWithAll(vm.VAppID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp information"))
		return
	}

	hasAccess := false
	if vapp.VDC != nil {
		for _, role := range user.UserRoles {
			if role.OrganizationID == vapp.VDC.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to access this VM"))
		return
	}

	// Convert to response format
	vmResponse := VMResponse{
		ID:        vm.ID.String(),
		Name:      vm.Name,
		VAppID:    vm.VAppID.String(),
		VMName:    vm.VMName,
		Namespace: vm.Namespace,
		Status:    vm.Status,
		CPUCount:  vm.CPUCount,
		MemoryMB:  vm.MemoryMB,
		CreatedAt: vm.CreatedAt.Format(time.RFC3339),
		UpdatedAt: vm.UpdatedAt.Format(time.RFC3339),
	}

	// Add vApp name if preloaded
	if vm.VApp != nil {
		vmResponse.VAppName = vm.VApp.Name
	}

	// Add VDC and organization names
	if vapp.VDC != nil {
		vmResponse.VDCName = vapp.VDC.Name
		if vapp.VDC.Organization != nil {
			vmResponse.OrgName = vapp.VDC.Organization.Name
		}
	}

	SendSuccess(c, http.StatusOK, vmResponse)
}

// createVMInVAppHandler handles POST /api/vApp/{vapp-id}/vms - create a new VM in a vApp
func (s *Server) createVMInVAppHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse vApp ID from URL parameter
	vappIDStr := c.Param("vapp-id")
	vappID, err := uuid.Parse(vappIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid vApp ID format"))
		return
	}

	// Parse request body
	var req CreateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid request body", err.Error()))
		return
	}

	// Check if user has access to the vApp's organization
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Get vApp with VDC and organization info for access control
	vapp, err := s.vappRepo.GetWithAll(vappID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "vApp not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp"))
		}
		return
	}

	hasAccess := false
	if vapp.VDC != nil {
		for _, role := range user.UserRoles {
			if role.OrganizationID == vapp.VDC.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to create VMs in this vApp"))
		return
	}

	// Set default VM name if not provided
	vmName := req.VMName
	if vmName == "" {
		vmName = req.Name + "-vm"
	}

	// Create VM
	vm := &models.VM{
		Name:      req.Name,
		VAppID:    vappID,
		VMName:    vmName,
		Namespace: vapp.VDC.Organization.Namespace,
		Status:    "UNRESOLVED", // Default status
		CPUCount:  req.CPUCount,
		MemoryMB:  req.MemoryMB,
	}

	if err := s.vmRepo.Create(vm); err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to create VM"))
		return
	}

	// Convert to response format
	vmResponse := VMResponse{
		ID:        vm.ID.String(),
		Name:      vm.Name,
		VAppID:    vm.VAppID.String(),
		VAppName:  vapp.Name,
		VMName:    vm.VMName,
		Namespace: vm.Namespace,
		Status:    vm.Status,
		CPUCount:  vm.CPUCount,
		MemoryMB:  vm.MemoryMB,
		CreatedAt: vm.CreatedAt.Format(time.RFC3339),
		UpdatedAt: vm.UpdatedAt.Format(time.RFC3339),
	}

	if vapp.VDC != nil {
		vmResponse.VDCName = vapp.VDC.Name
		if vapp.VDC.Organization != nil {
			vmResponse.OrgName = vapp.VDC.Organization.Name
		}
	}

	SendSuccess(c, http.StatusCreated, vmResponse)
}

// updateVMHandler handles PUT /api/vm/{vm-id} - update a VM
func (s *Server) updateVMHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse VM ID
	vmIDStr := c.Param("vm-id")
	vmID, err := uuid.Parse(vmIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid VM ID format"))
		return
	}

	// Parse request body
	var req UpdateVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid request body", err.Error()))
		return
	}

	// Get existing VM
	vm, err := s.vmRepo.GetByID(vmID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "VM not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve VM"))
		}
		return
	}

	// Check if user has access to this VM's organization
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Get vApp with VDC and organization info for access control
	vapp, err := s.vappRepo.GetWithAll(vm.VAppID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp information"))
		return
	}

	hasAccess := false
	if vapp.VDC != nil {
		for _, role := range user.UserRoles {
			if role.OrganizationID == vapp.VDC.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to update this VM"))
		return
	}

	// Update VM fields
	if req.Name != "" {
		vm.Name = req.Name
	}
	if req.CPUCount != nil {
		vm.CPUCount = req.CPUCount
	}
	if req.MemoryMB != nil {
		vm.MemoryMB = req.MemoryMB
	}
	if req.Status != "" {
		vm.Status = req.Status
	}

	// Save updates
	if err := s.vmRepo.Update(vm); err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to update VM"))
		return
	}

	// Convert to response format
	vmResponse := VMResponse{
		ID:        vm.ID.String(),
		Name:      vm.Name,
		VAppID:    vm.VAppID.String(),
		VAppName:  vapp.Name,
		VMName:    vm.VMName,
		Namespace: vm.Namespace,
		Status:    vm.Status,
		CPUCount:  vm.CPUCount,
		MemoryMB:  vm.MemoryMB,
		CreatedAt: vm.CreatedAt.Format(time.RFC3339),
		UpdatedAt: vm.UpdatedAt.Format(time.RFC3339),
	}

	if vapp.VDC != nil {
		vmResponse.VDCName = vapp.VDC.Name
		if vapp.VDC.Organization != nil {
			vmResponse.OrgName = vapp.VDC.Organization.Name
		}
	}

	SendSuccess(c, http.StatusOK, vmResponse)
}

// deleteVMHandler handles DELETE /api/vm/{vm-id} - delete a VM
func (s *Server) deleteVMHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse VM ID
	vmIDStr := c.Param("vm-id")
	vmID, err := uuid.Parse(vmIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid VM ID format"))
		return
	}

	// Get existing VM
	vm, err := s.vmRepo.GetByID(vmID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "VM not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve VM"))
		}
		return
	}

	// Check if user has access to this VM's organization
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
		return
	}

	// Get vApp with VDC and organization info for access control
	vapp, err := s.vappRepo.GetWithAll(vm.VAppID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp information"))
		return
	}

	hasAccess := false
	if vapp.VDC != nil {
		for _, role := range user.UserRoles {
			if role.OrganizationID == vapp.VDC.OrganizationID {
				hasAccess = true
				break
			}
		}
	}

	if !hasAccess {
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to delete this VM"))
		return
	}

	// Delete VM
	if err := s.vmRepo.Delete(vmID); err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to delete VM"))
		return
	}

	// Return success response
	response := map[string]interface{}{
		"message": "VM deleted successfully",
		"vm_id":   vmID.String(),
		"deleted": true,
	}

	SendSuccess(c, http.StatusOK, response)
}
