package handlers

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhrivnak/ssvirt/pkg/api/types"
	"github.com/mhrivnak/ssvirt/pkg/database/models"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// CatalogItemHandler handles catalog item API endpoints
type CatalogItemHandler struct {
	catalogItemRepo *repositories.CatalogItemRepository
}

// NewCatalogItemHandler creates a new CatalogItemHandler
func NewCatalogItemHandler(catalogItemRepo *repositories.CatalogItemRepository) *CatalogItemHandler {
	return &CatalogItemHandler{
		catalogItemRepo: catalogItemRepo,
	}
}

// ListCatalogItems handles GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems
func (h *CatalogItemHandler) ListCatalogItems(c *gin.Context) {
	catalogID := c.Param("catalogUrn")

	// Validate catalog URN format
	if !strings.HasPrefix(catalogID, models.URNPrefixCatalog) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid catalog ID format",
		))
		return
	}

	// Parse pagination parameters
	page, pageSize := parsePaginationParams(c)

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get catalog items
	catalogItems, err := h.catalogItemRepo.ListByCatalogID(c.Request.Context(), catalogID, pageSize, offset)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog not found",
			))
			return
		}

		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve catalog items",
		))
		return
	}

	// Get total count
	totalCount, err := h.catalogItemRepo.CountByCatalogID(c.Request.Context(), catalogID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog not found",
			))
			return
		}

		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to count catalog items",
		))
		return
	}

	// Calculate pagination info
	pageCount := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	// Create paginated response
	response := types.Page[models.CatalogItem]{
		ResultTotal: totalCount,
		PageCount:   pageCount,
		Page:        page,
		PageSize:    pageSize,
		Values:      catalogItems,
	}

	c.JSON(http.StatusOK, response)
}

// GetCatalogItem handles GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems/{itemId}
func (h *CatalogItemHandler) GetCatalogItem(c *gin.Context) {
	catalogID := c.Param("catalogUrn")
	itemID := c.Param("itemId")

	// Validate catalog URN format
	if !strings.HasPrefix(catalogID, models.URNPrefixCatalog) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid catalog ID format",
		))
		return
	}

	// Validate catalog item URN format
	if !strings.HasPrefix(itemID, models.URNPrefixCatalogItem) {
		c.JSON(http.StatusBadRequest, NewAPIError(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid catalog item ID format",
		))
		return
	}

	// Get catalog item
	catalogItem, err := h.catalogItemRepo.GetByID(c.Request.Context(), catalogID, itemID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) || strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, NewAPIError(
				http.StatusNotFound,
				"Not Found",
				"Catalog item not found",
			))
			return
		}

		c.JSON(http.StatusInternalServerError, NewAPIError(
			http.StatusInternalServerError,
			"Internal Server Error",
			"Failed to retrieve catalog item",
		))
		return
	}

	c.JSON(http.StatusOK, catalogItem)
}

// parsePaginationParams extracts and validates pagination parameters from the request
func parsePaginationParams(c *gin.Context) (page, pageSize int) {
	// Default values
	page = 1
	pageSize = 25

	// Parse page parameter
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Parse pageSize parameter
	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
			// Limit maximum page size
			if pageSize > 100 {
				pageSize = 100
			}
		}
	}

	return page, pageSize
}
