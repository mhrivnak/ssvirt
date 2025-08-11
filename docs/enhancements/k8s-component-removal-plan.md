# Component Removal Plan for K8s Integration Re-enablement

## Overview

This document details the specific components that need to be removed when re-enabling Kubernetes integration with the embedded client approach. The goal is to eliminate the separate controller process while preserving all necessary functionality.

## Components to Remove

### 1. Separate Controller Process

#### Files to Delete Entirely

```bash
# Main controller binary
cmd/controller.disabled/main.go

# Controller implementation
pkg/controllers.disabled/vdc/controller.go

# Any other controller directories
pkg/controllers.disabled/template/
pkg/controllers.disabled/vapp/
```

**Rationale**: The separate controller process will be replaced by embedded Kubernetes client functionality in the API server.

**Functions to be Migrated**:
- VDC namespace lifecycle management → Embedded in VDC handlers
- Database polling → Replaced by synchronous operations
- Kubernetes event handling → Not needed with embedded approach

### 2. Helm Chart Controller Components

#### Files to Remove

**Complete file removal:**
```bash
chart/ssvirt/templates/controller-deployment.yaml
```

**Sections to remove from existing files:**

**From `chart/ssvirt/values.yaml`:**
```yaml
# Remove entire controller section (lines 65-101)
controller:
  enabled: true
  replicaCount: 1
  resources:
    limits:
      cpu: 200m
      memory: 256Mi
    requests:
      cpu: 100m
      memory: 128Mi
  livenessProbe:
    httpGet:
      path: /healthz
      port: 8081
    initialDelaySeconds: 30
    periodSeconds: 10
    timeoutSeconds: 5
    failureThreshold: 3
  readinessProbe:
    httpGet:
      path: /readyz
      port: 8081
    initialDelaySeconds: 5
    periodSeconds: 5
    timeoutSeconds: 3
    failureThreshold: 1
  nodeSelector: {}
  tolerations: []
  affinity: {}
```

**From `chart/ssvirt/templates/_helpers.tpl`:**
```yaml
# Remove controller-specific template helpers
{{- define "ssvirt.controllerName" -}}
{{- define "ssvirt.controllerLabels" -}}
{{- define "ssvirt.controllerSelectorLabels" -}}
{{- define "ssvirt.controllerServiceAccountName" -}}
```

#### Files to Modify

**`chart/ssvirt/templates/rbac.yaml`**
- Update ClusterRole permissions for API server
- Remove controller-specific subjects in ClusterRoleBinding
- Ensure API server has necessary Kubernetes permissions

**Before (Controller-focused):**
```yaml
rules:
# Basic namespace access for controller
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**After (API server needs):**
```yaml
rules:
# Namespace management for VDC operations
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]

# Template discovery in openshift namespace
- apiGroups: ["template.openshift.io"]
  resources: ["templates"]
  verbs: ["get", "list"]

# Template instantiation
- apiGroups: ["template.openshift.io"] 
  resources: ["templateinstances"]
  verbs: ["get", "list", "create", "update", "patch", "delete", "watch"]

