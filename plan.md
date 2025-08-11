# Implementation Plan: OpenShift-Based VMware Cloud Director

We are going to create an application that re-implements the basic capabilities of VMware Cloud Director. It will implement the exact same API specification. Instead of using vsphere, we will use OpenShift Virtualization and OpenShift Advanced Cluster Management to provide the required services including virtual machines and isolated networks.

## References and Documentation

https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html-single/virtualization/index

https://docs.redhat.com/en/documentation/red_hat_advanced_cluster_management_for_kubernetes/2.14

https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/

https://www.vmware.com/docs/vmw-cloud-director-datasheets

https://www.vmware.com/docs/vmw-cloud-director-briefing-paper


## Core Architecture Mapping

**VMware VCD → OpenShift Implementation:**
- **Organizations** → PostgreSQL metadata only (logical entities)
- **Virtual Data Centers (VDCs)** → Kubernetes Namespaces (`vdc-{org-name}-{vdc-name}`) + Resource Quotas + Network Policies + PostgreSQL metadata
- **VMs** → OpenShift Virtualization VirtualMachines
- **vApp Templates** → PostgreSQL metadata + OpenShift Virtualization VirtualMachineClusterInstanceTypes
- **Catalogs** → PostgreSQL catalog and template metadata store
- **vApps** → PostgreSQL metadata + OpenShift VirtualMachine orchestration
- **Networks** → User Defined Networks + PostgreSQL network configuration
- **Edge Gateways** → OpenShift Routes + Ingress Controllers + PostgreSQL configuration
- **Multi-tenancy** → Namespace isolation + ACM policy distribution

## Phase 1: MVP Implementation (Self-Service VM Provisioning)

### 1. Authentication & Authorization Layer
```
- JWT token authentication compatible with VCD API
- RBAC mapping organizations to OpenShift namespaces
- Multi-tenant context headers (x-vcloud-authorization)
```

### 2. Core API Endpoints (Minimum Viable)
**Authentication:**
- `POST /api/sessions` - Create session/login
- `DELETE /api/sessions` - Logout
- `GET /api/session` - Get current session info

**Organizations & VDCs:**
- `GET /api/org` - List accessible organizations
- `GET /api/org/{org-id}` - Get organization details
- `GET /api/org/{org-id}/vdcs/query` - Query VDCs in organization
- `GET /api/vdc/{vdc-id}` - Get VDC details

**Catalogs & Templates:**
- `GET /api/org/{org-id}/catalogs/query` - Query catalogs in organization
- `GET /api/catalog/{catalog-id}` - Get catalog details
- `GET /api/catalog/{catalog-id}/catalogItems/query` - Query catalog items
- `GET /api/catalogItem/{item-id}` - Get catalog item (vApp template)
- `POST /api/vdc/{vdc-id}/action/instantiateVAppTemplate` - Create vApp from template

**Virtual Machines:**
- `GET /api/vApp/{vapp-id}/vms/query` - Query VMs in vApp
- `GET /api/vm/{vm-id}` - Get VM details
- `POST /api/vApp/{vapp-id}/vms` - Create VM in vApp
- `PUT /api/vm/{vm-id}` - Update VM configuration
- `DELETE /api/vm/{vm-id}` - Delete VM

**vApp Operations:**
- `GET /api/vdc/{vdc-id}/vApps/query` - Query vApps in VDC
- `GET /api/vApp/{vapp-id}` - Get vApp details
- `DELETE /api/vApp/{vapp-id}` - Delete vApp

**VM Power Operations:**
- `POST /api/vm/{vm-id}/power/action/powerOn` - Power on VM
- `POST /api/vm/{vm-id}/power/action/powerOff` - Power off VM
- `POST /api/vm/{vm-id}/power/action/suspend` - Suspend VM
- `POST /api/vm/{vm-id}/power/action/reset` - Reset VM

### 3. Resource Management
```
- Organizations: OpenShift Namespaces + PostgreSQL org metadata (quotas, settings)
- VDCs: ResourceQuotas + NetworkPolicies + PostgreSQL VDC configuration
- Catalogs: PostgreSQL catalog metadata (name, description, org ownership)
- vApp Templates: PostgreSQL template metadata + OpenShift VirtualMachineClusterInstanceTypes
- vApps: PostgreSQL vApp metadata + OpenShift VirtualMachine orchestration
- Storage policies mapped to StorageClasses
```

### 4. Network Implementation
```
- Default tenant networks using Multus CNI
- Network isolation via UserDefinedNetwork
- Basic connectivity (no advanced edge gateway features in MVP)
```

## Technical Implementation Strategy

