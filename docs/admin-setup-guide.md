# Administrator Setup Guide

This guide provides detailed instructions for OpenShift administrators to configure the system after installing the SSVIRT Helm chart, preparing it for end users to log in and provision virtual machines.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Post-Installation Verification](#post-installation-verification)
3. [Organization and VDC Setup](#organization-and-vdc-setup)
4. [Network Configuration](#network-configuration)
5. [VM Templates and Instance Types](#vm-templates-and-instance-types)
6. [User Authentication and RBAC](#user-authentication-and-rbac)
7. [Storage Configuration](#storage-configuration)
8. [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)
9. [Security Considerations](#security-considerations)

## Prerequisites

Before proceeding with this guide, ensure the following components are installed and configured:

- OpenShift 4.19+ cluster with cluster-admin privileges
- OpenShift Virtualization operator installed and configured
- SSVIRT Helm chart successfully deployed
- PostgreSQL database accessible (embedded or external)
- DNS resolution configured for the SSVIRT route/ingress

## Post-Installation Verification

### 1. Verify Pod Status

Check that all SSVIRT components are running:

```bash
# Check pod status
oc get pods -n ssvirt-system

# Verify logs for any errors
oc logs -n ssvirt-system deployment/ssvirt-api-server
oc logs -n ssvirt-system deployment/ssvirt-controller
```

### 2. Verify Database Connectivity

Ensure the database is accessible and initialized:

```bash
# Check database connection from API server
oc exec -n ssvirt-system deployment/ssvirt-api-server -- \
  curl -f http://localhost:8080/healthz

# Verify database tables are created
oc exec -n ssvirt-system deployment/ssvirt-api-server -- \
  psql $DATABASE_URL -c "\dt"
```

### 3. Test API Endpoint Access

Verify the API is accessible through the configured route or ingress:

```bash
# Get the route URL
SSVIRT_URL=$(oc get route ssvirt -n ssvirt-system -o jsonpath='{.spec.host}')

# Test API connectivity
curl -k https://$SSVIRT_URL/api/versions
```

## Organization and VDC Setup

Organizations in SSVIRT map to OpenShift namespaces with associated metadata stored in PostgreSQL. Each organization can contain multiple Virtual Data Centers (VDCs).

### 1. Create an Organization

Organizations must be created through the SSVIRT API. Use the following steps:

```bash
# Create a session token (admin authentication)
ADMIN_TOKEN=$(curl -k -X POST https://$SSVIRT_URL/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}' | \
  jq -r '.token')

# Create an organization
curl -k -X POST https://$SSVIRT_URL/api/orgs \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "example-org",
    "displayName": "Example Organization",
    "description": "Example organization for testing"
  }'
```

### 2. Create a Virtual Data Center (VDC)

VDCs define resource quotas and network policies within an organization:

```bash
# Get organization ID
ORG_ID=$(curl -k -H "Authorization: Bearer $ADMIN_TOKEN" \
  https://$SSVIRT_URL/api/orgs | jq -r '.values[] | select(.name=="example-org") | .id')

# Create a VDC
curl -k -X POST https://$SSVIRT_URL/api/orgs/$ORG_ID/vdcs \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "example-vdc",
    "displayName": "Example VDC",
    "description": "Example VDC for VM provisioning",
    "computePolicy": {
      "cpuLimit": "10",
      "memoryLimitMb": 20480,
      "storageQuotaGb": 100
    },
    "networkQuota": 5
  }'
```

### 3. Verify Namespace Creation

After creating an organization, verify the corresponding OpenShift namespace was created:

```bash
# Check namespace creation
oc get namespace example-org

# Verify resource quotas are applied
oc get resourcequota -n example-org

# Check network policies
oc get networkpolicy -n example-org
```

## Network Configuration

SSVIRT uses OpenShift User Defined Networks (UDNs) for VM networking isolation.

### 1. Configure User Defined Networks

Create network configurations that VMs can use:

```bash
# Create a user defined network for the organization
cat <<EOF | oc apply -f -
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: example-org-network
  namespace: example-org
spec:
  topology: Layer2
  layer2:
    role: Primary
    subnets:
    - "192.168.100.0/24"
EOF
```

### 2. Verify Network Configuration

```bash
# Check UDN status
oc get userdefinednetwork -n example-org

# Verify network is ready
oc describe userdefinednetwork example-org-network -n example-org
```

## VM Templates and Instance Types

Set up VM templates and instance types that users can select when provisioning
VMs. Templates contain pre-built OS images with software already installed -
users do not provide their own OS images but select from administrator-provided
templates in catalogs.

### 1. Create VirtualMachineClusterInstanceTypes

Define compute resources for different VM sizes:

```bash
# Create small instance type
cat <<EOF | oc apply -f -
apiVersion: instancetype.kubevirt.io/v1beta1
kind: VirtualMachineClusterInstanceType
metadata:
  name: small
spec:
  cpu:
    guest: 1
  memory:
    guest: 2Gi
EOF

# Create medium instance type
cat <<EOF | oc apply -f -
apiVersion: instancetype.kubevirt.io/v1beta1
kind: VirtualMachineClusterInstanceType
metadata:
  name: medium
spec:
  cpu:
    guest: 2
  memory:
    guest: 4Gi
EOF

# Create large instance type
cat <<EOF | oc apply -f -
apiVersion: instancetype.kubevirt.io/v1beta1
kind: VirtualMachineClusterInstanceType
metadata:
  name: large
spec:
  cpu:
    guest: 4
  memory:
    guest: 8Gi
EOF
```

### 2. Configure VM Boot Sources

Create bootable disk images for common operating systems:

```bash
# Create RHEL 9 boot source
cat <<EOF | oc apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: rhel9-bootsource
  namespace: openshift-virtualization-os-images
  labels:
    instancetype.kubevirt.io/default-instancetype: medium
    instancetype.kubevirt.io/default-preference: rhel.9
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: ocs-storagecluster-ceph-rbd
  dataSource:
    name: rhel9
    kind: DataSource
    apiGroup: cdi.kubevirt.io
EOF

# Create Ubuntu 22.04 boot source
cat <<EOF | oc apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ubuntu2204-bootsource
  namespace: openshift-virtualization-os-images
  labels:
    instancetype.kubevirt.io/default-instancetype: medium
    instancetype.kubevirt.io/default-preference: ubuntu
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: ocs-storagecluster-ceph-rbd
  dataSource:
    name: ubuntu2204
    kind: DataSource
    apiGroup: cdi.kubevirt.io
EOF
```

### 3. Register Templates in SSVIRT

Add the VM templates to the SSVIRT catalog:

```bash
# Register RHEL 9 template
curl -k -X POST https://$SSVIRT_URL/api/catalogs/public/templates \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "rhel9-template",
    "displayName": "Red Hat Enterprise Linux 9",
    "description": "RHEL 9 virtual machine template",
    "osType": "rhel9Server64Guest",
    "bootImageRef": "rhel9-bootsource",
    "defaultInstanceType": "medium"
  }'

# Register Ubuntu template
curl -k -X POST https://$SSVIRT_URL/api/catalogs/public/templates \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ubuntu2204-template",
    "displayName": "Ubuntu 22.04 LTS",
    "description": "Ubuntu 22.04 LTS virtual machine template",
    "osType": "ubuntu64Guest",
    "bootImageRef": "ubuntu2204-bootsource",
    "defaultInstanceType": "medium"
  }'
```

## User Authentication and RBAC

Configure user authentication and role-based access control for organizations.

### 1. Create Organization Users

Users must be created and assigned to organizations:

```bash
# Create a user
curl -k -X POST https://$SSVIRT_URL/api/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "email": "user1@example.com",
    "password": "secure-password",
    "enabled": true
  }'

# Assign user to organization with appropriate role
curl -k -X POST https://$SSVIRT_URL/api/orgs/$ORG_ID/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "role": "vdc-user"
  }'
```

### 2. Configure OpenShift RBAC

Ensure the organization namespace has proper RBAC for VM management:

```bash
# Create role for VM operations
cat <<EOF | oc apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: example-org
  name: vm-user
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines", "virtualmachineinstances"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "create", "delete"]
- apiGroups: [""]
  resources: ["secrets", "configmaps"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
EOF

# Bind role to organization service account
oc create rolebinding vm-user-binding \
  --role=vm-user \
  --serviceaccount=example-org:default \
  -n example-org
```

## Storage Configuration

Configure storage classes and policies for VM disk provisioning.

### 1. Verify Storage Classes

Ensure appropriate storage classes are available:

```bash
# List available storage classes
oc get storageclass

# Set default storage class if needed
oc annotate storageclass <storage-class-name> \
  storageclass.kubernetes.io/is-default-class=true
```

### 2. Configure Storage Quotas

Set storage quotas for organizations:

```bash
# Update resource quota to include storage limits
oc patch resourcequota -n example-org compute-quota --type='merge' -p='{
  "spec": {
    "hard": {
      "requests.storage": "500Gi",
      "persistentvolumeclaims": "20"
    }
  }
}'
```

## Monitoring and Troubleshooting

Set up monitoring and establish troubleshooting procedures.

### 1. Monitor System Health

```bash
# Check API server metrics
curl -k https://$SSVIRT_URL/metrics

# Monitor database connections
oc exec -n ssvirt-system deployment/ssvirt-api-server -- \
  netstat -an | grep :5432

# Check resource usage
oc top pods -n ssvirt-system
```

### 2. Common Troubleshooting Steps

```bash
# Check for failed VM provisioning
oc get events -n example-org --field-selector type=Warning

# Verify OpenShift Virtualization status
oc get hyperconverged -n openshift-cnv

# Check network connectivity
oc get userdefinednetwork --all-namespaces
```

### 3. Log Analysis

```bash
# API server logs
oc logs -n ssvirt-system deployment/ssvirt-api-server -f

# Controller logs
oc logs -n ssvirt-system deployment/ssvirt-controller -f

# Filter for specific errors
oc logs -n ssvirt-system deployment/ssvirt-api-server | grep ERROR
```

## Security Considerations

### 1. Network Security

- Ensure network policies are properly configured to isolate organization namespaces
- Verify that inter-organization communication is blocked
- Configure egress policies for VM network access

### 2. Secrets Management

```bash
# Rotate JWT secrets periodically
oc patch secret ssvirt-secret -n ssvirt-system -p='{"data":{"jwt-secret":"'$(openssl rand -base64 32)'"}}'

# Restart API server to pick up new secret
oc rollout restart deployment/ssvirt-api-server -n ssvirt-system
```

⚠️ **Warning**: Rotating the JWT secret will invalidate all existing user session tokens. Users will need to re-authenticate after the secret rotation. Plan this operation during maintenance windows and notify users in advance.

### 3. Database Security

- Use encrypted connections to external PostgreSQL databases
- Regularly update database passwords
- Enable database audit logging

### 4. Image Security

- Scan VM boot images for vulnerabilities
- Use trusted image sources only
- Implement image signing and verification

## Validation Checklist

After completing the setup, verify the following:

- [ ] All SSVIRT pods are running and healthy
- [ ] Database connectivity is working
- [ ] API endpoints respond correctly
- [ ] At least one organization and VDC are created
- [ ] Network policies are applied to organization namespaces
- [ ] VM instance types are available
- [ ] VM templates are registered and accessible
- [ ] Test user can authenticate and access their organization
- [ ] Test VM can be provisioned successfully
- [ ] Storage quotas are enforced
- [ ] Monitoring and logging are functional

## Next Steps

Once the system is configured:

1. **User Training**: Provide end users with access credentials and usage documentation
2. **Backup Strategy**: Implement regular backups of the PostgreSQL database
3. **Monitoring**: Set up alerting for system health and resource usage
4. **Scaling**: Monitor resource usage and scale components as needed
5. **Updates**: Plan for regular updates of SSVIRT components and dependencies

For additional support and documentation, refer to:
- OpenShift Virtualization documentation
- SSVIRT API reference
- OpenShift networking guides
- Kubernetes RBAC documentation