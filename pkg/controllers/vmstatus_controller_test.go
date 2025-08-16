package controllers

import (
	"context"
	"testing"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

// MockVMRepository mocks the VM repository
type MockVMRepository struct {
	mock.Mock
}

func (m *MockVMRepository) GetByNamespaceAndVMName(ctx context.Context, namespace, vmName string) (*models.VM, error) {
	args := m.Called(ctx, namespace, vmName)
	if vm := args.Get(0); vm != nil {
		return vm.(*models.VM), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockVMRepository) GetByVAppAndVMName(ctx context.Context, vappID, vmName string) (*models.VM, error) {
	args := m.Called(ctx, vappID, vmName)
	if vm := args.Get(0); vm != nil {
		return vm.(*models.VM), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockVMRepository) UpdateStatus(ctx context.Context, vmID string, status string) error {
	args := m.Called(ctx, vmID, status)
	return args.Error(0)
}

func (m *MockVMRepository) CreateVM(ctx context.Context, vm *models.VM) error {
	args := m.Called(ctx, vm)
	return args.Error(0)
}

func (m *MockVMRepository) UpdateVMData(ctx context.Context, vmID string, cpuCount *int, memoryMB *int, guestOS string) error {
	args := m.Called(ctx, vmID, cpuCount, memoryMB, guestOS)
	return args.Error(0)
}

// MockVAppRepository mocks the VApp repository
type MockVAppRepository struct {
	mock.Mock
}

func (m *MockVAppRepository) GetByNameInVDC(ctx context.Context, vdcID, name string) (*models.VApp, error) {
	args := m.Called(ctx, vdcID, name)
	if vapp := args.Get(0); vapp != nil {
		return vapp.(*models.VApp), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockVAppRepository) CreateVApp(ctx context.Context, vapp *models.VApp) error {
	args := m.Called(ctx, vapp)
	return args.Error(0)
}

// MockVDCRepository mocks the VDC repository
type MockVDCRepository struct {
	mock.Mock
}

func (m *MockVDCRepository) GetByNamespace(ctx context.Context, namespaceName string) (*models.VDC, error) {
	args := m.Called(ctx, namespaceName)
	if vdc := args.Get(0); vdc != nil {
		return vdc.(*models.VDC), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockEventRecorder mocks the Kubernetes event recorder
type MockEventRecorder struct {
	Events []string
}

func (m *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	m.Events = append(m.Events, eventtype+":"+reason+":"+message)
}

func (m *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	// Not needed for these tests
}

func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	// Not needed for these tests
}

func TestVMStatusController_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = kubevirtv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = templatev1.AddToScheme(scheme)

	// Reset metrics for clean test state
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	tests := []struct {
		name           string
		setupVM        func() *kubevirtv1.VirtualMachine
		setupRepo      func(repo *MockVMRepository)
		expectedResult ctrl.Result
		expectedError  bool
		expectedEvents int
	}{
		{
			name: "VM not found - deletion handling",
			setupVM: func() *kubevirtv1.VirtualMachine {
				return nil // VM not found
			},
			setupRepo: func(repo *MockVMRepository) {
				repo.On("GetByNamespaceAndVMName", mock.Anything, "test-namespace", "test-vm").
					Return(nil, gorm.ErrRecordNotFound)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
			expectedEvents: 0,
		},
		{
			name: "VM status update - success",
			setupVM: func() *kubevirtv1.VirtualMachine {
				return &kubevirtv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"vapp.ssvirt.io/vapp-id": "vapp-123",
						},
					},
					Status: kubevirtv1.VirtualMachineStatus{
						PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
					},
				}
			},
			setupRepo: func(repo *MockVMRepository) {
				vm := &models.VM{
					ID:        "vm-123",
					Name:      "test-vm",
					VMName:    "test-vm",
					Namespace: "test-namespace",
					Status:    "POWERED_OFF",
					UpdatedAt: time.Now().Add(-5 * time.Minute),
				}
				repo.On("GetByVAppAndVMName", mock.Anything, "vapp-123", "test-vm").
					Return(vm, nil)
				repo.On("UpdateStatus", mock.Anything, "vm-123", "POWERED_ON").
					Return(nil)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
			expectedEvents: 1,
		},
		{
			name: "VM not managed by SSVirt",
			setupVM: func() *kubevirtv1.VirtualMachine {
				return &kubevirtv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-namespace",
					},
					Status: kubevirtv1.VirtualMachineStatus{
						PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
					},
				}
			},
			setupRepo: func(repo *MockVMRepository) {
				repo.On("GetByNamespaceAndVMName", mock.Anything, "test-namespace", "test-vm").
					Return(nil, gorm.ErrRecordNotFound)
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
			expectedEvents: 0,
		},
		{
			name: "Database update error",
			setupVM: func() *kubevirtv1.VirtualMachine {
				return &kubevirtv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-vm",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"vapp.ssvirt.io/vapp-id": "vapp-123",
						},
					},
					Status: kubevirtv1.VirtualMachineStatus{
						PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
					},
				}
			},
			setupRepo: func(repo *MockVMRepository) {
				vm := &models.VM{
					ID:        "vm-123",
					Name:      "test-vm",
					VMName:    "test-vm",
					Namespace: "test-namespace",
					Status:    "POWERED_OFF",
					UpdatedAt: time.Now().Add(-5 * time.Minute),
				}
				repo.On("GetByVAppAndVMName", mock.Anything, "vapp-123", "test-vm").
					Return(vm, nil)
				repo.On("UpdateStatus", mock.Anything, "vm-123", "POWERED_ON").
					Return(assert.AnError)
			},
			expectedResult: ctrl.Result{RequeueAfter: time.Minute},
			expectedError:  true,
			expectedEvents: 1, // Warning event
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repositories
			mockVMRepo := new(MockVMRepository)
			tt.setupRepo(mockVMRepo)
			mockVAppRepo := new(MockVAppRepository)
			mockVDCRepo := new(MockVDCRepository)

			// Create mock event recorder
			mockRecorder := &MockEventRecorder{}

			// Create fake client
			var objs []client.Object
			if vm := tt.setupVM(); vm != nil {
				objs = append(objs, vm)
			}
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create controller
			controller := &VMStatusController{
				Client:   fakeClient,
				Scheme:   scheme,
				VMRepo:   VMRepositoryInterface(mockVMRepo),
				VAppRepo: VAppRepositoryInterface(mockVAppRepo),
				VDCRepo:  VDCRepositoryInterface(mockVDCRepo),
				Recorder: mockRecorder,
			}

			// Create request
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-vm",
					Namespace: "test-namespace",
				},
			}

			// Execute reconcile
			result, err := controller.Reconcile(context.Background(), req)

			// Validate results
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Len(t, mockRecorder.Events, tt.expectedEvents)

			// Verify mock expectations
			mockVMRepo.AssertExpectations(t)
		})
	}
}

