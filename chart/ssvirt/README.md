# SSVIRT Helm Chart

This Helm chart deploys SSVIRT (OpenShift-based VMware Cloud Director implementation) on OpenShift/Kubernetes.

## Prerequisites

- OpenShift 4.19+
- OpenShift Virtualization operator installed
- Helm 3.8+
- Persistent storage available (for PostgreSQL)

## Installation

### Quick Start

```bash
# Add the chart repository (if publishing to a repository)
helm repo add ssvirt https://your-chart-repo.com
helm repo update

# Install with default values (embedded PostgreSQL)
helm install my-ssvirt ssvirt/ssvirt \
  --set auth.jwtSecret="your-secure-secret-here" \
  --namespace ssvirt-system \
  --create-namespace
```

### Local Installation

```bash
# Install from local chart directory
helm install my-ssvirt ./chart/ssvirt \
  --set auth.jwtSecret="your-secure-secret-here" \
  --namespace ssvirt-system \
  --create-namespace
```

### Production Installation with External Database

```bash
helm install my-ssvirt ./chart/ssvirt \
  --set auth.jwtSecret="your-secure-jwt-secret" \
  --set postgresql.enabled=false \
  --set externalDatabase.host="postgres.example.com" \
  --set externalDatabase.port=5432 \
  --set externalDatabase.database="ssvirt" \
  --set externalDatabase.username="ssvirt" \
  --set externalDatabase.password="your-db-password" \
  --set route.host="ssvirt.apps.your-cluster.com" \
  --namespace ssvirt-system \
  --create-namespace
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `quay.io/ssvirt/ssvirt` |
| `image.tag` | Container image tag | `latest` |
| `auth.jwtSecret` | JWT secret for authentication | `""` (auto-generated) |
| `postgresql.enabled` | Enable embedded PostgreSQL | `true` |
| `route.enabled` | Enable OpenShift Route | `true` |
| `ingress.enabled` | Enable Ingress (alternative to Route) | `false` |

### API Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `apiServer.enabled` | Enable API server deployment | `true` |
| `apiServer.replicaCount` | Number of API server replicas | `2` |
| `apiServer.resources.requests.cpu` | API server CPU request | `250m` |
| `apiServer.resources.requests.memory` | API server memory request | `256Mi` |
| `apiServer.service.port` | API server service port | `8080` |

### Controller Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.enabled` | Enable controller deployment | `true` |
| `controller.replicaCount` | Number of controller replicas | `1` |
| `controller.resources.requests.cpu` | Controller CPU request | `100m` |
| `controller.resources.requests.memory` | Controller memory request | `128Mi` |

### Database Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Use embedded PostgreSQL | `true` |
| `postgresql.auth.database` | PostgreSQL database name | `ssvirt` |
| `postgresql.auth.username` | PostgreSQL username | `ssvirt` |
| `postgresql.auth.password` | PostgreSQL password | `""` (auto-generated) |
| `postgresql.auth.postgresPassword` | PostgreSQL admin password | `""` (auto-generated) |
| `externalDatabase.host` | External database host | `""` |
| `externalDatabase.port` | External database port | `5432` |

> **Security Note**: PostgreSQL passwords are automatically generated with strong random values when left empty. The generated passwords persist across helm upgrades to prevent service disruption. For production deployments, you may optionally provide explicit passwords, but auto-generation is recommended for better security.

### OpenShift Route Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `route.enabled` | Enable OpenShift Route | `true` |
| `route.host` | Route hostname | `""` (auto-generated) |
| `route.tls.enabled` | Enable TLS termination | `true` |
| `route.tls.termination` | TLS termination type | `edge` |

## Security Considerations

### OpenShift Security Context Constraints (SCCs)

This chart is designed to work with OpenShift's default SCCs:
- Runs as non-root user
- Uses read-only root filesystem where possible
- Drops all capabilities
- Uses seccomp runtime default profile

### RBAC Permissions

The chart creates the following RBAC resources:
- **Organization Controller**: Cluster-wide permissions for namespace and OpenShift Virtualization resource management
- **API Server**: Read-only permissions for resource queries

