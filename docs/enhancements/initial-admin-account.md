# Enhancement: Initial Admin Account

## Summary

This enhancement proposes adding support for creating an initial admin account when SSVirt is deployed. Currently, when an administrator deploys the service, there is no way to use it because there are no default accounts or bootstrap mechanism for creating the first user. This creates a "chicken and egg" problem where you need an account to create accounts.

## Motivation

### Problem Statement

When SSVirt is deployed fresh:
1. **No default accounts exist** - The database is empty with no users
2. **No bootstrap mechanism** - There's no way to create the first admin user
3. **Manual intervention required** - Admins must manually create database entries or use external tools
4. **Poor user experience** - The service appears broken after deployment since no one can log in

### Goals

- Provide a secure way to create an initial admin account during deployment
- Support Kubernetes-native credential management through secrets
- Ensure the admin account has appropriate permissions to manage the system
- Make the system immediately usable after deployment without manual database manipulation

### Non-Goals

- Creating multiple default accounts
- Implementing a full user management UI (separate enhancement)
- Changing the existing authentication mechanisms

## Proposal

### Design Overview

The initial admin account will be created during application startup if:
1. No users exist in the database, AND
2. Admin credentials are provided via configuration or Kubernetes secret

### Implementation Approach

#### 1. Configuration Structure

Add new configuration fields to support initial admin account creation:

```go
type Config struct {
    // ... existing fields ...
    
    InitialAdmin struct {
        Enabled   bool   `mapstructure:"enabled"`
        Username  string `mapstructure:"username"`
        Password  string `mapstructure:"password"`
        Email     string `mapstructure:"email"`
        FirstName string `mapstructure:"first_name"`
        LastName  string `mapstructure:"last_name"`
    } `mapstructure:"initial_admin"`
}
```

#### 2. Environment Variable Mapping

Support environment variables for secure credential passing:
- `SSVIRT_INITIAL_ADMIN_ENABLED=true`
- `SSVIRT_INITIAL_ADMIN_USERNAME=admin`
- `SSVIRT_INITIAL_ADMIN_PASSWORD=<secure-password>`
- `SSVIRT_INITIAL_ADMIN_EMAIL=admin@example.com`
- `SSVIRT_INITIAL_ADMIN_FIRST_NAME=System`
- `SSVIRT_INITIAL_ADMIN_LAST_NAME=Administrator`

#### 3. Kubernetes Secret Integration

Create a dedicated secret for admin credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ssvirt-initial-admin
  namespace: ssvirt-system
type: Opaque
data:
  username: YWRtaW4=  # base64 encoded "admin"
  password: <base64-encoded-secure-password>
  email: YWRtaW5AZXhhbXBsZS5jb20=  # base64 encoded "admin@example.com"
  first-name: U3lzdGVt  # base64 encoded "System"
  last-name: QWRtaW5pc3RyYXRvcg==  # base64 encoded "Administrator"
```

#### 4. Bootstrap Logic

Add bootstrap functionality during application startup:

```go
func (db *DB) BootstrapInitialAdmin(cfg *config.Config) error {
    // Only create if no users exist
    var userCount int64
    if err := db.DB.Model(&models.User{}).Count(&userCount).Error; err != nil {
        return fmt.Errorf("failed to count users: %w", err)
    }
    
    if userCount > 0 {
        log.Println("Users already exist, skipping initial admin creation")
        return nil
    }
    
    if !cfg.InitialAdmin.Enabled || cfg.InitialAdmin.Username == "" || cfg.InitialAdmin.Password == "" {
        log.Println("Initial admin not configured, skipping creation")
        return nil
    }
    
    // Create initial admin user with system administrator privileges
    return db.createInitialAdmin(cfg.InitialAdmin)
}
```

#### 5. Admin Role System

Introduce a system-level admin role that has privileges across all organizations:

```go
// Add to user model
type User struct {
    // ... existing fields ...
    IsSystemAdmin bool `gorm:"default:false" json:"is_system_admin"`
}

// System-level roles
const (
    RoleSystemAdmin = "SystemAdmin"  // Can manage entire system
    RoleOrgAdmin    = "OrgAdmin"     // Can manage specific organization  
    RoleVAppUser    = "VAppUser"     // Can use vApps in organization
    RoleVAppAuthor  = "VAppAuthor"   // Can create/modify vApps
)
```

### Security Considerations

#### Password Management
- **Generated passwords**: If no password is provided, generate a secure random password
- **Password complexity**: Enforce minimum password requirements
- **Secret rotation**: Support updating the initial admin credentials

#### Credential Storage
- **Environment variables**: Secure for container environments
- **Kubernetes secrets**: Native secret management with RBAC
- **No plaintext**: Never log or store passwords in plaintext
- **Cleanup**: Optionally remove environment variables after user creation

#### Access Control
- **System admin role**: Highest privilege level for initial setup
- **Audit logging**: Log initial admin account creation
- **Disable option**: Allow disabling the bootstrap mechanism

### Helm Chart Integration

#### Values Configuration

```yaml
# values.yaml
initialAdmin:
  enabled: true
  # Option 1: Direct configuration (not recommended for production)
  username: ""
  password: ""
  email: ""
  firstName: ""
  lastName: ""
  
  # Option 2: Existing secret reference (recommended)
  existingSecret: "ssvirt-initial-admin"
  
  # Option 3: Auto-generate password and store in secret
  autoGenerate: true
  secretName: "ssvirt-initial-admin-generated"
