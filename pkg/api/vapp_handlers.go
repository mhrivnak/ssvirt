package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// VAppResponse represents a vApp response
type VAppResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      string           `json:"status"`
	VDCID       string           `json:"vdc_id"`
	VDCName     string           `json:"vdc_name"`
	TemplateID  *string          `json:"template_id"`
	TemplateName *string         `json:"template_name"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	VMs         []VAppVMResponse `json:"vms,omitempty"`
}

// VAppVMResponse represents a VM within a vApp response
type VAppVMResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	VMName    string `json:"vm_name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	CPUCount  *int   `json:"cpu_count"`
	MemoryMB  *int   `json:"memory_mb"`
}

// VAppQueryResponse represents a vApp query response
type VAppQueryResponse struct {
	VApps []VAppResponse `json:"vapps"`
	Total int            `json:"total"`
}

// vAppsQueryHandler handles GET /api/vApps/query - list vApps accessible to user
func (s *Server) vAppsQueryHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

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

	// Return empty list if user has no organization access
	if len(orgIDs) == 0 {
		response := VAppQueryResponse{
			VApps: []VAppResponse{},
			Total: 0,
		}
		SendSuccess(c, http.StatusOK, response)
		return
	}

	// Get vApps accessible to user (based on organization membership)
	vapps, err := s.vappRepo.GetByOrganizationIDs(orgIDs)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApps"))
		return
	}

	// Convert to response format
	vappResponses := make([]VAppResponse, len(vapps))
	for i, vapp := range vapps {
		vappResponse := VAppResponse{
			ID:          vapp.ID.String(),
			Name:        vapp.Name,
			Description: vapp.Description,
			Status:      vapp.Status,
			VDCID:       vapp.VDCID.String(),
			CreatedAt:   vapp.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   vapp.UpdatedAt.Format(time.RFC3339),
		}

		// Add VDC name if preloaded
		if vapp.VDC != nil {
			vappResponse.VDCName = vapp.VDC.Name
		}

		// Add template info if available
		if vapp.TemplateID != nil {
			templateIDStr := vapp.TemplateID.String()
			vappResponse.TemplateID = &templateIDStr
			if vapp.Template != nil {
				vappResponse.TemplateName = &vapp.Template.Name
			}
		}

		// Add VM information if preloaded
		if len(vapp.VMs) > 0 {
			vmResponses := make([]VAppVMResponse, len(vapp.VMs))
			for j, vm := range vapp.VMs {
				vmResponses[j] = VAppVMResponse{
					ID:        vm.ID.String(),
					Name:      vm.Name,
					VMName:    vm.VMName,
					Namespace: vm.Namespace,
					Status:    vm.Status,
					CPUCount:  vm.CPUCount,
					MemoryMB:  vm.MemoryMB,
				}
			}
			vappResponse.VMs = vmResponses
		}

		vappResponses[i] = vappResponse
	}

	response := VAppQueryResponse{
		VApps: vappResponses,
		Total: len(vappResponses),
	}

	SendSuccess(c, http.StatusOK, response)
}

// vAppHandler handles GET /api/vApp/{vapp-id} - get specific vApp with details
func (s *Server) vAppHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse vApp ID
	vappIDStr := c.Param("vapp-id")
	vappID, err := uuid.Parse(vappIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid vApp ID format"))
		return
	}

	// Get vApp with all related data
	vapp, err := s.vappRepo.GetWithAll(vappID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "vApp not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp"))
		}
		return
	}

	// Check if user has access to this vApp's organization
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
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

	// Convert to response format
	vappResponse := VAppResponse{
		ID:          vapp.ID.String(),
		Name:        vapp.Name,
		Description: vapp.Description,
		Status:      vapp.Status,
		VDCID:       vapp.VDCID.String(),
		CreatedAt:   vapp.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   vapp.UpdatedAt.Format(time.RFC3339),
	}

	// Add VDC name if preloaded
	if vapp.VDC != nil {
		vappResponse.VDCName = vapp.VDC.Name
	}

	// Add template info if available
	if vapp.TemplateID != nil {
		templateIDStr := vapp.TemplateID.String()
		vappResponse.TemplateID = &templateIDStr
		if vapp.Template != nil {
			vappResponse.TemplateName = &vapp.Template.Name
		}
	}

	// Add VM information
	if len(vapp.VMs) > 0 {
		vmResponses := make([]VAppVMResponse, len(vapp.VMs))
		for i, vm := range vapp.VMs {
			vmResponses[i] = VAppVMResponse{
				ID:        vm.ID.String(),
				Name:      vm.Name,
				VMName:    vm.VMName,
				Namespace: vm.Namespace,
				Status:    vm.Status,
				CPUCount:  vm.CPUCount,
				MemoryMB:  vm.MemoryMB,
			}
		}
		vappResponse.VMs = vmResponses
	}

	SendSuccess(c, http.StatusOK, vappResponse)
}

// deleteVAppHandler handles DELETE /api/vApp/{vapp-id} - delete specific vApp
func (s *Server) deleteVAppHandler(c *gin.Context) {
	// Get user claims from JWT middleware
	claims, exists := auth.GetClaims(c)
	if !exists {
		SendError(c, NewAPIError(http.StatusUnauthorized, "Unauthorized", "Invalid or missing authentication token"))
		return
	}

	// Parse vApp ID
	vappIDStr := c.Param("vapp-id")
	vappID, err := uuid.Parse(vappIDStr)
	if err != nil {
		SendError(c, NewAPIError(http.StatusBadRequest, "Bad Request", "Invalid vApp ID format"))
		return
	}

	// Get vApp with VDC info for access control
	vapp, err := s.vappRepo.GetWithAll(vappID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			SendError(c, NewAPIError(http.StatusNotFound, "Not Found", "vApp not found"))
		} else {
			SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve vApp"))
		}
		return
	}

	// Check if user has access to this vApp's organization
	user, err := s.userRepo.GetWithRoles(claims.UserID)
	if err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve user information"))
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
		SendError(c, NewAPIError(http.StatusForbidden, "Forbidden", "You do not have permission to delete this vApp"))
		return
	}

	// Use transaction for atomic deletion of vApp and its VMs
	tx := s.db.DB.Begin()
	if tx.Error != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to start transaction"))
		return
	}
	defer func() {
		if r := recover(); r != nil {
			if tx != nil {
				tx.Rollback()
			}
			// Log the panic for debugging
			log.Printf("panic during vApp deletion: %v", r)
			panic(r)
		}
	}()

	// Delete associated VMs first
	if err := tx.Where("v_app_id = ?", vappID).Delete(&models.VM{}).Error; err != nil {
		tx.Rollback()
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to delete associated VMs"))
		return
	}

	// Delete the vApp
	if err := tx.Delete(&models.VApp{}, vappID).Error; err != nil {
		tx.Rollback()
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to delete vApp"))
		return
	}

	if err := tx.Commit().Error; err != nil {
		SendError(c, NewAPIError(http.StatusInternalServerError, "Internal Server Error", "Failed to commit deletion transaction"))
		return
	}

	// Return success response
	response := map[string]interface{}{
		"message": "vApp deleted successfully",
		"vapp_id": vappID.String(),
		"deleted": true,
	}

	SendSuccess(c, http.StatusOK, response)
}