### Secrets Management

- JWT secrets are stored in Kubernetes Secrets
- Database credentials are managed via Secrets
- Auto-generation of JWT secret if not provided (not recommended for production)

## Monitoring and Observability

### Health Checks

Both API server and controller include:
- Liveness probes on `/healthz`
- Readiness probes on `/readyz`

### Metrics

Enable monitoring with Prometheus:

> **Note**: Requires Prometheus Operator/openshift-monitoring stack to be installed; otherwise skip `serviceMonitor.enabled=true`.

```bash
helm upgrade my-ssvirt ./chart/ssvirt \
  --set monitoring.enabled=true \
  --set monitoring.serviceMonitor.enabled=true
```

## High Availability

### API Server HA

> **Note**: Enabling HPA requires Kubernetes Metrics Server or OpenShift-metrics to be installed for CPU/memory metrics.

```bash
helm upgrade my-ssvirt ./chart/ssvirt \
  --set apiServer.replicaCount=3 \
  --set podDisruptionBudget.enabled=true \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=10
```

### Database HA

For production, use an external highly available PostgreSQL:

```bash
helm upgrade my-ssvirt ./chart/ssvirt \
  --set postgresql.enabled=false \
  --set externalDatabase.host="postgres-ha.example.com" \
  # ... other external DB settings
```

## Upgrading

```bash
# Upgrade to new version
helm upgrade my-ssvirt ./chart/ssvirt \
  --namespace ssvirt-system

# Upgrade with new values
helm upgrade my-ssvirt ./chart/ssvirt \
  --set image.tag="v0.2.0" \
  --namespace ssvirt-system
```

## Uninstallation

```bash
# Uninstall the release
helm uninstall my-ssvirt --namespace ssvirt-system

# Clean up namespace (if desired)
kubectl delete namespace ssvirt-system
```

> **Warning**: `helm uninstall` removes the release objects but persistent volumes, PVCs, and Secrets created by the chart may remain. For complete cleanup:
> 
> ```bash
> # Remove PostgreSQL PVCs (if using embedded PostgreSQL)
> kubectl delete pvc -l app.kubernetes.io/instance=my-ssvirt -n ssvirt-system
> 
> # Remove any remaining secrets (optional)
> kubectl delete secret my-ssvirt-config my-ssvirt-external-db -n ssvirt-system
> ```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n ssvirt-system -l app.kubernetes.io/instance=my-ssvirt
```

### View Logs

```bash
# API Server logs
kubectl logs -n ssvirt-system deployment/my-ssvirt-api-server -f

# Controller logs
kubectl logs -n ssvirt-system deployment/my-ssvirt-controller -f
```

### Database Connection Issues

```bash
# Check database secret
kubectl get secret my-ssvirt-config -n ssvirt-system -o yaml

# Check PostgreSQL status (if using embedded)
kubectl get pods -n ssvirt-system -l app.kubernetes.io/name=postgresql
```

### API Access Issues

```bash
# Check route/ingress
kubectl get route my-ssvirt -n ssvirt-system  # OpenShift
kubectl get ingress my-ssvirt -n ssvirt-system  # Kubernetes

# Test API locally
kubectl port-forward -n ssvirt-system svc/my-ssvirt-api-server 8080:8080
curl http://localhost:8080/healthz
```

## Development

### Template Testing

```bash
# Test template rendering
helm template test-release ./chart/ssvirt \
  --set auth.jwtSecret=test \
  --debug

# Validate chart
helm lint ./chart/ssvirt

# Test installation (dry-run)
helm install test-release ./chart/ssvirt \
  --dry-run --debug \
  --namespace ssvirt-system
```

## Support

For issues and questions:
- GitHub Issues: [https://github.com/mhrivnak/ssvirt/issues](https://github.com/mhrivnak/ssvirt/issues)
- Documentation: [https://github.com/mhrivnak/ssvirt](https://github.com/mhrivnak/ssvirt)