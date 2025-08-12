# SSVirt - Self Service Virt

SSVirt is an OpenShift-native implementation of standard APIs, providing self-service virtual machine provisioning using OpenShift Virtualization and OpenShift Advanced Cluster Management.

## Overview

This project re-implements the basic capabilities of VMware Cloud Director using OpenShift technologies:
- Organizations → OpenShift Namespaces with RBAC
- Virtual Data Centers (VDCs) → Resource Quotas + Network Policies  
- VMs → OpenShift Virtualization VirtualMachines
- vApp Templates → PostgreSQL metadata + VirtualMachineClusterInstanceTypes
- Catalogs → PostgreSQL catalog and template metadata store

## Project Structure

```
ssvirt/
├── cmd/
│   ├── api-server/           # Main API server entry point
│   └── controller/           # Controller manager entry point
├── pkg/
│   ├── api/                  # HTTP handlers and API types
│   ├── controllers/          # Kubernetes controllers
│   ├── database/             # PostgreSQL models and migrations
│   ├── k8s/                  # Kubernetes client wrapper
│   ├── auth/                 # Authentication and authorization
│   └── config/               # Configuration management
├── internal/
│   ├── translator/           # VCD ↔ OpenShift resource translation
│   └── validator/            # Request validation logic
├── manifests/                # Kubernetes deployment manifests
├── test/                     # Unit, integration and e2e tests
└── scripts/                  # Setup and utility scripts
```

## Development

### Prerequisites

- Go 1.24+
- PostgreSQL 13+
- OpenShift 4.19+ with OpenShift Virtualization

### Building

```bash
# Build binaries
make build

# Run tests
make test

# Build container image
make container-build

# Format code
make fmt

# Run linter
make lint
```

### Configuration

Configuration is handled via environment variables with `SSVIRT_` prefix or YAML config files:

```yaml
database:
  url: "postgresql://user:password@localhost:5432/ssvirt"
  max_connections: 25
api:
  port: 8080
  tls_cert: "/etc/certs/tls.crt"
  tls_key: "/etc/certs/tls.key"
auth:
  jwt_secret: "your-secret-key"
  token_expiry: "24h"
kubernetes:
  namespace: "ssvirt-system"
log:
  level: "info"
  format: "json"
```

## API Compatibility

SSVirt implements the VMware Cloud Director OpenAPI specification for:
- **Authentication**: Session-based authentication with JWT tokens
- **User Management**: Complete CRUD operations for user accounts
- **Organization Management**: Complete CRUD operations for organizations
- **Role Management**: Role assignment and management
- **VDC Operations**: Virtual Data Center management and operations
- **Catalog Management**: Catalog and vApp template browsing and management
- **vApp Lifecycle**: vApp instantiation, management, and deletion
- **Virtual Machine Operations**: Complete VM CRUD operations and power management

### CloudAPI Endpoints

The following VMware Cloud Director compatible endpoints are available:

**Core Management:**
- `/cloudapi/1.0.0/sessions` - Authentication and session management
- `/cloudapi/1.0.0/users` - User account management (GET, POST, PUT, DELETE)
- `/cloudapi/1.0.0/orgs` - Organization management (GET, POST, PUT, DELETE) 
- `/cloudapi/1.0.0/roles` - Role and permissions management

**Infrastructure:**
- `/cloudapi/1.0.0/vdcs` - Virtual Data Center operations
- `/cloudapi/1.0.0/catalogs` - Catalog and template management
- `/cloudapi/1.0.0/vapps` - vApp lifecycle management
- `/cloudapi/1.0.0/vms` - Virtual machine operations

See [docs/api-reference.md](docs/api-reference.md) for complete API documentation or [docs/openapi.yaml](docs/openapi.yaml) for the OpenAPI specification.

## Development Status

This project is under active development. See [plan.md](plan.md) for detailed implementation roadmap.

## License

See [LICENSE](LICENSE) file for details.