```

#### Template Updates

```yaml
# templates/secret.yaml (addition)
{{- if and .Values.initialAdmin.enabled .Values.initialAdmin.autoGenerate }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Values.initialAdmin.secretName | default "ssvirt-initial-admin-generated" }}
  namespace: {{ .Release.Namespace }}
type: Opaque
data:
  username: {{ .Values.initialAdmin.username | default "admin" | b64enc }}
  password: {{ randAlphaNum 32 | b64enc }}
  email: {{ .Values.initialAdmin.email | default "admin@example.com" | b64enc }}
  first-name: {{ .Values.initialAdmin.firstName | default "System" | b64enc }}
  last-name: {{ .Values.initialAdmin.lastName | default "Administrator" | b64enc }}
{{- end }}
```

## Implementation Plan

### Phase 1: Core Bootstrap Logic (Week 1)
1. Add initial admin configuration structure
2. Implement bootstrap logic in database connection
3. Add user creation with system admin privileges
4. Add environment variable support

### Phase 2: Kubernetes Integration (Week 2)  
1. Add Helm chart secret templates
2. Implement secret-based credential loading
3. Add password generation capability
4. Update deployment templates

### Phase 3: Security & Documentation (Week 3)
1. Add password complexity validation
2. Implement audit logging
3. Add configuration validation
4. Update admin setup guide
5. Add troubleshooting documentation

### Phase 4: Testing & Validation (Week 4)
1. Add unit tests for bootstrap logic
2. Add integration tests with Helm charts
3. Test secret rotation scenarios
4. Validate security measures

## Testing Strategy

### Unit Tests
- Test bootstrap logic with various configurations
- Test user creation with system admin privileges
- Test password generation and validation
- Test configuration validation

### Integration Tests
- Test Helm chart deployment with initial admin
- Test secret-based credential loading
- Test environment variable configuration
- Test startup behavior with/without existing users

### Security Tests
- Verify password hashing and storage
- Test credential cleanup after creation
- Validate RBAC with system admin role
- Test secret rotation capabilities

## Documentation Updates

### Admin Setup Guide
- Add section on initial admin account configuration
- Document Kubernetes secret setup
- Provide troubleshooting steps
- Include security best practices

### User Guide  
- Document system administrator capabilities
- Explain role hierarchy
- Provide user management instructions

## Backward Compatibility

This enhancement is fully backward compatible:
- **Existing deployments**: No impact if initial admin is not configured
- **Existing users**: System continues to work normally
- **Database schema**: New fields have default values
- **API compatibility**: No changes to existing endpoints

## Alternatives Considered

### 1. CLI Tool for Admin Creation
**Pros**: Simple, direct control
**Cons**: Requires additional tooling, not Kubernetes-native, manual process

### 2. Init Container Approach
**Pros**: Kubernetes-native, runs before main container
**Cons**: More complex deployment, harder to manage credentials

### 3. Web-based Setup Wizard
**Pros**: User-friendly interface
**Cons**: Security risk if left enabled, complex implementation

### 4. External Identity Provider Only
**Pros**: Leverages existing systems
**Cons**: Requires external dependencies, not suitable for standalone deployments

## Risks and Mitigations

### Risk: Credential Exposure
**Mitigation**: Use Kubernetes secrets, support credential cleanup, enforce password complexity

### Risk: Privilege Escalation  
**Mitigation**: Clear role separation, audit logging, principle of least privilege

### Risk: Bootstrap Conflicts
**Mitigation**: Atomic operations, proper error handling, idempotent creation

### Risk: Production Misconfiguration
**Mitigation**: Clear documentation, validation checks, secure defaults

## Success Criteria

1. **Immediate usability**: Fresh deployment is immediately usable with admin credentials
2. **Security**: Admin credentials are handled securely using industry best practices
3. **Kubernetes integration**: Works seamlessly with Helm charts and secrets
4. **Documentation**: Clear setup instructions for administrators
5. **Testing**: Comprehensive test coverage for all scenarios
6. **Backward compatibility**: No impact on existing deployments

## Future Enhancements

1. **Multi-admin support**: Create multiple initial administrators
2. **LDAP/OIDC integration**: Bootstrap with external identity providers  
3. **Role templates**: Predefined role configurations for common scenarios
4. **Web UI**: Administrative interface for user management
5. **Credential rotation**: Automatic periodic credential updates