func TestMapVMStatus(t *testing.T) {
	tests := []struct {
		name     string
		vm       *kubevirtv1.VirtualMachine
		expected string
	}{
		{
			name: "Running VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
				},
			},
			expected: "POWERED_ON",
		},
		{
			name: "Stopped VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
				},
			},
			expected: "POWERED_OFF",
		},
		{
			name: "Starting VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStarting,
				},
			},
			expected: "POWERING_ON",
		},
		{
			name: "Stopping VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStopping,
				},
			},
			expected: "POWERING_OFF",
		},
		{
			name: "Terminating VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusTerminating,
				},
			},
			expected: "POWERING_OFF",
		},
		{
			name: "Provisioning VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusProvisioning,
				},
			},
			expected: "STARTING",
		},
		{
			name: "Paused VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusPaused,
				},
			},
			expected: "SUSPENDED",
		},
		{
			name: "Migrating VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusMigrating,
				},
			},
			expected: "POWERED_ON",
		},
		{
			name: "CrashLoopBackOff VM",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusCrashLoopBackOff,
				},
			},
			expected: "ERROR",
		},
		{
			name: "VM with deletion timestamp",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
				},
			},
			expected: "DELETING",
		},
		{
			name: "VM with no status but should be running",
			vm: &kubevirtv1.VirtualMachine{
				Spec: kubevirtv1.VirtualMachineSpec{
					Running: &[]bool{true}[0],
				},
			},
			expected: "STARTING",
		},
		{
			name: "VM with no status and should not be running",
			vm: &kubevirtv1.VirtualMachine{
				Spec: kubevirtv1.VirtualMachineSpec{
					Running: &[]bool{false}[0],
				},
			},
			expected: "STOPPED",
		},
		{
			name: "Unknown VM status",
			vm: &kubevirtv1.VirtualMachine{
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusUnknown,
				},
			},
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapVMStatus(tt.vm)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractVMInfo(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"vapp.ssvirt.io/vapp-id": "vapp-123",
				"vdc.ssvirt.io/vdc-id":   "vdc-456",
			},
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusRunning,
		},
	}

	controller := &VMStatusController{}
	info := controller.extractVMInfo(vm)

	assert.Equal(t, "test-vm", info.Name)
	assert.Equal(t, "test-namespace", info.Namespace)
	assert.Equal(t, "POWERED_ON", info.Status)
	assert.Equal(t, "vapp-123", info.VAppID)
	assert.Equal(t, "vdc-456", info.VDCID)
	assert.WithinDuration(t, time.Now(), info.UpdatedAt, time.Second)
}

