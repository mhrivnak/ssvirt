# SSVirt - OpenShift-Based VMware Cloud Director

SSVirt is an OpenShift-native implementation of VMware Cloud Director (VCD) APIs, providing self-service virtual machine provisioning using OpenShift Virtualization and OpenShift Advanced Cluster Management.

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

- Go 1.21+
- PostgreSQL 13+
- OpenShift 4.14+ with OpenShift Virtualization

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
- Authentication and session management
- Organization and VDC operations  
- Catalog and vApp template browsing
- vApp instantiation and management
- Virtual machine CRUD operations
- VM power state management

## Development Status

This project is under active development. See [plan.md](plan.md) for detailed implementation roadmap.

## License

See [LICENSE](LICENSE) file for details.