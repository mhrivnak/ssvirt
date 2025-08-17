# vApp Status Lifecycle Enhancement

## Overview

Currently, vApp status is set to `INSTANTIATING` during creation and never
changes from that state. This enhancement proposes implementing a complete vApp
status lifecycle that accurately reflects the state of vApps and their
underlying resources throughout their lifecycle.

## Problem Statement

### Current Behavior
- vApps are created with status `INSTANTIATING` in `pkg/api/handlers/vm_creation.go:240`
- When Kubernetes service is available, status may be updated to `INSTANTIATING` again in line 434
- When Kubernetes service is unavailable, status remains `INSTANTIATING`
- No controller or mechanism exists to update status based on actual TemplateInstance or VM states
- Status field comment suggests support for multiple states but only `INSTANTIATING` is effectively used
- Users see perpetually "instantiating" vApps regardless of actual deployment state

### Code Analysis
From the current codebase:
- **VM status tracking**: Exists via `VMStatusController` that updates VM status based on KubeVirt resources
- **TemplateInstance creation**: Handled in `kubernetes.go` with `CreateTemplateInstance` method
- **Status checking**: `GetTemplateInstance` method exists but is not actively used for vApp status updates
- **vApp deletion**: Has status checks for running VMs but no comprehensive status lifecycle

### Impact
- Poor user experience with misleading status information
- No visibility into vApp deployment progress or failures
- Inability to determine when vApps are ready for use
- Inconsistent with VMware Cloud Director API expectations

## Proposed Solution

### vApp Status Values

Based on VMware Cloud Director API specifications and common vApp lifecycle patterns, implement the following status values:

#### Core Status Values
- `INSTANTIATING` - Initial state, currently being instantiated from template
- `DEPLOYED` - Successfully deployed and available
- `FAILED` - Instantiation or deployment failed
- `DELETING` - Currently being deleted
- `DELETED` - Marked for deletion (soft delete state)

#### Transitional Status Values
- `POWERING_ON` - VMs within vApp are powering on
- `POWERING_OFF` - VMs within vApp are powering off

### Status Sources and Derivation

#### 1. TemplateInstance Status Monitoring
**Source**: OpenShift TemplateInstance.Status
**Mapping**:
- TemplateInstance conditions with `status: "True"` and `type: "Ready"` → `DEPLOYED`
- TemplateInstance conditions with `status: "False"` and `type: "InstantiateFailure"` → `FAILED`
- TemplateInstance in progress (no ready condition) → `INSTANTIATING`

#### 2. VM Aggregated Status
**Source**: VM status within the vApp
**Logic**:
- All VMs `POWERED_OFF` → vApp `DEPLOYED` (deployed but not running)
- All VMs `POWERED_ON` → vApp `DEPLOYED` (fully operational)
- Mixed VM states → vApp `DEPLOYED` (partially operational)
- Any VM still provisioning → vApp `INSTANTIATING` (still provisioning)
- Any VM `DELETING`/`DELETED` → vApp `DELETING`

#### 3. Resource Availability Check
**Source**: Kubernetes resource validation
**Validation**:
- Check VirtualMachine resources exist in target namespace
- Verify resource specifications match expected configuration
- Validate resource readiness and health

### Status Transition Rules

```
INSTANTIATING → DEPLOYED/FAILED
DEPLOYED → POWERING_OFF → DEPLOYED
DEPLOYED → POWERING_ON → DEPLOYED
ANY_STATE → DELETING → DELETED
FAILED → INSTANTIATING (on retry)
```

### Implementation Architecture

#### 1. vApp Status Controller
Create a new Kubernetes controller specifically for vApp status management:

```go
// VAppStatusController watches TemplateInstance and VM resources
type VAppStatusController struct {
    client.Client
    VAppRepo     VAppRepositoryInterface
    VMRepo       VMRepositoryInterface
    TemplateRepo TemplateRepositoryInterface
}
```

**Responsibilities**:
- Watch TemplateInstance resources for vApp instantiation status
- Aggregate VM statuses within each vApp
- Update vApp status based on combined state evaluation
- Handle status transition validation and logging

#### 2. Status Evaluation Engine
Implement a status evaluation engine that considers multiple inputs:

```go
type VAppStatusEvaluator struct {
    templateInstanceStatus *TemplateInstanceStatus
    vmStatuses            []string
    resourceHealth        *ResourceHealthStatus
}

func (e *VAppStatusEvaluator) EvaluateStatus() string {
    // Priority order:
    // 1. Check for deletion state
    // 2. Check TemplateInstance status for instantiation/failure
    // 3. Aggregate VM statuses for operational state
    // 4. Consider resource health for stability
}
```

