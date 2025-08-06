# SSVIRT User Guide

This guide explains how end users can interact with the SSVIRT system to provision and manage virtual machines through the VMware Cloud Director-compatible API.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Authentication](#authentication)
3. [Understanding Organizations and VDCs](#understanding-organizations-and-vdcs)
4. [Browsing Available Templates](#browsing-available-templates)
5. [Creating Virtual Machines](#creating-virtual-machines)
6. [Managing Virtual Machines](#managing-virtual-machines)
7. [Networking](#networking)
8. [Storage Management](#storage-management)
9. [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)
10. [API Reference](#api-reference)

## Getting Started

### Prerequisites

- Valid user account created by your administrator
- Access to the SSVIRT API endpoint URL
- curl, HTTP client, or compatible tools
- Basic understanding of REST APIs and JSON

### System Overview

SSVIRT provides a VMware Cloud Director-compatible API for managing virtual machines on OpenShift. Key concepts:

- **Organization**: Your logical tenant space (database entity)
- **Virtual Data Center (VDC)**: Physical resource pool with dedicated Kubernetes namespace (`vdc-{org-name}-{vdc-name}`)
- **vApp**: Group of related VMs (logical container)
- **VM**: Individual virtual machine instance
- **Template**: Pre-configured VM image with OS and software already installed, ready for deployment
- **Network**: Isolated network for VM communication

## Authentication

### Login and Session Management

All API interactions require authentication. Start by creating a session:

```bash
# Set your SSVIRT API endpoint
export SSVIRT_URL="https://ssvirt.apps.your-cluster.com"

# Login and get session token
curl -k -X POST $SSVIRT_URL/api/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "username": "your-username",
    "password": "your-password"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "name": "your-username",
    "org": "your-org"
  }
}
```

Save the token for subsequent requests:
```bash
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Session Information

Check your current session:
```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/session
```

### Logout

End your session when finished:
```bash
curl -k -X DELETE -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/sessions
```

## Understanding Organizations and VDCs

### List Your Organizations

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org
```

Response:
```json
{
  "values": [
    {
      "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc",
      "name": "your-org",
      "displayName": "Your Organization",
      "description": "Your organization description"
    }
  ]
}
```

### Get Organization Details

```bash
export ORG_ID="urn:vcloud:org:12345678-1234-1234-1234-123456789abc"

curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org/$ORG_ID
```

### List Virtual Data Centers

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org/$ORG_ID/vdcs/query
```

Response:
```json
{
  "values": [
    {
      "id": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
      "name": "production-vdc",
      "status": "POWERED_ON",
      "computePolicy": {
        "cpuLimit": "10",
        "memoryLimitMb": 20480
      }
    }
  ]
}
```

### Get VDC Details

```bash
export VDC_ID="urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321"

curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vdc/$VDC_ID
```

## Browsing Available Templates

Templates contain pre-configured operating system images with software already
installed. System administrators populate catalogs with templates that users can
deploy. Users select from available templates rather than providing their own OS
images.

### List VM Templates

Browse available VM templates in the catalog:

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/catalogs/query?filter=name==public
```

### Get Template Details

```bash
export TEMPLATE_ID="urn:vcloud:catalogitem:11111111-2222-3333-4444-555555555555"

curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/catalogs/public/templates/$TEMPLATE_ID
```

Response:
```json
{
  "id": "urn:vcloud:catalogitem:11111111-2222-3333-4444-555555555555",
  "name": "rhel9-template",
  "displayName": "Red Hat Enterprise Linux 9",
  "description": "RHEL 9 virtual machine template",
  "osType": "rhel9Server64Guest",
  "defaultInstanceType": "medium"
}
```

### List Instance Types

See available VM sizes:

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/instanceTypes
```

Response:
```json
{
  "values": [
    {
      "name": "small",
      "cpu": {"guest": 1},
      "memory": {"guest": "2Gi"}
    },
    {
      "name": "medium", 
      "cpu": {"guest": 2},
      "memory": {"guest": "4Gi"}
    },
    {
      "name": "large",
      "cpu": {"guest": 4},
      "memory": {"guest": "8Gi"}
    }
  ]
}
```

## Creating Virtual Machines

### Create a vApp Container

First, create a vApp to contain your VMs:

```bash
curl -k -X POST $SSVIRT_URL/api/vdc/$VDC_ID/vapps \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-web-app",
    "description": "Web application environment",
    "powerOn": false
  }'
```

Response:
```json
{
  "id": "urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "my-web-app",
  "status": "POWERED_OFF",
  "href": "/api/vapp/urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
}
```

### Add VM to vApp

Create a VM within the vApp:

```bash
export VAPP_ID="urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

curl -k -X POST $SSVIRT_URL/api/vapp/$VAPP_ID/vms \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "web-server-01",
    "description": "Primary web server",
    "templateRef": "urn:vcloud:catalogitem:11111111-2222-3333-4444-555555555555",
    "instanceType": "medium",
    "powerOn": true,
    "guestCustomization": {
      "enabled": true,
      "computerName": "web-server-01",
      "adminPassword": "SecurePassword123!"
    },
    "networkConfig": {
      "primaryNetworkConnection": {
        "network": "default",
        "ipAddressAllocationMode": "DHCP"
      }
    }
  }'
```

Response:
```json
{
  "id": "urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj",
  "name": "web-server-01",
  "status": "POWERED_OFF",
  "href": "/api/vapp/urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/vm/urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj"
}
```

### Direct VM Creation (Alternative)

You can also create a VM directly without a vApp:

```bash
curl -k -X POST $SSVIRT_URL/api/vdc/$VDC_ID/vms \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "database-server",
    "description": "PostgreSQL database server",
    "templateRef": "urn:vcloud:catalogitem:11111111-2222-3333-4444-555555555555",
    "instanceType": "large",
    "storageProfile": {
      "diskSizeGb": 50
    }
  }'
```

### Advanced VM Configuration

Create a VM with custom specifications:

```bash
curl -k -X POST $SSVIRT_URL/api/vdc/$VDC_ID/vms \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "custom-vm",
    "templateRef": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
    "computeConfig": {
      "cpuCount": 4,
      "memoryMb": 8192
    },
    "storageProfile": {
      "diskSizeGb": 100,
      "storageClass": "fast-ssd"
    },
    "networkConfig": {
      "primaryNetworkConnection": {
        "network": "production-network",
        "ipAddressAllocationMode": "STATIC",
        "ipAddress": "192.168.100.50"
      }
    },
    "guestCustomization": {
      "enabled": true,
      "computerName": "custom-vm",
      "adminPassword": "VerySecurePassword123!",
      "customizationScript": "#!/bin/bash\necho \"VM initialized\" > /tmp/init.log"
    }
  }'
```

## Managing Virtual Machines

### List Your VMs

Get all VMs in your organization:

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vms/query
```

Get VMs in specific VDC:
```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vdc/$VDC_ID/vms
```

### Get VM Details

```bash
export VM_ID="urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj"

curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID
```

### Power Operations

**Power On VM:**
```bash
curl -k -X POST $SSVIRT_URL/api/vm/$VM_ID/power/action/powerOn \
  -H "Authorization: Bearer $TOKEN"
```

**Power Off VM:**
```bash
curl -k -X POST $SSVIRT_URL/api/vm/$VM_ID/power/action/powerOff \
  -H "Authorization: Bearer $TOKEN"
```

**Reboot VM:**
```bash
curl -k -X POST $SSVIRT_URL/api/vm/$VM_ID/power/action/reboot \
  -H "Authorization: Bearer $TOKEN"
```

**Suspend VM:**
```bash
curl -k -X POST $SSVIRT_URL/api/vm/$VM_ID/power/action/suspend \
  -H "Authorization: Bearer $TOKEN"
```

### Update VM Configuration

**Resize VM:**
```bash
curl -k -X PUT $SSVIRT_URL/api/vm/$VM_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "computeConfig": {
      "cpuCount": 4,
      "memoryMb": 8192
    }
  }'
```

**Add Disk:**
```bash
curl -k -X POST $SSVIRT_URL/api/vm/$VM_ID/disks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sizeGb": 20,
    "storageClass": "standard"
  }'
```

### Delete VM

```bash
curl -k -X DELETE $SSVIRT_URL/api/vm/$VM_ID \
  -H "Authorization: Bearer $TOKEN"
```

## Networking

### List Available Networks

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org/$ORG_ID/networks
```

### Connect VM to Network

```bash
curl -k -X POST $SSVIRT_URL/api/vm/$VM_ID/networkConnections \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "network": "production-network",
    "ipAddressAllocationMode": "DHCP",
    "isConnected": true,
    "isPrimary": false
  }'
```

### Update Network Configuration

```bash
curl -k -X PUT $SSVIRT_URL/api/vm/$VM_ID/networkConnections/0 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ipAddressAllocationMode": "STATIC",
    "ipAddress": "192.168.100.100",
    "netmask": "255.255.255.0",
    "gateway": "192.168.100.1"
  }'
```

## Storage Management

### List VM Disks

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID/disks
```

### Resize Disk

```bash
export DISK_ID="disk-001"

curl -k -X PUT $SSVIRT_URL/api/vm/$VM_ID/disks/$DISK_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sizeGb": 100
  }'
```

### Remove Disk

```bash
curl -k -X DELETE $SSVIRT_URL/api/vm/$VM_ID/disks/$DISK_ID \
  -H "Authorization: Bearer $TOKEN"
```

## Monitoring and Troubleshooting

### Check VM Status

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID/status
```

### Get VM Console Access

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID/console
```

Response:
```json
{
  "consoleUrl": "https://console.apps.cluster.com/vm/console?token=...",
  "protocol": "vnc"
}
```

### View VM Metrics

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID/metrics
```

### Troubleshooting Common Issues

**VM Won't Start:**
1. Check VDC resource quotas
2. Verify template availability
3. Check network configuration

```bash
# Check resource usage
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vdc/$VDC_ID/usage

# Check events
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID/events
```

**Network Issues:**
```bash
# Verify network connectivity
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID/networkConnections

# Check network policies
curl -k -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org/$ORG_ID/networks/$NETWORK_ID
```

## API Reference

### Base URL Structure

All API endpoints follow this pattern:
```
https://your-ssvirt-domain.com/api/{resource}/{id?}/{action?}
```

### Common Headers

```
Authorization: Bearer {jwt-token}
Content-Type: application/json
Accept: application/json
```

### Response Codes

- `200 OK` - Successful operation
- `201 Created` - Resource created successfully
- `202 Accepted` - Asynchronous operation accepted
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication required
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict
- `500 Internal Server Error` - Server error

### Error Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid instance type specified",
    "details": [
      "Instance type 'xlarge' is not available in this VDC"
    ]
  }
}
```

### Pagination

List endpoints support pagination:

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/api/vms/query?page=1&pageSize=25&sortBy=name&sortOrder=asc"
```

### Filtering

Use OData-style filters:

```bash
# Filter VMs by status
curl -k -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/api/vms/query?filter=status==POWERED_ON"

# Multiple filters
curl -k -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/api/vms/query?filter=status==POWERED_ON;name==*web*"
```

## Best Practices

1. **Session Management**: Always logout when finished to prevent token leakage
2. **Error Handling**: Check response codes and handle errors gracefully
3. **Resource Cleanup**: Delete unused VMs and vApps to conserve resources
4. **Security**: Use strong passwords and keep credentials secure
5. **Monitoring**: Regularly check VM status and resource usage
6. **Backup**: Implement backup strategies for important VM data
7. **Networks**: Use appropriate networks for security isolation
8. **Templates**: Choose the right template and instance type for your workload

## Example Workflows

### Complete VM Creation Workflow

```bash
#!/bin/bash
set -e

# Configuration
SSVIRT_URL="https://ssvirt.apps.your-cluster.com"
USERNAME="your-username"
PASSWORD="your-password"

# 1. Login
echo "Logging in..."
TOKEN=$(curl -sk -X POST $SSVIRT_URL/api/sessions \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" | \
  jq -r '.token')

# 2. Get organization and VDC
echo "Getting organization info..."
ORG_ID=$(curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org | jq -r '.values[0].id')

VDC_ID=$(curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/org/$ORG_ID/vdcs/query | jq -r '.values[0].id')

# 3. Create vApp
echo "Creating vApp..."
VAPP_RESPONSE=$(curl -sk -X POST $SSVIRT_URL/api/vdc/$VDC_ID/vapps \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "demo-app",
    "description": "Demo application"
  }')

VAPP_ID=$(echo $VAPP_RESPONSE | jq -r '.id')

# 4. Create VM
echo "Creating VM..."
VM_RESPONSE=$(curl -sk -X POST $SSVIRT_URL/api/vapp/$VAPP_ID/vms \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "demo-vm",
    "templateRef": "urn:vcloud:catalogitem:11111111-2222-3333-4444-555555555555",
    "instanceType": "medium",
    "powerOn": true
  }')

VM_ID=$(echo $VM_RESPONSE | jq -r '.id')

# 5. Wait for VM to be ready
echo "Waiting for VM to start..."
while true; do
  STATUS=$(curl -sk -H "Authorization: Bearer $TOKEN" \
    $SSVIRT_URL/api/vm/$VM_ID | jq -r '.status')
  
  if [ "$STATUS" = "POWERED_ON" ]; then
    echo "VM is running!"
    break
  fi
  
  echo "Current status: $STATUS"
  sleep 10
done

# 6. Get VM details
echo "VM Details:"
curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/vm/$VM_ID | jq '.'

# 7. Logout
curl -sk -X DELETE -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/api/sessions

echo "Demo complete!"
```

For more information and advanced usage examples, consult your system administrator or refer to the SSVIRT API documentation.