package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	templatev1 "github.com/openshift/api/template/v1"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// KubernetesService provides Kubernetes operations for SSVirt
type KubernetesService interface {
	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	HealthCheck(ctx context.Context) error

	// Namespace management for VDCs
	CreateNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error
	UpdateNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error
	DeleteNamespaceForVDC(ctx context.Context, vdc *models.VDC) error
	EnsureNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error

	// Template discovery
	ListAvailableTemplates(ctx context.Context) ([]*TemplateInfo, error)
	GetTemplate(ctx context.Context, name string) (*TemplateInfo, error)

	// Template instantiation
	CreateTemplateInstance(ctx context.Context, req *TemplateInstanceRequest) (*TemplateInstanceResult, error)
	GetTemplateInstance(ctx context.Context, namespace, name string) (*TemplateInstanceStatus, error)
	DeleteTemplateInstance(ctx context.Context, namespace, name string) error

	// Resource management
	EnsureNamespaceResources(ctx context.Context, namespace string, vdc *models.VDC) error
}

// TemplateInfo represents an OpenShift template available for instantiation
type TemplateInfo struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	IconClass   string            `json:"iconClass,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Parameters  []TemplateParam   `json:"parameters"`
	Objects     []TemplateObject  `json:"objects"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TemplateParam represents a template parameter