func TestFindVMRecord(t *testing.T) {
	tests := []struct {
		name      string
		vm        *kubevirtv1.VirtualMachine
		setupRepo func(repo *MockVMRepository)
		expectErr bool
	}{
		{
			name: "Find by vApp and VM name",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"vapp.ssvirt.io/vapp-id": "vapp-123",
					},
				},
			},
			setupRepo: func(repo *MockVMRepository) {
				vm := &models.VM{ID: "vm-123"}
				repo.On("GetByVAppAndVMName", mock.Anything, "vapp-123", "test-vm").
					Return(vm, nil)
			},
			expectErr: false,
		},
		{
			name: "Find by namespace and VM name (fallback)",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
				},
			},
			setupRepo: func(repo *MockVMRepository) {
				vm := &models.VM{ID: "vm-123"}
				repo.On("GetByNamespaceAndVMName", mock.Anything, "test-namespace", "test-vm").
					Return(vm, nil)
			},
			expectErr: false,
		},
		{
			name: "VM not found",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
				},
			},
			setupRepo: func(repo *MockVMRepository) {
				repo.On("GetByNamespaceAndVMName", mock.Anything, "test-namespace", "test-vm").
					Return(nil, gorm.ErrRecordNotFound)
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockVMRepo := new(MockVMRepository)
			tt.setupRepo(mockVMRepo)
			mockVAppRepo := new(MockVAppRepository)
			mockVDCRepo := new(MockVDCRepository)

			controller := &VMStatusController{
				VMRepo:   VMRepositoryInterface(mockVMRepo),
				VAppRepo: VAppRepositoryInterface(mockVAppRepo),
				VDCRepo:  VDCRepositoryInterface(mockVDCRepo),
			}

			vm, err := controller.findOrCreateVMRecord(context.Background(), tt.vm)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, vm)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, vm)
			}

			mockVMRepo.AssertExpectations(t)
		})
	}
}

func TestEnsureVAppLabel(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = kubevirtv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = templatev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		vm            *kubevirtv1.VirtualMachine
		templateInst  *templatev1.TemplateInstance
		expectedLabel string
		expectUpdate  bool
		expectError   bool
	}{
		{
			name: "VM already has vapp.ssvirt label - no update",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"vapp.ssvirt": "existing-vapp",
					},
				},
			},
			expectedLabel: "existing-vapp",
			expectUpdate:  false,
			expectError:   false,
		},
		{
			name: "VM with template instance owner - should update label",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"template.openshift.io/template-instance-owner": "test-template-uid",
					},
				},
			},
			templateInst: &templatev1.TemplateInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-template-instance",
					Namespace: "test-namespace",
					UID:       "test-template-uid",
				},
			},
			expectedLabel: "my-template-instance",
			expectUpdate:  true,
			expectError:   false,
		},
		{
			name: "VM without template instance owner - no update",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
					Labels:    map[string]string{},
				},
			},
			expectedLabel: "",
			expectUpdate:  false,
			expectError:   false,
		},
		{
			name: "VM with template instance owner but TemplateInstance not found - no update",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"template.openshift.io/template-instance-owner": "non-existent-uid",
					},
				},
			},
			expectedLabel: "",
			expectUpdate:  false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Create fake client with objects
			var objs []client.Object
			objs = append(objs, tt.vm)
			if tt.templateInst != nil {
				objs = append(objs, tt.templateInst)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			// Create controller with fake client
			mockRecorder := &MockEventRecorder{}
			controller := &VMStatusController{
				Client:   fakeClient,
				Scheme:   scheme,
				Recorder: mockRecorder,
			}

			// Call ensureVAppLabel
			updatedVM, err := controller.ensureVAppLabel(ctx, tt.vm)

			// Check error expectation
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Check update expectation
			if tt.expectUpdate {
				assert.NotNil(t, updatedVM, "Expected updated VM to be returned")
				assert.Equal(t, tt.expectedLabel, updatedVM.Labels["vapp.ssvirt"])

				// Verify the VM was actually updated in the fake client
				var vmInClient kubevirtv1.VirtualMachine
				err = fakeClient.Get(ctx, types.NamespacedName{
					Name:      tt.vm.Name,
					Namespace: tt.vm.Namespace,
				}, &vmInClient)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLabel, vmInClient.Labels["vapp.ssvirt"])
			} else {
				assert.Nil(t, updatedVM, "Expected no update, but got updated VM")

				// Verify original VM labels are unchanged
				var vmInClient kubevirtv1.VirtualMachine
				err = fakeClient.Get(ctx, types.NamespacedName{
					Name:      tt.vm.Name,
					Namespace: tt.vm.Namespace,
				}, &vmInClient)
				assert.NoError(t, err)

				if tt.expectedLabel != "" {
					// Case where label should remain unchanged
					assert.Equal(t, tt.expectedLabel, vmInClient.Labels["vapp.ssvirt"])
				} else {
					// Case where no vapp.ssvirt label should exist
					_, hasLabel := vmInClient.Labels["vapp.ssvirt"]
					assert.False(t, hasLabel, "Expected no vapp.ssvirt label")
				}
			}
		})
	}
}