### Backend Components:
1. **API Gateway** (Go with Gin/Echo + controller-runtime client)
   - Translate VCD API calls to PostgreSQL queries + Kubernetes API calls
   - Handle authentication and tenant context
   - Implement VCD-compatible response formats
   - PostgreSQL connection pooling and ORM (GORM)
   - controller-runtime client for all Kubernetes API interactions

2. **Resource Controllers** (controller-runtime based)
   - Organization controller: Namespace creation/deletion + PostgreSQL sync
   - VDC controller: ResourceQuota management + PostgreSQL metadata
   - vApp controller: VM orchestration based on PostgreSQL vApp definitions
   - Template controller: Sync between PostgreSQL templates and OpenShift instance types
   - Built using controller-runtime manager and reconciler pattern

3. **Kubernetes Client Layer** (controller-runtime)
   - Unified client for all OpenShift/Kubernetes resource management
   - VirtualMachine, VirtualMachineInstanceType operations
   - Namespace, ResourceQuota, NetworkPolicy management
   - UserDefinedNetwork and networking resource operations

4. **Database Layer** (PostgreSQL)
   - Catalog and template metadata management
   - Organization and VDC configuration storage
   - vApp instance tracking and status
   - User authentication and authorization data

5. **Network Management**
   - Integration with OpenShift OVN-Kubernetes via controller-runtime client
   - Basic network isolation and connectivity

### Database Requirements (PostgreSQL):
**Core Schema Design:**
```sql
-- Organizations (database-only entities)
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    description TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Virtual Data Centers (map to Kubernetes namespaces)
CREATE TABLE vdcs (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    organization_id UUID REFERENCES organizations(id),
    namespace_name VARCHAR(253) UNIQUE, -- Kubernetes namespace: vdc-{org-name}-{vdc-name}
    allocation_model VARCHAR(50), -- PayAsYouGo, AllocationPool, ReservationPool
    cpu_limit INTEGER,
    memory_limit_mb INTEGER,
    storage_limit_mb INTEGER,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Catalogs
CREATE TABLE catalogs (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    organization_id UUID REFERENCES organizations(id),
    description TEXT,
    is_shared BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

-- vApp Templates
CREATE TABLE vapp_templates (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    catalog_id UUID REFERENCES catalogs(id),
    description TEXT,
    vm_instance_type VARCHAR(255), -- OpenShift VirtualMachineInstanceType
    os_type VARCHAR(100),
    cpu_count INTEGER,
    memory_mb INTEGER,
    disk_size_gb INTEGER,
    template_data JSONB, -- Template configuration
    created_at TIMESTAMP DEFAULT NOW()
);

-- vApps (instances)
CREATE TABLE vapps (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    vdc_id UUID REFERENCES vdcs(id),
    template_id UUID REFERENCES vapp_templates(id),
    status VARCHAR(50), -- RESOLVED, DEPLOYED, SUSPENDED, etc.
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Virtual Machines (metadata)
CREATE TABLE vms (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    vapp_id UUID REFERENCES vapps(id),
    vm_name VARCHAR(255), -- OpenShift VM resource name
    namespace VARCHAR(255), -- OpenShift namespace
    status VARCHAR(50),
    cpu_count INTEGER,
    memory_mb INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

```

## Development Phases

**Phase 1 (MVP - 8-12 weeks):**
- PostgreSQL database setup and schema creation
- Basic VM CRUD operations via vApp instantiation
- vApp template support and catalog browsing from PostgreSQL
- Single organization support with PostgreSQL metadata
- Default networking
- Authentication framework with PostgreSQL user storage

**Phase 2 (6-8 weeks):**
- Multi-organization support
- Custom networks and isolation
- Advanced catalog management (upload, versioning)
- Resource quotas and policies

**Phase 3 (8-10 weeks):**
- Advanced networking (edge gateways simulation)
- Snapshot management
- Monitoring and metrics
- Advanced RBAC

## Key Technical Decisions

1. **Use OpenShift Virtualization VirtualMachine CRDs** for VM management
2. **Implement VCD API compatibility layer** rather than extending existing APIs
3. **Use controller-runtime for all Kubernetes API interactions** (client and controllers)
4. **PostgreSQL for VCD-specific metadata** and business logic storage
5. **Leverage ACM for multi-cluster scenarios** (future scalability)
6. **Use Kubernetes-native RBAC** with VCD organization mapping
7. **Implement asynchronous task tracking** using Kubernetes Jobs

## MVP Feature Set for Self-Service Provisioning

✅ **Essential Features:**
- Create vApp from template (instantiateVAppTemplate)
- Browse catalogs and vApp templates
- Start/stop/restart VMs
- Delete VMs and vApps
- List user's VMs and vApps
- Basic resource allocation (CPU/memory/disk)
- Network connectivity within organization
- Authentication and authorization

