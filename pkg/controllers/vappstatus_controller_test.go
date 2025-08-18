package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

func TestVAppStatusEvaluator_EvaluateStatus(t *testing.T) {
	tests := []struct {
		name                   string
		templateInstanceReady  bool
		templateInstanceFailed bool
		vmStatuses             []string
		hasVMs                 bool
		expectedStatus         string
	}{
		{
			name:                   "TemplateInstance failed",
			templateInstanceReady:  false,
			templateInstanceFailed: true,
			vmStatuses:             []string{},
			hasVMs:                 false,
			expectedStatus:         models.VAppStatusFailed,
		},
		{
			name:                   "TemplateInstance not ready, no VMs",
			templateInstanceReady:  false,
			templateInstanceFailed: false,
			vmStatuses:             []string{},
			hasVMs:                 false,
			expectedStatus:         models.VAppStatusInstantiating,
		},
		{
			name:                   "TemplateInstance ready but no VMs yet",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{},
			hasVMs:                 false,
			expectedStatus:         models.VAppStatusInstantiating,
		},
		{
			name:                   "TemplateInstance ready, VM still unresolved",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"UNRESOLVED"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusInstantiating,
		},
		{
			name:                   "TemplateInstance ready, VM empty status",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{""},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusInstantiating,
		},
		{
			name:                   "TemplateInstance ready, VM deleting",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"DELETING"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusDeleting,
		},
		{
			name:                   "TemplateInstance ready, VM deleted",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"DELETED"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusDeleting,
		},
		{
			name:                   "TemplateInstance ready, all VMs powered off",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"POWERED_OFF"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusDeployed,
		},
		{
			name:                   "TemplateInstance ready, all VMs powered on",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"POWERED_ON", "POWERED_ON"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusDeployed,
		},
		{
			name:                   "TemplateInstance ready, mixed VM states",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"POWERED_ON", "POWERED_OFF"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusDeployed,
		},
		{
			name:                   "TemplateInstance ready, VM suspended",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"SUSPENDED"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusDeployed,
		},
		{
			name:                   "TemplateInstance ready, mixed with one unresolved",
			templateInstanceReady:  true,
			templateInstanceFailed: false,
			vmStatuses:             []string{"POWERED_ON", "UNRESOLVED"},
			hasVMs:                 true,
			expectedStatus:         models.VAppStatusInstantiating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := &VAppStatusEvaluator{
				templateInstanceReady:  tt.templateInstanceReady,
				templateInstanceFailed: tt.templateInstanceFailed,
				vmStatuses:             tt.vmStatuses,
				hasVMs:                 tt.hasVMs,
			}

			result := evaluator.EvaluateStatus()
			assert.Equal(t, tt.expectedStatus, result)
		})
	}
}

func TestIsValidVAppStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "Valid status - INSTANTIATING",
			status:   models.VAppStatusInstantiating,
			expected: true,
		},
		{
			name:     "Valid status - DEPLOYED",
			status:   models.VAppStatusDeployed,
			expected: true,
		},
		{
			name:     "Valid status - FAILED",
			status:   models.VAppStatusFailed,
			expected: true,
		},
		{
			name:     "Valid status - DELETING",
			status:   models.VAppStatusDeleting,
			expected: true,
		},
		{
			name:     "Valid status - DELETED",
			status:   models.VAppStatusDeleted,
			expected: true,
		},
		{
			name:     "Valid status - POWERING_ON",
			status:   models.VAppStatusPoweringOn,
			expected: true,
		},
		{
			name:     "Valid status - POWERING_OFF",
			status:   models.VAppStatusPoweringOff,
			expected: true,
		},
		{
			name:     "Invalid status - empty",
			status:   "",
			expected: false,
		},
		{
			name:     "Invalid status - unknown",
			status:   "UNKNOWN_STATUS",
			expected: false,
		},
		{
			name:     "Invalid status - lowercase",
			status:   "deployed",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := models.IsValidVAppStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}
