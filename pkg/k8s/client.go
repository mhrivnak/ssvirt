package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Client wraps the controller-runtime client for OpenShift Virtualization operations
type Client struct {
	client.Client
	config *rest.Config
	cache  cache.Cache
}

// NewClient creates a new Kubernetes client using controller-runtime with caching
func NewClient() (*Client, error) {
	// Try to get cluster config first (in-cluster), then fall back to kubeconfig
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}
	return createClientWithCache(cfg)
}

// NewReadyClient creates a new client and starts the cache, waiting for sync
func NewReadyClient(ctx context.Context) (*Client, error) {
	client, err := NewClient()
	if err != nil {
		return nil, err
	}
	// Start the cache in a goroutine
	go func() {
		if err := client.Start(ctx); err != nil {
			// Log error but don't fail the client creation
			// The client will still work for direct API calls
			klog.Warningf("Failed to start cache: %v", err)
		}
	}()
	// Wait for cache to sync with a timeout
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !client.WaitForCacheSync(syncCtx) {
		// Cache didn't sync, but client can still be used for direct API calls
		klog.Warning("Cache did not sync within timeout, proceeding with direct API calls")
	}
	return client, nil
}

// createScheme creates and configures the runtime scheme with all required APIs
func createScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	// Add core Kubernetes APIs
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add core/v1 to scheme: %w", err)
	}
	// Add KubeVirt APIs
	if err := kubevirtv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add kubevirt APIs to scheme: %w", err)
	}
	return scheme, nil
}

// createClientWithCache creates a client with caching enabled
func createClientWithCache(cfg *rest.Config) (*Client, error) {
	// Create scheme with all required APIs
	scheme, err := createScheme()
	if err != nil {
		return nil, err
	}
	// Create cache with optimized settings
	cacheOptions := cache.Options{
		Scheme: scheme,
		// Resync period for cache refresh
		SyncPeriod: &[]time.Duration{10 * time.Minute}[0],
		// Cache for all namespaces by default
		DefaultNamespaces: map[string]cache.Config{},
	}
	// Create cache
	kubeCache, err := cache.New(cfg, cacheOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes cache: %w", err)
	}
	// Create client with cache for reads
	cl, err := client.New(cfg, client.Options{
		Scheme: scheme,
		Cache: &client.CacheOptions{
			Reader: kubeCache,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &Client{
		Client: cl,
		config: cfg,
		cache:  kubeCache,
	}, nil
}

// NewClientWithConfig creates a new Kubernetes client with explicit config path and caching
func NewClientWithConfig(kubeconfigPath string) (*Client, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig %s: %w", kubeconfigPath, err)
	}
	return createClientWithCache(cfg)
}

// GetConfig returns the REST config
func (c *Client) GetConfig() *rest.Config {
	return c.config
}

// GetCache returns the cache instance
func (c *Client) GetCache() cache.Cache {
	return c.cache
}

// Start starts the cache and begins watching for changes
func (c *Client) Start(ctx context.Context) error {
	if c.cache == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.cache.Start(ctx)
}

// WaitForCacheSync waits for the cache to sync before returning
func (c *Client) WaitForCacheSync(ctx context.Context) bool {
	if c.cache == nil {
		return false
	}
	return c.cache.WaitForCacheSync(ctx)
}

// VirtualMachineOperations provides methods for managing VirtualMachine resources
type VirtualMachineOperations struct {
	client    client.Client
	namespace string
}

// VMs returns a VirtualMachineOperations instance for the specified namespace
func (c *Client) VMs(namespace string) *VirtualMachineOperations {
	return &VirtualMachineOperations{
		client:    c.Client,
		namespace: namespace,
	}
}

// Create creates a new VirtualMachine
func (vmo *VirtualMachineOperations) Create(ctx context.Context, vm *kubevirtv1.VirtualMachine) error {
	vm.Namespace = vmo.namespace
	return vmo.client.Create(ctx, vm)
}

// Get retrieves a VirtualMachine by name
func (vmo *VirtualMachineOperations) Get(ctx context.Context, name string) (*kubevirtv1.VirtualMachine, error) {
	vm := &kubevirtv1.VirtualMachine{}
	err := vmo.client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: vmo.namespace,
	}, vm)
	return vm, err
}

// Update updates a VirtualMachine
func (vmo *VirtualMachineOperations) Update(ctx context.Context, vm *kubevirtv1.VirtualMachine) error {
	vm.Namespace = vmo.namespace
	return vmo.client.Update(ctx, vm)
}

// Delete deletes a VirtualMachine by name
func (vmo *VirtualMachineOperations) Delete(ctx context.Context, name string) error {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: vmo.namespace,
		},
	}
	return vmo.client.Delete(ctx, vm)
}

// List lists VirtualMachines in the namespace
func (vmo *VirtualMachineOperations) List(ctx context.Context, opts ...client.ListOption) (*kubevirtv1.VirtualMachineList, error) {
	vmList := &kubevirtv1.VirtualMachineList{}
	listOpts := append(opts, client.InNamespace(vmo.namespace))
	err := vmo.client.List(ctx, vmList, listOpts...)
	return vmList, err
}

// Patch patches a VirtualMachine
func (vmo *VirtualMachineOperations) Patch(ctx context.Context, vm *kubevirtv1.VirtualMachine, patch client.Patch) error {
	vm.Namespace = vmo.namespace
	return vmo.client.Patch(ctx, vm, patch)
}

// NamespaceOperations provides methods for managing Namespace resources
type NamespaceOperations struct {
	client client.Client
}

// Namespaces returns a NamespaceOperations instance
func (c *Client) Namespaces() *NamespaceOperations {
	return &NamespaceOperations{
		client: c.Client,
	}
}

// Create creates a new Namespace
func (no *NamespaceOperations) Create(ctx context.Context, ns *corev1.Namespace) error {
	return no.client.Create(ctx, ns)
}

// Get retrieves a Namespace by name
func (no *NamespaceOperations) Get(ctx context.Context, name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	err := no.client.Get(ctx, client.ObjectKey{Name: name}, ns)
	return ns, err
}

// Update updates a Namespace
func (no *NamespaceOperations) Update(ctx context.Context, ns *corev1.Namespace) error {
	return no.client.Update(ctx, ns)
}

// Delete deletes a Namespace by name
func (no *NamespaceOperations) Delete(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return no.client.Delete(ctx, ns)
}

// List lists all Namespaces
func (no *NamespaceOperations) List(ctx context.Context, opts ...client.ListOption) (*corev1.NamespaceList, error) {
	nsList := &corev1.NamespaceList{}
	err := no.client.List(ctx, nsList, opts...)
	return nsList, err
}

// Health checks if the client can connect to the cluster
func (c *Client) Health(ctx context.Context) error {
	// Check direct connectivity first with a simple list operation
	_, err := c.Namespaces().List(ctx, client.Limit(1))
	if err != nil {
		return fmt.Errorf("failed to connect to kubernetes cluster: %w", err)
	}
	// Check if cache is running (this is optional for health check)
	if c.cache != nil {
		// Try to get informer for namespaces to verify cache is accessible
		_, err := c.cache.GetInformer(ctx, &corev1.Namespace{})
		if err != nil {
			// Cache not ready, but client connectivity is fine
			return nil
		}
	}
	return nil
}