✅ **Recently Completed CloudAPI Features:**
- **User Management API**: Complete CRUD operations for user accounts
  - `GET /cloudapi/1.0.0/users` - List users with pagination
  - `POST /cloudapi/1.0.0/users` - Create user accounts with validation
  - `GET /cloudapi/1.0.0/users/{id}` - Get user details with entity references
  - `PUT /cloudapi/1.0.0/users/{id}` - Update user accounts with conflict detection
  - `DELETE /cloudapi/1.0.0/users/{id}` - Delete user accounts
- **Organization Management API**: Complete CRUD operations for organizations
  - `GET /cloudapi/1.0.0/orgs` - List organizations with pagination
  - `POST /cloudapi/1.0.0/orgs` - Create organizations with validation
  - `GET /cloudapi/1.0.0/orgs/{id}` - Get organization details with computed fields
  - `PUT /cloudapi/1.0.0/orgs/{id}` - Update organizations with conflict detection
  - `DELETE /cloudapi/1.0.0/orgs/{id}` - Delete organizations (protects Provider org)
- **VMware Cloud Director API Compliance**: Full URN ID format support, entity references, and proper error handling
- **Comprehensive Documentation**: API reference guide and user management guide
- **Security Features**: Password hashing, uniqueness validation, authentication requirements

🔄 **Deferred Features:**
- Advanced networking (edge gateways, NAT, firewall)
- Cross-organization networking
- Snapshots and backup
- Advanced monitoring
- Advanced catalog management (upload, versioning)

## Implementation Details

### Project Structure
```
ssvirt/
├── cmd/
│   ├── api-server/           # Main API server entry point
│   └── controller/           # Controller manager entry point
├── pkg/
│   ├── api/
│   │   ├── handlers/         # HTTP handlers for VCD API endpoints
│   │   ├── middleware/       # Authentication, logging, CORS
│   │   └── types/           # VCD API request/response types
│   ├── controllers/         # Kubernetes controllers
│   │   ├── organization/
│   │   ├── vdc/
│   │   ├── vapp/
│   │   └── template/
│   ├── database/
│   │   ├── models/          # PostgreSQL models (GORM)
│   │   ├── migrations/      # Database schema migrations
│   │   └── repositories/    # Data access layer
│   ├── k8s/
│   │   ├── client/          # controller-runtime client wrapper
│   │   └── resources/       # Kubernetes resource helpers
│   ├── auth/                # Authentication and authorization
│   └── config/              # Configuration management
├── internal/
│   ├── translator/          # VCD ↔ OpenShift resource translation
│   └── validator/           # Request validation logic
├── manifests/
│   ├── crd/                 # Custom Resource Definitions
│   ├── rbac/                # RBAC configuration
│   └── deployment/          # Kubernetes deployment manifests
├── scripts/
│   ├── setup-db.sh          # Database initialization
│   └── generate-certs.sh    # TLS certificate generation
├── test/
│   ├── unit/                # Unit tests
│   ├── integration/         # Integration tests
│   └── e2e/                 # End-to-end tests
├── go.mod
├── go.sum
├── Containerfile
├── Makefile
└── README.md
```

### Dependencies & Build Configuration
**Go Modules (go.mod):**
```go
module github.com/mhrivnak/ssvirt

require (
    k8s.io/api v0.29.0
    k8s.io/client-go v0.29.0
    sigs.k8s.io/controller-runtime v0.17.0
    kubevirt.io/api v1.1.0
    
    github.com/gin-gonic/gin v1.9.1
    github.com/golang-jwt/jwt/v5 v5.2.0
    
    gorm.io/gorm v1.25.5
    gorm.io/driver/postgres v1.5.4
    
    github.com/spf13/viper v1.18.2
    github.com/sirupsen/logrus v1.9.3
    
    github.com/stretchr/testify v1.8.4
    github.com/testcontainers/testcontainers-go v0.26.0
)
```

**Makefile:**
```makefile
.PHONY: build test container-build deploy

build:
	go build -o bin/api-server ./cmd/api-server
	go build -o bin/controller ./cmd/controller

test:
	go test ./...

integration-test:
	go test ./test/integration/...

container-build:
	podman build -t ssvirt:latest .

generate:
	go generate ./...
	controller-gen crd paths=./pkg/api/... output:crd:dir=./manifests/crd

deploy:
	kubectl apply -f manifests/
```

### Testing Strategy
**Unit Tests:**
- Database repository layer tests with testcontainers PostgreSQL
- API handler tests with mock dependencies
- Controller reconciliation logic tests
- VCD ↔ OpenShift translation logic tests