func TestExtractVMIData(t *testing.T) {
	tests := []struct {
		name     string
		vmi      *kubevirtv1.VirtualMachineInstance
		expected VMIData
	}{
		{
			name: "Complete VMI with all data",
			vmi: &kubevirtv1.VirtualMachineInstance{
				Status: kubevirtv1.VirtualMachineInstanceStatus{
					CurrentCPUTopology: &kubevirtv1.CPUTopology{
						Cores:   2,
						Sockets: 1,
						Threads: 1,
					},
					Memory: &kubevirtv1.MemoryStatus{
						GuestCurrent: resourceQuantityPtr(4 * 1024 * 1024 * 1024), // 4GB in bytes
					},
					GuestOSInfo: kubevirtv1.VirtualMachineInstanceGuestOSInfo{
						PrettyName: "CentOS Stream 9",
					},
				},
			},
			expected: VMIData{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
		},
		{
			name: "VMI with partial data",
			vmi: &kubevirtv1.VirtualMachineInstance{
				Status: kubevirtv1.VirtualMachineInstanceStatus{
					CurrentCPUTopology: &kubevirtv1.CPUTopology{
						Cores:   4,
						Sockets: 2,
						Threads: 1,
					},
				},
			},
			expected: VMIData{
				CPUCount: intPtr(8), // 4 cores * 2 sockets * 1 thread
				MemoryMB: nil,
				GuestOS:  "",
			},
		},
		{
			name: "VMI with guest OS name and version",
			vmi: &kubevirtv1.VirtualMachineInstance{
				Status: kubevirtv1.VirtualMachineInstanceStatus{
					GuestOSInfo: kubevirtv1.VirtualMachineInstanceGuestOSInfo{
						Name:    "Ubuntu",
						Version: "22.04",
					},
				},
			},
			expected: VMIData{
				CPUCount: nil,
				MemoryMB: nil,
				GuestOS:  "Ubuntu 22.04",
			},
		},
		{
			name: "Empty VMI",
			vmi: &kubevirtv1.VirtualMachineInstance{
				Status: kubevirtv1.VirtualMachineInstanceStatus{},
			},
			expected: VMIData{
				CPUCount: nil,
				MemoryMB: nil,
				GuestOS:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVMIData(tt.vmi)
			assert.Equal(t, tt.expected.CPUCount, result.CPUCount)
			assert.Equal(t, tt.expected.MemoryMB, result.MemoryMB)
			assert.Equal(t, tt.expected.GuestOS, result.GuestOS)
		})
	}
}

func TestExtractVMSpecData(t *testing.T) {
	tests := []struct {
		name     string
		vm       *kubevirtv1.VirtualMachine
		expected VMIData
	}{
		{
			name: "VM with complete spec",
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"vm.kubevirt.io/os": "rhel9",
					},
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Domain: kubevirtv1.DomainSpec{
								CPU: &kubevirtv1.CPU{
									Cores:   2,
									Sockets: 1,
									Threads: 1,
								},
								Memory: &kubevirtv1.Memory{
									Guest: resourceQuantityPtr(2 * 1024 * 1024 * 1024), // 2GB
								},
							},
						},
					},
				},
			},
			expected: VMIData{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(2048),
				GuestOS:  "rhel9",
			},
		},
		{
			name: "VM with multi-socket CPU",
			vm: &kubevirtv1.VirtualMachine{
				Spec: kubevirtv1.VirtualMachineSpec{
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Domain: kubevirtv1.DomainSpec{
								CPU: &kubevirtv1.CPU{
									Cores:   2,
									Sockets: 2,
									Threads: 2,
								},
							},
						},
					},
				},
			},
			expected: VMIData{
				CPUCount: intPtr(8), // 2 cores * 2 sockets * 2 threads
				MemoryMB: nil,
				GuestOS:  "",
			},
		},
		{
			name: "VM with no spec data",
			vm: &kubevirtv1.VirtualMachine{
				Spec: kubevirtv1.VirtualMachineSpec{
					Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
						Spec: kubevirtv1.VirtualMachineInstanceSpec{
							Domain: kubevirtv1.DomainSpec{},
						},
					},
				},
			},
			expected: VMIData{
				CPUCount: nil,
				MemoryMB: nil,
				GuestOS:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVMSpecData(tt.vm)
			assert.Equal(t, tt.expected.CPUCount, result.CPUCount)
			assert.Equal(t, tt.expected.MemoryMB, result.MemoryMB)
			assert.Equal(t, tt.expected.GuestOS, result.GuestOS)
		})
	}
}