type TemplateParam struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	Generate    string `json:"generate,omitempty"`
	From        string `json:"from,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// TemplateObject represents an object in a template
type TemplateObject struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// TemplateInstanceRequest represents a request to instantiate a template
type TemplateInstanceRequest struct {
	TemplateName string                  `json:"templateName"`
	Namespace    string                  `json:"namespace"`
	Name         string                  `json:"name"`
	Parameters   []TemplateInstanceParam `json:"parameters,omitempty"`
	Labels       map[string]string       `json:"labels,omitempty"`
}

// TemplateInstanceParam represents a parameter for template instantiation
type TemplateInstanceParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TemplateInstanceResult represents the result of template instantiation
type TemplateInstanceResult struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Status    TemplateInstanceStatus `json:"status"`
}

// TemplateInstanceStatus represents the status of a template instance
type TemplateInstanceStatus struct {
	Phase      string                 `json:"phase"`
	Message    string                 `json:"message,omitempty"`
	Objects    []TemplateInstanceObj  `json:"objects,omitempty"`
	Conditions []TemplateInstanceCond `json:"conditions,omitempty"`
}

// TemplateInstanceObj represents an object created by template instantiation
type TemplateInstanceObj struct {
	Ref corev1.ObjectReference `json:"ref"`
}

// TemplateInstanceCond represents a condition of template instantiation
type TemplateInstanceCond struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// kubernetesService implements KubernetesService
type kubernetesService struct {
	client       client.Client
	cache        cache.Cache
	scheme       *runtime.Scheme
	directClient client.Client // For write operations
	started      bool

	// Configuration
	templateNamespace string
	cacheResync       time.Duration
}

// NewKubernetesService creates a new Kubernetes service
func NewKubernetesService(templateNamespace string) (KubernetesService, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	scheme := runtime.NewScheme()

	// Add required schemes
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add core/v1 to scheme: %w", err)
	}

	if err := templatev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add template/v1 to scheme: %w", err)
	}

	if err := networkingv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add networking/v1 to scheme: %w", err)
	}

	// Create cache for read operations
	cache, err := cache.New(cfg, cache.Options{
		Scheme:     scheme,
		SyncPeriod: &[]time.Duration{10 * time.Minute}[0],
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create direct client for write operations
	directClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create direct client: %w", err)
	}

	// Create cached client for read operations
	cachedClient, err := client.New(cfg, client.Options{
		Scheme: scheme,
		Cache: &client.CacheOptions{
			Reader: cache,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cached client: %w", err)
	}

	return &kubernetesService{
		client:            cachedClient,
		cache:             cache,
		scheme:            scheme,
		directClient:      directClient,
		templateNamespace: templateNamespace,
		cacheResync:       10 * time.Minute,
	}, nil
}

// Start initializes the Kubernetes service and starts the cache
func (k *kubernetesService) Start(ctx context.Context) error {
	if k.started {
		return nil
	}

	// Start cache in background
	go func() {
		if err := k.cache.Start(ctx); err != nil {
			// Log error but don't fail startup
			// Service will fall back to direct API calls
			fmt.Printf("Cache failed to start: %v\n", err)
		}
	}()

	// Wait for cache sync with timeout
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if !k.cache.WaitForCacheSync(syncCtx) {
		// Cache didn't sync but service can still work with direct calls
		fmt.Println("Warning: Cache did not sync, using direct API calls")
	}

	k.started = true
	return nil
}

// Stop gracefully stops the Kubernetes service
func (k *kubernetesService) Stop(ctx context.Context) error {
	k.started = false
	return nil
}

// HealthCheck verifies connectivity to Kubernetes cluster
func (k *kubernetesService) HealthCheck(ctx context.Context) error {
	// Test connectivity with a simple operation
	_, err := k.directClient.RESTMapper().RESTMappings(corev1.SchemeGroupVersion.WithKind("Namespace").GroupKind())
	return err
}

// CreateNamespaceForVDC creates a Kubernetes namespace for a VDC
func (k *kubernetesService) CreateNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error {
	if vdc.Namespace == "" {
		return fmt.Errorf("VDC namespace name is empty")
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: vdc.Namespace,
			Labels: map[string]string{
				"ssvirt.io/organization":       org.Name,
				"ssvirt.io/organization-id":    org.ID,
				"ssvirt.io/vdc":                vdc.Name,
				"ssvirt.io/vdc-id":             vdc.ID,
				"app.kubernetes.io/managed-by": "ssvirt",
				"app.kubernetes.io/component":  "vdc",
			},
			Annotations: map[string]string{
				"ssvirt.io/organization-display-name": org.DisplayName,
				"ssvirt.io/organization-description":  org.Description,
				"ssvirt.io/vdc-description":           vdc.Description,
				"ssvirt.io/created-by":                "ssvirt-api-server",
			},
		},
	}

	if err := k.directClient.Create(ctx, namespace); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", vdc.Namespace, err)
	}

	// Create resource quota and network policies
	if err := k.EnsureNamespaceResources(ctx, vdc.Namespace, vdc); err != nil {
		// Try to cleanup namespace
		_ = k.directClient.Delete(ctx, namespace)
		return fmt.Errorf("failed to create namespace resources: %w", err)
	}

	return nil
}

// UpdateNamespaceForVDC updates an existing namespace for a VDC
func (k *kubernetesService) UpdateNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error {
	if vdc.Namespace == "" {
		return fmt.Errorf("VDC namespace name is empty")
	}

	namespace := &corev1.Namespace{}
	err := k.directClient.Get(ctx, client.ObjectKey{Name: vdc.Namespace}, namespace)
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", vdc.Namespace, err)
	}

	// Update labels and annotations
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	namespace.Labels["ssvirt.io/organization"] = org.Name
	namespace.Labels["ssvirt.io/organization-id"] = org.ID
	namespace.Labels["ssvirt.io/vdc"] = vdc.Name
	namespace.Labels["ssvirt.io/vdc-id"] = vdc.ID
	namespace.Labels["app.kubernetes.io/managed-by"] = "ssvirt"
	namespace.Labels["app.kubernetes.io/component"] = "vdc"

	namespace.Annotations["ssvirt.io/organization-display-name"] = org.DisplayName
	namespace.Annotations["ssvirt.io/organization-description"] = org.Description
	namespace.Annotations["ssvirt.io/vdc-description"] = vdc.Description

	if err := k.directClient.Update(ctx, namespace); err != nil {
		return fmt.Errorf("failed to update namespace %s: %w", vdc.Namespace, err)
	}

	return k.EnsureNamespaceResources(ctx, vdc.Namespace, vdc)
}

// DeleteNamespaceForVDC deletes the namespace for a VDC
func (k *kubernetesService) DeleteNamespaceForVDC(ctx context.Context, vdc *models.VDC) error {
	if vdc.Namespace == "" {
		return nil // Nothing to delete
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: vdc.Namespace,
		},
	}

	err := k.directClient.Delete(ctx, namespace)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete namespace %s: %w", vdc.Namespace, err)
	}

	return nil
}

// EnsureNamespaceForVDC ensures the namespace exists for a VDC
func (k *kubernetesService) EnsureNamespaceForVDC(ctx context.Context, vdc *models.VDC, org *models.Organization) error {
	if vdc.Namespace == "" {
		return fmt.Errorf("VDC namespace name is empty")
	}

	namespace := &corev1.Namespace{}
	err := k.directClient.Get(ctx, client.ObjectKey{Name: vdc.Namespace}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return k.CreateNamespaceForVDC(ctx, vdc, org)
		}
		return fmt.Errorf("failed to check namespace %s: %w", vdc.Namespace, err)
	}

	return k.UpdateNamespaceForVDC(ctx, vdc, org)
}

// EnsureNamespaceResources creates resource quota and network policies for VDC namespace
func (k *kubernetesService) EnsureNamespaceResources(ctx context.Context, namespace string, vdc *models.VDC) error {
	// Create resource quota
	if err := k.createResourceQuota(ctx, namespace, vdc); err != nil {
		return fmt.Errorf("failed to create resource quota: %w", err)
	}

	// Create network policies
	if err := k.createNetworkPolicies(ctx, namespace, vdc); err != nil {
		return fmt.Errorf("failed to create network policies: %w", err)
	}

	return nil
}

func (k *kubernetesService) createResourceQuota(ctx context.Context, namespace string, vdc *models.VDC) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-quota",
			Namespace: namespace,
			Labels: map[string]string{
				"ssvirt.io/vdc-id":             vdc.ID,
				"app.kubernetes.io/managed-by": "ssvirt",
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				// Set reasonable defaults
				corev1.ResourcePods:                   resource.MustParse("50"),
				corev1.ResourcePersistentVolumeClaims: resource.MustParse("20"),
				corev1.ResourceServices:               resource.MustParse("10"),
				corev1.ResourceSecrets:                resource.MustParse("50"),
				corev1.ResourceConfigMaps:             resource.MustParse("50"),
			},
		},
	}

	// Add VDC-specific limits if configured
	if vdc.CPULimit > 0 {
		quota.Spec.Hard[corev1.ResourceRequestsCPU] = resource.MustParse(fmt.Sprintf("%d", vdc.CPULimit))
		quota.Spec.Hard[corev1.ResourceLimitsCPU] = resource.MustParse(fmt.Sprintf("%d", vdc.CPULimit))
	}

	if vdc.MemoryLimit > 0 {
		memoryLimit := fmt.Sprintf("%dMi", vdc.MemoryLimit)
		quota.Spec.Hard[corev1.ResourceRequestsMemory] = resource.MustParse(memoryLimit)
		quota.Spec.Hard[corev1.ResourceLimitsMemory] = resource.MustParse(memoryLimit)
	}

	// Check if quota already exists
	existingQuota := &corev1.ResourceQuota{}
	err := k.directClient.Get(ctx, client.ObjectKey{Name: "vdc-quota", Namespace: namespace}, existingQuota)
	if err != nil {
		if errors.IsNotFound(err) {
			return k.directClient.Create(ctx, quota)
		}
		return fmt.Errorf("failed to check existing quota: %w", err)
	}

	// Update existing quota
	existingQuota.Spec = quota.Spec
	existingQuota.Labels = quota.Labels
	return k.directClient.Update(ctx, existingQuota)
}

func (k *kubernetesService) createNetworkPolicies(ctx context.Context, namespace string, vdc *models.VDC) error {
	// Create a network policy that provides basic VDC isolation
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vdc-isolation",
			Namespace: namespace,
			Labels: map[string]string{
				"ssvirt.io/vdc-id":             vdc.ID,
				"app.kubernetes.io/managed-by": "ssvirt",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			PodSelector: metav1.LabelSelector{}, // Apply to all pods in namespace
			// Allow DNS resolution
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"name": "openshift-dns",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolTCP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
					},
				},
			},
		},
	}

	// Check if policy already exists
	existingPolicy := &networkingv1.NetworkPolicy{}
	err := k.directClient.Get(ctx, client.ObjectKey{Name: "vdc-isolation", Namespace: namespace}, existingPolicy)
	if err != nil {
		if errors.IsNotFound(err) {
			return k.directClient.Create(ctx, policy)
		}
		return fmt.Errorf("failed to check existing network policy: %w", err)
	}

	// Update existing policy
	existingPolicy.Spec = policy.Spec
	existingPolicy.Labels = policy.Labels
	return k.directClient.Update(ctx, existingPolicy)
}

// ListAvailableTemplates retrieves templates from the configured namespace
func (k *kubernetesService) ListAvailableTemplates(ctx context.Context) ([]*TemplateInfo, error) {
	templateList := &templatev1.TemplateList{}

	err := k.client.List(ctx, templateList, client.InNamespace(k.templateNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list templates in namespace %s: %w", k.templateNamespace, err)
	}

	templates := make([]*TemplateInfo, 0, len(templateList.Items))

	for _, tmpl := range templateList.Items {
		templateInfo := k.convertTemplate(&tmpl)
		templates = append(templates, templateInfo)
	}

	return templates, nil
}

// GetTemplate retrieves a specific template by name
func (k *kubernetesService) GetTemplate(ctx context.Context, name string) (*TemplateInfo, error) {
	template := &templatev1.Template{}

	err := k.client.Get(ctx, client.ObjectKey{
		Namespace: k.templateNamespace,
		Name:      name,
	}, template)

	if err != nil {
		return nil, fmt.Errorf("failed to get template %s: %w", name, err)
	}

	return k.convertTemplate(template), nil
}

func (k *kubernetesService) convertTemplate(tmpl *templatev1.Template) *TemplateInfo {
	info := &TemplateInfo{
		Name:        tmpl.Name,
		DisplayName: tmpl.Annotations["openshift.io/display-name"],
		Description: tmpl.Annotations["description"],
		IconClass:   tmpl.Annotations["iconClass"],
		Labels:      tmpl.Labels,
		Annotations: tmpl.Annotations,
	}

	// Convert parameters
	info.Parameters = make([]TemplateParam, len(tmpl.Parameters))
	for i, param := range tmpl.Parameters {
		info.Parameters[i] = TemplateParam{
			Name:        param.Name,
			DisplayName: param.DisplayName,
			Description: param.Description,
			Value:       param.Value,
			Generate:    param.Generate,
			From:        param.From,
			Required:    param.Required,
		}
	}

	// Convert objects (summary only)
	info.Objects = make([]TemplateObject, len(tmpl.Objects))
	for i, obj := range tmpl.Objects {
		// Extract kind and apiVersion from Object metadata
		kind := ""
		apiVersion := ""
		if obj.Object != nil {
			// Use reflection to safely extract fields since we don't know the exact type
			gvk := obj.Object.GetObjectKind().GroupVersionKind()
			kind = gvk.Kind
			apiVersion = gvk.GroupVersion().String()
		}
		info.Objects[i] = TemplateObject{
			Kind:       kind,
			APIVersion: apiVersion,
		}
	}

	// Extract tags from annotations
	if tags := tmpl.Annotations["tags"]; tags != "" {
		info.Tags = strings.Split(tags, ",")
		for i := range info.Tags {
			info.Tags[i] = strings.TrimSpace(info.Tags[i])
		}
	}

	return info
}

// CreateTemplateInstance creates a new template instance
func (k *kubernetesService) CreateTemplateInstance(ctx context.Context, req *TemplateInstanceRequest) (*TemplateInstanceResult, error) {
	// Create secret with parameters
	if err := k.createParameterSecret(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create parameter secret: %w", err)
	}

	// Create TemplateInstance resource
	templateInstance := &templatev1.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ssvirt",
				"ssvirt.io/template-name":      req.TemplateName,
			},
		},
		Spec: templatev1.TemplateInstanceSpec{
			Template: templatev1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      req.TemplateName,
					Namespace: k.templateNamespace,
				},
			},
			Secret: &corev1.LocalObjectReference{
				Name: req.Name + "-params",
			},
		},
	}

	// Add custom labels
	for key, value := range req.Labels {
		templateInstance.Labels[key] = value
	}

	// Create the template instance
	if err := k.directClient.Create(ctx, templateInstance); err != nil {
		return nil, fmt.Errorf("failed to create template instance: %w", err)
	}

	return &TemplateInstanceResult{
		Name:      templateInstance.Name,
		Namespace: templateInstance.Namespace,
		Status: TemplateInstanceStatus{
			Phase: "Creating",
		},
	}, nil
}

func (k *kubernetesService) createParameterSecret(ctx context.Context, req *TemplateInstanceRequest) error {
	data := make(map[string]string)
	for _, param := range req.Parameters {
		data[param.Name] = param.Value
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name + "-params",
			Namespace: req.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ssvirt",
				"ssvirt.io/template-instance":  req.Name,
			},
		},
		StringData: data,
	}

	return k.directClient.Create(ctx, secret)
}

// GetTemplateInstance retrieves the status of a template instance
func (k *kubernetesService) GetTemplateInstance(ctx context.Context, namespace, name string) (*TemplateInstanceStatus, error) {
	templateInstance := &templatev1.TemplateInstance{}
	err := k.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, templateInstance)

	if err != nil {
		return nil, fmt.Errorf("failed to get template instance %s/%s: %w", namespace, name, err)
	}

	status := &TemplateInstanceStatus{
		// Note: OpenShift TemplateInstanceStatus doesn't have Phase/Message fields
		// We'll use Ready condition instead
		Phase:   "Unknown",
		Message: "Template instance created",
	}

	// Check conditions for status
	for _, condition := range templateInstance.Status.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == "True" {
				status.Phase = "Ready"
			} else {
				status.Phase = "Pending"
			}
			status.Message = condition.Message
			break
		}
	}

	// Convert objects
	status.Objects = make([]TemplateInstanceObj, len(templateInstance.Status.Objects))
	for i, obj := range templateInstance.Status.Objects {
		status.Objects[i] = TemplateInstanceObj{
			Ref: obj.Ref,
		}
	}

	// Convert conditions
	status.Conditions = make([]TemplateInstanceCond, len(templateInstance.Status.Conditions))
	for i, cond := range templateInstance.Status.Conditions {
		status.Conditions[i] = TemplateInstanceCond{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		}
	}

	return status, nil
}

// DeleteTemplateInstance deletes a template instance
func (k *kubernetesService) DeleteTemplateInstance(ctx context.Context, namespace, name string) error {
	templateInstance := &templatev1.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	err := k.directClient.Delete(ctx, templateInstance)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete template instance %s/%s: %w", namespace, name, err)
	}

	// Also delete the parameter secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-params",
			Namespace: namespace,
		},
	}

	err = k.directClient.Delete(ctx, secret)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete parameter secret %s/%s: %w", namespace, name+"-params", err)
	}

	return nil
}