# Resource management for VDC namespaces
- apiGroups: [""]
  resources: ["resourcequotas"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"] 
  verbs: ["get", "list", "create", "update", "patch", "delete"]
```

**`chart/ssvirt/templates/serviceaccount.yaml`**
- Ensure only API server ServiceAccount exists
- Remove controller-specific ServiceAccount if separate

### 3. Build and Deployment Configuration

#### Dockerfile Changes

**If there's a separate controller binary target:**
```dockerfile
# Remove controller build stage
FROM golang:1.21-alpine AS controller-builder
# ... controller-specific build steps

# Remove controller binary copy
COPY --from=controller-builder /app/ssvirt-controller /usr/local/bin/
```

**Keep only API server binary:**
```dockerfile 
FROM golang:1.21-alpine AS api-builder
# ... api server build steps

FROM alpine:3.18
COPY --from=api-builder /app/ssvirt-api-server /usr/local/bin/
```

#### Makefile Targets

**Remove controller-specific targets:**
```makefile
# Remove these targets
build-controller:
	go build -o bin/ssvirt-controller cmd/controller/main.go

docker-build-controller:
	docker build -t $(CONTROLLER_IMG) -f Dockerfile.controller .

deploy-controller:
	kubectl apply -f config/controller/
```

### 4. Documentation Updates

#### Files to Update

**`README.md`**
- Remove references to separate controller process
- Update architecture diagrams
- Update deployment instructions
- Remove controller-specific configuration examples

**`chart/ssvirt/README.md`**
- Remove controller configuration documentation
- Update values.yaml examples
- Update deployment instructions

**Architecture Documentation**
- Update system architecture diagrams
- Remove controller component from documentation
- Update deployment architecture

### 5. Configuration Files

#### Files to Remove/Update

**Remove if controller-specific:**
```bash
config/controller/
deployments/controller/
examples/controller-config.yaml
```

**Update shared configuration:**
- Remove controller-specific environment variables
- Update API server configuration to include Kubernetes client settings

### 6. Testing Infrastructure

#### Test Files to Remove

```bash
test/e2e/controller/
test/integration/controller/
test/unit/controller/
```

#### Test Files to Update

- Update API server tests to include Kubernetes operations
- Update integration tests for embedded client
- Update Helm chart tests to remove controller checks

### 7. CI/CD Pipeline Updates

#### GitHub Actions / CI Configuration

**Remove controller-specific jobs:**
```yaml
# Remove from .github/workflows/
- name: Test Controller
  run: make test-controller

- name: Build Controller Image  
  run: make docker-build-controller

- name: Deploy Controller
  run: make deploy-controller
```

**Update existing jobs:**
```yaml
# Update API server tests to include K8s integration tests
- name: Test API Server
  run: make test-api-server test-k8s-integration
```

## Implementation Procedure

### Step 1: Component Removal
1. Delete controller-specific files from codebase
2. Update Helm chart and remove controller templates
3. Update RBAC for API server needs

### Step 2: Enable Embedded Client
1. Move `pkg/k8s.disabled/` to `pkg/k8s/`
2. Remove build constraints (`//go:build ignore`)
3. Integrate client into API server initialization
4. Add Kubernetes operations to API handlers

### Step 3: Validation
1. Test VDC namespace creation
2. Test template discovery
3. Test template instantiation

### Step 4: Cleanup
1. Remove any remaining controller references
2. Update documentation
3. Update CI/CD pipelines

## Checklist for Complete Removal

### Code Changes
- [ ] Delete `cmd/controller.disabled/` directory
- [ ] Delete `pkg/controllers.disabled/` directory  
- [ ] Remove controller build targets from Makefile
- [ ] Update Dockerfile to remove controller binary
- [ ] Remove controller tests

### Helm Chart Changes
- [ ] Delete `chart/ssvirt/templates/controller-deployment.yaml`
- [ ] Remove controller section from `values.yaml`
- [ ] Remove controller helpers from `_helpers.tpl`
- [ ] Update RBAC for API server permissions
- [ ] Update ServiceAccount configuration

### Documentation Changes
- [ ] Update README.md
- [ ] Update chart README.md
- [ ] Update architecture documentation
- [ ] Remove controller configuration examples

### CI/CD Changes  
- [ ] Remove controller build/test jobs
- [ ] Update deployment scripts
- [ ] Update container image builds

### Operational Changes
- [ ] Remove controller monitoring/alerting configurations
- [ ] Update troubleshooting guides

## Rollback Plan

If issues arise during implementation:

1. **Code Rollback**:
   - Revert Git commits related to controller removal
   - Restore disabled directories with build constraints
   - Revert Helm chart changes

2. **Resource Cleanup**:
   - Clean up any test Kubernetes resources
   - Verify no orphaned resources

## Success Criteria

The removal is successful when:

1. **Functionality Preserved**:
   - VDC creation still creates namespaces
   - Template discovery works from API server
   - Template instantiation creates TemplateInstances

2. **Deployment Simplified**:
   - Only one deployment (API server) required
   - Reduced RBAC complexity
   - Single image build and deployment

3. **No Orphaned Resources**:
   - No leftover controller deployments
   - No unused RBAC resources
   - No orphaned Kubernetes resources

4. **Documentation Updated**:
   - All references to controller removed
   - New architecture documented
   - Deployment guides updated

This removal plan ensures a clean transition from the separate controller architecture to the embedded Kubernetes client approach.