func TestFormatGuestOS(t *testing.T) {
	tests := []struct {
		name     string
		osInfo   *kubevirtv1.VirtualMachineInstanceGuestOSInfo
		expected string
	}{
		{
			name: "Pretty name available",
			osInfo: &kubevirtv1.VirtualMachineInstanceGuestOSInfo{
				PrettyName: "CentOS Stream 9",
				Name:       "centos",
				Version:    "9",
			},
			expected: "CentOS Stream 9",
		},
		{
			name: "Name and version available",
			osInfo: &kubevirtv1.VirtualMachineInstanceGuestOSInfo{
				Name:    "Ubuntu",
				Version: "22.04",
			},
			expected: "Ubuntu 22.04",
		},
		{
			name: "Only name available",
			osInfo: &kubevirtv1.VirtualMachineInstanceGuestOSInfo{
				Name: "Fedora",
			},
			expected: "Fedora",
		},
		{
			name: "Only ID available",
			osInfo: &kubevirtv1.VirtualMachineInstanceGuestOSInfo{
				ID: "rhel",
			},
			expected: "rhel",
		},
		{
			name:     "Empty OS info",
			osInfo:   &kubevirtv1.VirtualMachineInstanceGuestOSInfo{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatGuestOS(tt.osInfo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNeedsVMDataUpdate(t *testing.T) {
	controller := &VMStatusController{}

	tests := []struct {
		name     string
		vmRecord *models.VM
		newData  VMIData
		expected bool
	}{
		{
			name: "CPU count change needed",
			vmRecord: &models.VM{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			newData: VMIData{
				CPUCount: intPtr(4),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			expected: true,
		},
		{
			name: "Memory change needed",
			vmRecord: &models.VM{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(2048),
				GuestOS:  "CentOS Stream 9",
			},
			newData: VMIData{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			expected: true,
		},
		{
			name: "Guest OS change needed",
			vmRecord: &models.VM{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 8",
			},
			newData: VMIData{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			expected: true,
		},
		{
			name: "No change needed",
			vmRecord: &models.VM{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			newData: VMIData{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			expected: false,
		},
		{
			name: "New data has nil values - no change",
			vmRecord: &models.VM{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			newData: VMIData{
				CPUCount: nil,
				MemoryMB: nil,
				GuestOS:  "",
			},
			expected: false,
		},
		{
			name: "VM has nil values but new data has values - change needed",
			vmRecord: &models.VM{
				CPUCount: nil,
				MemoryMB: nil,
				GuestOS:  "",
			},
			newData: VMIData{
				CPUCount: intPtr(2),
				MemoryMB: intPtr(4096),
				GuestOS:  "CentOS Stream 9",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := controller.needsVMDataUpdate(tt.vmRecord, tt.newData)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapVMIToVM(t *testing.T) {
	controller := &VMStatusController{}

	tests := []struct {
		name     string
		obj      client.Object
		expected []reconcile.Request
	}{
		{
			name: "VMI with VM owner reference",
			obj: &kubevirtv1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "VirtualMachine",
							APIVersion: "kubevirt.io/v1",
							Name:       "test-vm",
						},
					},
				},
			},
			expected: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "test-namespace",
						Name:      "test-vm",
					},
				},
			},
		},
		{
			name: "VMI without VM owner reference",
			obj: &kubevirtv1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi",
					Namespace: "test-namespace",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind:       "ReplicaSet",
							APIVersion: "apps/v1",
							Name:       "test-rs",
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Non-VMI object",
			obj: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-namespace",
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := controller.mapVMIToVM(context.Background(), tt.obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions for tests
func intPtr(i int) *int {
	return &i
}

func resourceQuantityPtr(bytes int64) *resource.Quantity {
	q := resource.NewQuantity(bytes, resource.BinarySI)
	return q
}
