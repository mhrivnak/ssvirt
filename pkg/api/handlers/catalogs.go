package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
	"github.com/mhrivnak/ssvirt/pkg/services"
)

type CatalogHandlers struct {
	catalogRepo *repositories.CatalogRepository
	orgRepo     *repositories.OrganizationRepository
	k8sService  services.KubernetesService
}

func NewCatalogHandlers(catalogRepo *repositories.CatalogRepository, orgRepo *repositories.OrganizationRepository, k8sService services.KubernetesService) *CatalogHandlers {
	return &CatalogHandlers{
		catalogRepo: catalogRepo,
		orgRepo:     orgRepo,
		k8sService:  k8sService,
	}
}

// CatalogCreateRequest represents the request body for creating a catalog
type CatalogCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	OrgID       string `json:"orgId" binding:"required"` // Organization URN
	IsPublished bool   `json:"isPublished"`
}

// CatalogResponse represents the VCD-compliant catalog response
type CatalogResponse struct {
	ID                       string                    `json:"id"`
	Name                     string                    `json:"name"`
	Description              string                    `json:"description"`
	Org                      models.OrgReference       `json:"org"`
	IsPublished              bool                      `json:"isPublished"`
	IsSubscribed             bool                      `json:"isSubscribed"`
	CreationDate             string                    `json:"creationDate"`
	NumberOfVAppTemplates    int                       `json:"numberOfVAppTemplates"`
	NumberOfMedia            int                       `json:"numberOfMedia"`
	CatalogStorageProfiles   []interface{}             `json:"catalogStorageProfiles"`
	PublishConfig            models.PublishConfig      `json:"publishConfig"`
	SubscriptionConfig       models.SubscriptionConfig `json:"subscriptionConfig"`
	DistributedCatalogConfig interface{}               `json:"distributedCatalogConfig"`
	Owner                    models.OwnerReference     `json:"owner"`
	IsLocal                  bool                      `json:"isLocal"`
	Version                  int                       `json:"version"`
}

// ListCatalogs handles GET /cloudapi/1.0.0/catalogs
func (h *CatalogHandlers) ListCatalogs(c *gin.Context) {
	// Parse pagination parameters
	page := 1
	pageSize := 25

	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	if sizeParam := c.Query("pageSize"); sizeParam != "" {
		if s, err := strconv.Atoi(sizeParam); err == nil && s > 0 && s <= 128 {
			pageSize = s
		}
	}

	offset := (page - 1) * pageSize

	// Get catalogs with pagination
	catalogs, err := h.catalogRepo.ListWithPagination(pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve catalogs",
			err.Error(),
		))
		return
	}

	// Get total count
	totalCount, err := h.catalogRepo.CountAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to count catalogs",
			err.Error(),
		))
		return
	}

	// Convert to response format
	catalogResponses := make([]CatalogResponse, len(catalogs))
	for i, catalog := range catalogs {
		catalogResponse := h.toCatalogResponse(catalog)

		// Enrich with OpenShift template count if k8s service is available
		if h.k8sService != nil {
			templates, err := h.k8sService.ListAvailableTemplates(c.Request.Context())
			if err == nil {
				catalogResponse.NumberOfVAppTemplates = len(templates)
			}
		}

		catalogResponses[i] = catalogResponse
	}

	// Build paginated response
	response := types.NewPage(catalogResponses, page, pageSize, totalCount)

	c.JSON(http.StatusOK, response)
}

// GetCatalog handles GET /cloudapi/1.0.0/catalogs/{catalogUrn}
func (h *CatalogHandlers) GetCatalog(c *gin.Context) {
	catalogURN := c.Param("catalogUrn")

	// Validate catalog URN format
	if !strings.HasPrefix(catalogURN, models.URNPrefixCatalog) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid catalog URN format",
			"Catalog ID must be a valid URN with prefix 'urn:vcloud:catalog:'",
		))
		return
	}

	// Get catalog
	catalog, err := h.catalogRepo.GetByURN(catalogURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog not found",
				fmt.Sprintf("Catalog with ID '%s' does not exist", catalogURN),
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve catalog",
			err.Error(),
		))
		return
	}

	c.JSON(http.StatusOK, h.toCatalogResponse(*catalog))
}

// CreateCatalog handles POST /cloudapi/1.0.0/catalogs
func (h *CatalogHandlers) CreateCatalog(c *gin.Context) {
	var req CatalogCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid request body",
			err.Error(),
		))
		return
	}

	// Validate organization URN format
	if !strings.HasPrefix(req.OrgID, models.URNPrefixOrg) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid organization URN format",
			"Organization ID must be a valid URN with prefix 'urn:vcloud:org:'",
		))
		return
	}

	// Verify organization exists
	_, err := h.orgRepo.GetByID(req.OrgID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Organization not found",
				fmt.Sprintf("Organization with ID '%s' does not exist", req.OrgID),
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to query organization",
			err.Error(),
		))
		return
	}

	// Create catalog model with defaults
	catalog := &models.Catalog{
		Name:           req.Name,
		Description:    req.Description,
		OrganizationID: req.OrgID,
		IsPublished:    req.IsPublished,
		IsSubscribed:   false, // Default
		IsLocal:        true,  // Default
		Version:        1,     // Default
		OwnerID:        "",    // Default empty for now
	}

	// Create catalog
	if err := h.catalogRepo.Create(catalog); err != nil {
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to create catalog",
			err.Error(),
		))
		return
	}

	c.JSON(http.StatusCreated, h.toCatalogResponse(*catalog))
}

