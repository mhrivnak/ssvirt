package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// TemplateService provides access to OpenShift Templates via Kubernetes client
type TemplateService struct {
	client client.Client
	cache  cache.Cache
	mapper *TemplateMapper
}

// Ensure TemplateService implements TemplateServiceInterface
var _ TemplateServiceInterface = (*TemplateService)(nil)

// TemplateMapper handles conversion between OpenShift Templates and CatalogItems
type TemplateMapper struct{}

// NewTemplateService creates a new TemplateService with caching client
func NewTemplateService() (*TemplateService, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	scheme := runtime.NewScheme()
	if err := templatev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add template scheme: %w", err)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	cacheClient, err := cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &TemplateService{
		client: c,
		cache:  cacheClient,
		mapper: &TemplateMapper{},
	}, nil
}

// Start starts the cache
func (s *TemplateService) Start(ctx context.Context) error {
	return s.cache.Start(ctx)
}

// ListCatalogItems returns catalog items for the specified catalog with pagination
func (s *TemplateService) ListCatalogItems(ctx context.Context, catalogID string, limit, offset int) ([]models.CatalogItem, error) {
	templates, err := s.getFilteredTemplates(ctx)
	if err != nil {
		return nil, err
	}

	// Convert templates to catalog items
	var catalogItems []models.CatalogItem
	for _, template := range templates {
		catalogItem := s.mapper.TemplateToCatalogItem(&template, catalogID)
		catalogItems = append(catalogItems, *catalogItem)
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(catalogItems) {
		start = len(catalogItems)
	}
	if end > len(catalogItems) {
		end = len(catalogItems)
	}

	return catalogItems[start:end], nil
}

// CountCatalogItems returns the total count of catalog items for the specified catalog
func (s *TemplateService) CountCatalogItems(ctx context.Context, catalogID string) (int64, error) {
	templates, err := s.getFilteredTemplates(ctx)
	if err != nil {
		return 0, err
	}

	return int64(len(templates)), nil
}

// GetCatalogItem returns a specific catalog item by ID
func (s *TemplateService) GetCatalogItem(ctx context.Context, catalogID, itemID string) (*models.CatalogItem, error) {
	// Extract UUID from catalogitem URN
	if !strings.HasPrefix(itemID, models.URNPrefixCatalogItem) {
		return nil, fmt.Errorf("invalid catalog item URN format")
	}

	templateUID := strings.TrimPrefix(itemID, models.URNPrefixCatalogItem)

	// Get all templates and find the one with matching UID
	templates, err := s.getFilteredTemplates(ctx)
	if err != nil {
		return nil, err
	}

	for _, template := range templates {
		if string(template.UID) == templateUID {
			catalogItem := s.mapper.TemplateToCatalogItem(&template, catalogID)
			return catalogItem, nil
		}
	}

	return nil, fmt.Errorf("catalog item not found")
}

// getFilteredTemplates retrieves templates from openshift namespace with required labels/annotations
func (s *TemplateService) getFilteredTemplates(ctx context.Context) ([]templatev1.Template, error) {
	var templateList templatev1.TemplateList

	// Create label selector for templates with required label
	labelSelector := labels.Set{
		"template.kubevirt.io/version": "", // We just need the label to exist, value doesn't matter
	}.AsSelector()

	err := s.client.List(ctx, &templateList, &client.ListOptions{
		Namespace:     "openshift",
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	// Filter templates that also have the required annotation
	var filteredTemplates []templatev1.Template
	for _, template := range templateList.Items {
		if template.Annotations != nil {
			if _, hasAnnotation := template.Annotations["template.kubevirt.io/containerdisks"]; hasAnnotation {
				filteredTemplates = append(filteredTemplates, template)
			}
		}
	}

	return filteredTemplates, nil
}

// TemplateToCatalogItem converts an OpenShift Template to a CatalogItem
func (m *TemplateMapper) TemplateToCatalogItem(template *templatev1.Template, catalogID string) *models.CatalogItem {
	description := ""
	if template.Annotations != nil {
		if desc, ok := template.Annotations["description"]; ok {
			description = desc
		} else if desc, ok := template.Annotations["template.openshift.io/long-description"]; ok {
			description = desc
		}
	}

	// Check if published based on labels
	isPublished := false
	if template.Labels != nil {
		if published, ok := template.Labels["catalog.ssvirt.io/published"]; ok && published == "true" {
			isPublished = true
		}
	}

	// Extract resource requirements
	numberOfVMs := m.ExtractVMCount(template)
	numberOfCpus, memoryAllocation, storageAllocation := m.ExtractResourceRequirements(template)

	// Estimate size (simplified calculation)
	size := int64(numberOfVMs * 2 * 1024 * 1024 * 1024) // 2GB per VM estimate

	return &models.CatalogItem{
		ID:           models.URNPrefixCatalogItem + string(template.UID),
		Name:         template.Name,
		Description:  description,
		CatalogID:    catalogID,
		IsPublished:  isPublished,
		IsExpired:    false,
		CreationDate: template.CreationTimestamp.Format(time.RFC3339),
		Size:         size,
		Status:       "RESOLVED",
		Entity: models.CatalogItemEntity{
			Name:              template.Name,
			Description:       description,
			Type:              "application/vnd.vmware.vcloud.vAppTemplate+xml",
			NumberOfVMs:       numberOfVMs,
			NumberOfCpus:      numberOfCpus,
			MemoryAllocation:  memoryAllocation,
			StorageAllocation: storageAllocation,
		},
		Owner: models.EntityRef{
			Name: "System",
			ID:   "",
		},
		Catalog: models.EntityRef{
			Name: "Templates", // Default name, could be enhanced to look up actual catalog
			ID:   catalogID,
		},
	}
}

// ExtractVMCount counts the number of VM objects in the template
func (m *TemplateMapper) ExtractVMCount(template *templatev1.Template) int {
	count := 0
	for _, obj := range template.Objects {
		// Look for VirtualMachine or VirtualMachineInstance objects in Raw data
		if obj.Raw != nil {
			// Simple string search in the raw YAML/JSON for kind field
			objStr := string(obj.Raw)
			if strings.Contains(objStr, `"kind":"VirtualMachine"`) ||
				strings.Contains(objStr, `"kind": "VirtualMachine"`) ||
				strings.Contains(objStr, `kind: VirtualMachine`) ||
				strings.Contains(objStr, `"kind":"VirtualMachineInstance"`) ||
				strings.Contains(objStr, `"kind": "VirtualMachineInstance"`) ||
				strings.Contains(objStr, `kind: VirtualMachineInstance`) {
				count++
			}
		}
	}

	// If no VMs found, assume at least 1
	if count == 0 {
		count = 1
	}

	return count
}

// ExtractResourceRequirements extracts CPU, memory, and storage requirements from template parameters
func (m *TemplateMapper) ExtractResourceRequirements(template *templatev1.Template) (cpus int, memory int64, storage int64) {
	cpus = 1                          // Default values
	memory = 1024 * 1024 * 1024       // 1GB
	storage = 10 * 1024 * 1024 * 1024 // 10GB

	// Look through template parameters for resource specifications
	for _, param := range template.Parameters {
		switch strings.ToLower(param.Name) {
		case "cpu", "cpus", "vcpu", "vcpus":
			if val, err := strconv.Atoi(param.Value); err == nil {
				cpus = val
			}
		case "memory", "ram":
			// Try to parse memory value (could be in various formats)
			if val, err := strconv.ParseInt(param.Value, 10, 64); err == nil {
				memory = val
			}
		case "storage", "disk", "disksize":
			// Try to parse storage value
			if val, err := strconv.ParseInt(param.Value, 10, 64); err == nil {
				storage = val
			}
		}
	}

	return cpus, memory, storage
}