**Integration Tests:**
- Full API endpoint tests against real PostgreSQL + mock Kubernetes
- Controller tests against test Kubernetes cluster
- Database migration and schema validation tests

**End-to-End Tests:**
- Complete VCD API workflow tests
- VM provisioning from template to running state
- Multi-tenant isolation verification

### Configuration Management
**Environment Variables:**
```bash
# Database
DATABASE_URL=postgresql://user:password@localhost:5432/vcd
DATABASE_MAX_CONNECTIONS=25

# Kubernetes
KUBECONFIG=/path/to/kubeconfig
KUBERNETES_NAMESPACE=vcd-system

# API Server
API_PORT=8080
API_TLS_CERT_FILE=/etc/certs/tls.crt
API_TLS_KEY_FILE=/etc/certs/tls.key

# Authentication
JWT_SECRET_KEY=your-secret-key
JWT_TOKEN_EXPIRY=24h

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

**Configuration Struct:**
```go
type Config struct {
    Database struct {
        URL            string
        MaxConnections int
    }
    API struct {
        Port     int
        TLSCert  string
        TLSKey   string
    }
    Auth struct {
        JWTSecret     string
        TokenExpiry   time.Duration
    }
    Kubernetes struct {
        Namespace string
    }
    Log struct {
        Level  string
        Format string
    }
}
```

### Deployment Configuration
**Kubernetes Manifests:**
- Deployment for API server and controller
- Service and Ingress for API exposure
- PostgreSQL StatefulSet or external connection
- RBAC for controller permissions
- ConfigMap for configuration
- Secret for sensitive data (JWT keys, DB credentials)

### Error Handling & Logging
**Error Strategy:**
- Structured errors with VCD-compatible error codes
- Proper HTTP status codes matching VCD API
- Kubernetes controller error handling with retries
- Database transaction rollback on failures

**Logging:**
- Structured logging (JSON) with correlation IDs
- Request/response logging middleware
- Controller reconciliation logging
- Audit logging for administrative actions

### Security Considerations
**Authentication:**
- JWT token-based authentication
- Integration with external identity providers (OIDC)
- API key support for service accounts

**Authorization:**
- Role-based access control matching VCD roles
- Tenant isolation enforcement
- Resource ownership validation

**Network Security:**
- TLS termination at API gateway
- mTLS for internal service communication
- Network policies for pod-to-pod communication

### Development Workflow
**Code Generation:**
- controller-gen for CRD generation
- GORM model generation from database schema
- OpenAPI spec generation from handlers

**Development Tools:**
- Air for hot reloading during development
- golangci-lint for code quality
- Pre-commit hooks for formatting and linting

This plan provides a clear path to implement core VCD functionality using OpenShift technologies, focusing on the essential self-service VM provisioning capabilities while maintaining API compatibility for future expansion.

## Phase 1 Implementation Tasks (Sequential Order for PRs)

### High Priority Foundation (Tasks 1-4)
1. **Project setup** - Initialize Go module, directory structure, Makefile, and basic configuration
2. **Database setup** - PostgreSQL schema creation, migrations, and GORM models  
3. **Authentication framework** - JWT token handling, middleware, and basic user management
4. **Core API server** - Gin setup, basic routing, error handling, and health endpoints

### Medium Priority API Implementation (Tasks 5-12)
5. **Organization & VDC API endpoints** - `GET /api/org`, `GET /api/org/{org-id}`, `GET /api/vdc/{vdc-id}`
6. **Authentication API endpoints** - `POST /api/sessions`, `DELETE /api/sessions`, `GET /api/session`
7. **Catalog API endpoints** - `GET catalogs/query`, `GET catalog details`, `GET catalog items`
8. **vApp Template instantiation** - `POST /api/vdc/{vdc-id}/action/instantiateVAppTemplate`
9. **vApp management endpoints** - `GET vApps/query`, `GET vApp details`, `DELETE vApp`
10. **VM CRUD operations** - `GET VMs`, `POST create VM`, `PUT update VM`, `DELETE VM`
11. **VM power operations** - powerOn, powerOff, suspend, reset actions
12. **Kubernetes client setup** - controller-runtime client for OpenShift Virtualization

### Low Priority Controllers & Networking (Tasks 13-16)
13. **Organization controller** - Namespace creation/deletion and PostgreSQL sync
14. **VDC controller** - ResourceQuota management and PostgreSQL metadata sync
15. **vApp controller** - VM orchestration based on PostgreSQL vApp definitions
16. **Basic networking setup** - Default tenant networks using Multus CNI and UserDefinedNetwork

**Note:** Each task is sized for individual pull requests with clear boundaries and dependencies. The ordering ensures core infrastructure is established before building API endpoints, and controllers come after the API layer is functional.