// DeleteCatalog handles DELETE /cloudapi/1.0.0/catalogs/{catalogUrn}
func (h *CatalogHandlers) DeleteCatalog(c *gin.Context) {
	catalogURN := c.Param("catalogUrn")

	// Validate catalog URN format
	if !strings.HasPrefix(catalogURN, models.URNPrefixCatalog) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid catalog URN format",
			"Catalog ID must be a valid URN with prefix 'urn:vcloud:catalog:'",
		))
		return
	}

	// Verify catalog exists
	_, err := h.catalogRepo.GetByURN(catalogURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog not found",
				fmt.Sprintf("Catalog with ID '%s' does not exist", catalogURN),
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve catalog",
			err.Error(),
		))
		return
	}

	// Delete catalog with validation (checks for dependent templates)
	if err := h.catalogRepo.DeleteWithValidation(catalogURN); err != nil {
		if errors.Is(err, repositories.ErrCatalogHasDependencies) {
			c.JSON(http.StatusConflict, NewAPIError(
				http.StatusConflict,
				"Conflict",
				"Cannot delete catalog with dependent resources",
				"Catalog contains vApp templates that must be deleted first",
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to delete catalog",
			err.Error(),
		))
		return
	}

	c.Status(http.StatusNoContent)
}

// ListCatalogItemsFromTemplates handles GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems
// This enhanced version queries OpenShift templates directly
func (h *CatalogHandlers) ListCatalogItemsFromTemplates(c *gin.Context) {
	catalogURN := c.Param("catalogUrn")

	// Validate catalog URN format
	if !strings.HasPrefix(catalogURN, models.URNPrefixCatalog) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid catalog URN format",
			"Catalog ID must be a valid URN with prefix 'urn:vcloud:catalog:'",
		))
		return
	}

	// Verify catalog exists
	_, err := h.catalogRepo.GetByURN(catalogURN)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog not found",
				fmt.Sprintf("Catalog with ID '%s' does not exist", catalogURN),
			))
			return
		}
		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve catalog",
			err.Error(),
		))
		return
	}

	// Query OpenShift templates if k8s service is available
	if h.k8sService != nil {
		templates, err := h.k8sService.ListAvailableTemplates(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, NewAPIError(
				http.StatusInternalServerError,
				"Internal Server Error",
				"Failed to query templates",
				err.Error(),
			))
			return
		}

		// Convert templates to catalog item responses
		catalogItems := make([]interface{}, len(templates))
		for i, template := range templates {
			catalogItems[i] = map[string]interface{}{
				"id":          fmt.Sprintf("urn:vcloud:catalogitem:%s", template.Name),
				"name":        template.Name,
				"description": template.Description,
				"catalogId":   catalogURN,
				"entityType":  "vappTemplate",
				"status":      "RESOLVED",
				"href":        fmt.Sprintf("/cloudapi/1.0.0/catalogs/%s/catalogItems/urn:vcloud:catalogitem:%s", catalogURN, template.Name),
			}
		}

		// Return paginated response
		response := types.NewPage(catalogItems, 1, len(catalogItems), int64(len(catalogItems)))
		c.JSON(http.StatusOK, response)
		return
	}

	// Fallback to empty list if no k8s service
	response := types.NewPage([]interface{}{}, 1, 0, 0)
	c.JSON(http.StatusOK, response)
}

// toCatalogResponse converts a catalog model to VCD-compliant response format
func (h *CatalogHandlers) toCatalogResponse(catalog models.Catalog) CatalogResponse {
	return CatalogResponse{
		ID:                       catalog.ID,
		Name:                     catalog.Name,
		Description:              catalog.Description,
		Org:                      catalog.Org(),
		IsPublished:              catalog.IsPublished,
		IsSubscribed:             catalog.IsSubscribed,
		CreationDate:             catalog.CreationDate(),
		NumberOfVAppTemplates:    catalog.NumberOfVAppTemplates(),
		NumberOfMedia:            catalog.NumberOfMedia(),
		CatalogStorageProfiles:   catalog.CatalogStorageProfiles(),
		PublishConfig:            catalog.PublishConfigObj(),
		SubscriptionConfig:       catalog.SubscriptionConfigObj(),
		DistributedCatalogConfig: catalog.DistributedCatalogConfig(),
		Owner:                    catalog.Owner(),
		IsLocal:                  catalog.IsLocal,
		Version:                  catalog.Version,
	}
}