#### 3. Status Update Mechanism
- **Event-driven updates**: On TemplateInstance, VirtualMachine, and VirtualMachineInstance status changes
- **Controller-runtime reconciliation**: Default reconciliation behavior only
- **Error handling**: Retry logic with exponential backoff
- **Metrics**: Track status transition frequency and duration

#### 4. Database Schema Updates
No schema changes required - using existing `status` field in `v_apps` table.

#### 5. API Response Updates
No changes to vApp API response structure - only the existing `status` field will be updated with accurate values:

```json
{
  "id": "urn:vcloud:vapp:...",
  "status": "DEPLOYED",
  "name": "my-vapp",
  "description": "My vApp description"
}
```

## Implementation Plan

### Phase 1: Core Status Lifecycle (Week 1-2)
1. Implement vApp status constants and validation
2. Create VAppStatusController with basic TemplateInstance monitoring
3. Add status evaluation logic for core states
4. Update vApp creation flow to use new status lifecycle

### Phase 2: VM Integration (Week 3)
1. Integrate VM status aggregation into vApp status evaluation
2. Implement VM-based status transitions (DEPLOYED, SUSPENDED, etc.)
3. Add real-time status updates based on VM changes
4. Test status accuracy with various VM configurations

### Phase 3: Enhanced Monitoring (Week 4)
1. Add Kubernetes resource health checking
2. Implement status metadata and detailed reporting
3. Add metrics and observability for status transitions
4. Performance optimization and error handling improvements

### Phase 4: Integration Testing (Week 5)
1. End-to-end testing of status lifecycle
2. Load testing with multiple concurrent vApp operations
3. Edge case testing (network failures, resource constraints)
4. Documentation and API reference updates

## Testing Strategy

### Unit Tests
- Status evaluation logic with various input combinations
- Status transition validation and edge cases
- Mock TemplateInstance and VM status scenarios

### Integration Tests
- vApp lifecycle from creation to deletion
- Status accuracy during VM operations (power on/off, suspend)
- Controller behavior during Kubernetes API unavailability

### End-to-End Tests
- Complete vApp deployment scenarios
- User experience validation through API responses
- Performance benchmarks for status update latency

## Monitoring and Observability

### Metrics
- `vapp_status_transitions_total{from, to}` - Status transition counters
- `vapp_status_update_duration_seconds` - Status update latency
- `vapp_status_evaluation_errors_total` - Error rate monitoring
- `vapp_current_status{status}` - Current vApp status distribution

### Logging
- Structured logging for all status transitions
- Debug logging for status evaluation decisions
- Error logging with context for troubleshooting

### Health Checks
- Controller readiness and liveness probes
- Status update pipeline health monitoring
- Dependency availability checks (Kubernetes API, database)

## Backward Compatibility

### API Compatibility
- Existing status values remain valid during transition period
- New status values introduced gradually with feature flags
- API version compatibility maintained

### Database Compatibility
- No schema changes required
- Existing vApps will be migrated to appropriate status
- Migration script for existing `INSTANTIATING` vApps

### User Experience
- Clear documentation of new status values and meanings
- Migration guide for API consumers
- Gradual rollout with monitoring for issues

## Risk Mitigation

### Technical Risks
1. **Status flapping**: Implement debouncing and minimum state duration
2. **Performance impact**: Async processing and rate limiting
3. **Inconsistent state**: Comprehensive validation and reconciliation
4. **Controller failures**: High availability and automatic recovery

### Operational Risks
1. **Status confusion**: Clear documentation and user education
2. **API breaking changes**: Careful versioning and compatibility testing
3. **Migration issues**: Thorough testing and rollback procedures
4. **Monitoring overhead**: Efficient metrics collection and storage

## Success Criteria

### Functional Requirements
- [ ] vApp status accurately reflects actual state
- [ ] Status transitions follow defined lifecycle rules
- [ ] Real-time updates within 30 seconds of state changes
- [ ] Error states properly detected and reported

### Performance Requirements
- [ ] Status updates complete within 5 seconds
- [ ] Controller handles 1000+ vApps without degradation
- [ ] Minimal impact on existing API response times
- [ ] Resource usage within acceptable limits

### Reliability Requirements
- [ ] 99.9% status update success rate
- [ ] Automatic recovery from temporary failures
- [ ] Consistent behavior during high load
- [ ] Graceful degradation during dependency outages

## Conclusion

This enhancement addresses the critical gap in vApp status management by implementing a comprehensive status lifecycle that accurately reflects vApp state. The proposed solution provides clear status visibility, follows VMware Cloud Director patterns, and maintains backward compatibility while enabling better user experience and operational monitoring.

The phased implementation approach ensures gradual rollout with proper testing and validation at each stage, minimizing risk while delivering immediate value to users through accurate status reporting.