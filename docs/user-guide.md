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
curl -i -u admin -X POST $SSVIRT_URL/cloudapi/1.0.0/sessions
```

Response headers:
```
HTTP/1.1 200 OK
server: nginx/1.26.3
date: Tue, 12 Aug 2025 14:23:29 GMT
content-type: application/json; charset=utf-8
content-length: 613
access-control-allow-credentials: true
access-control-allow-headers: Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization
access-control-allow-methods: GET, POST, PUT, DELETE, OPTIONS
access-control-allow-origin: *
access-control-expose-headers: Content-Length
authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidXJuOnZjbG91ZDp1c2VyOjViZWNmYjFjLWVmMGYtNDk0Ni1hMzA0LTlkY2YwYzk2OTg0MSIsInVzZXJuYW1lIjoiYWRtaW4iLCJzZXNzaW9uX2lkIjoidXJuOnZjbG91ZDpzZXNzaW9uOjViNDg2NzQ2LTgxZWQtNDNhNi05YTQwLTYxNTAzNGQxOTE5NyIsImV4cCI6MTc1NTA5NTAwOSwibmJmIjoxNzU1MDA4NjA5LCJpYXQiOjE3NTUwMDg2MDl9.jpB1GS-D6LZ2nwUeSsvkNvZZr0MW0E6hola3mMHlsco
set-cookie: d458673311421c8f051987738f42631d=062eebc690264a271b87ed1e3f176ac6; path=/; HttpOnly; Secure; SameSite=None
```

Response body:
```json
{
  "id": "urn:vcloud:session:94235a1c-6dc8-452c-af0c-5eb05e213958",
  "site": {
    "name": "SSVirt Provider",
    "id": "urn:vcloud:site:00000000-0000-0000-0000-000000000001"
  },
  "user": {
    "name": "admin",
    "id": "urn:vcloud:user:5becfb1c-ef0f-4946-a304-9dcf0c969841"
  },
  "org": {
    "name": "Provider",
    "id": "urn:vcloud:org:9f372aca-56ce-4c4c-bf52-6582fe4b5c44"
  },
  "operatingOrg": {
    "name": "Provider",
    "id": "urn:vcloud:org:9f372aca-56ce-4c4c-bf52-6582fe4b5c44"
  },
  "location": "us-west-1",
  "roles": [
    "System Administrator"
  ],
  "roleRefs": [
    {
      "name": "System Administrator",
      "id": "urn:vcloud:role:9443e3b2-6cc4-4007-bcae-ea0adee976f9"
    }
  ],
  "sessionIdleTimeoutMinutes": 30
}
```

Save the token for subsequent requests:
```bash
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Session Information

Check your current session:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/cloudapi/1.0.0/sessions/urn:vcloud:session:5b486746-81ed-43a6-9a40-615034d19197"
```

Response
```json
{
  "id": "urn:vcloud:session:5b486746-81ed-43a6-9a40-615034d19197",
  "site": {
    "name": "SSVirt Provider",
    "id": "urn:vcloud:site:00000000-0000-0000-0000-000000000001"
  },
  "user": {
    "name": "admin",
    "id": "urn:vcloud:user:5becfb1c-ef0f-4946-a304-9dcf0c969841"
  },
  "org": {
    "name": "Provider",
    "id": "urn:vcloud:org:9f372aca-56ce-4c4c-bf52-6582fe4b5c44"
  },
  "operatingOrg": {
    "name": "Provider",
    "id": "urn:vcloud:org:9f372aca-56ce-4c4c-bf52-6582fe4b5c44"
  },
  "location": "us-west-1",
  "roles": [
    "System Administrator"
  ],
  "roleRefs": [
    {
      "name": "System Administrator",
      "id": "urn:vcloud:role:9443e3b2-6cc4-4007-bcae-ea0adee976f9"
    }
  ],
  "sessionIdleTimeoutMinutes": 30
}
```

### Logout

End your session when finished:
```bash
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/cloudapi/1.0.0/sessions/urn:vcloud:session:5b486746-81ed-43a6-9a40-615034d19197"
```

## Understanding Organizations and VDCs

### List Your Organizations

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/orgs
```

Response:
```json
{
  "resultTotal": 1,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc",
      "name": "your-org",
      "displayName": "Your Organization",
      "description": "Your organization description",
      "isEnabled": true,
      "orgVdcCount": 2,
      "catalogCount": 1,
      "vappCount": 3,
      "runningVMCount": 5,
      "userCount": 10,
      "diskCount": 15,
      "canManageOrgs": false,
      "canPublish": true,
      "directlyManagedOrgCount": 0,
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-20T14:45:00Z"
    }
  ]
}
```

### Get Organization Details

```bash
export ORG_ID="urn:vcloud:org:12345678-1234-1234-1234-123456789abc"

curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/orgs/$ORG_ID
```

Response:
```json
{
  "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc",
  "name": "your-org",
  "displayName": "Your Organization", 
  "description": "Your organization description",
  "isEnabled": true,
  "orgVdcCount": 2,
  "catalogCount": 1,
  "vappCount": 3,
  "runningVMCount": 5,
  "userCount": 10,
  "diskCount": 15,
  "canManageOrgs": false,
  "canPublish": true,
  "directlyManagedOrgCount": 0,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-20T14:45:00Z"
}
```

### List Virtual Data Centers

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vdcs
```

Response:
```json
{
  "resultTotal": 2,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
      "name": "production-vdc",
      "description": "Production environment VDC",
      "orgEntityRef": {
        "name": "your-org",
        "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc"
      }
    }
  ]
}
```

### Get VDC Details

```bash
export VDC_ID="urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321"

curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID
```

Response:
```json
{
  "id": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
  "name": "production-vdc",
  "description": "Production environment VDC",
  "orgEntityRef": {
    "name": "your-org",
    "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc"
  }
}
```

## Browsing Available Templates

Templates contain pre-configured operating system images with software already
installed. System administrators populate catalogs with templates that users can
deploy. Users select from available templates rather than providing their own OS
images.

### List Catalogs

Browse available catalogs:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/catalogs
```

Response:
```json
{
  "resultTotal": 1,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:catalog:11111111-2222-3333-4444-555555555555",
      "name": "public",
      "description": "Public catalog with VM templates",
      "org": {
        "name": "Provider",
        "id": "urn:vcloud:org:9f372aca-56ce-4c4c-bf52-6582fe4b5c44"
      },
      "isPublished": true,
      "isSubscribed": false,
      "numberOfVAppTemplates": 5,
      "numberOfMedia": 0,
      "isLocal": true,
      "version": 1
    }
  ]
}
```

### List Catalog Items (VM Templates)

Get available VM templates from a catalog:

```bash
export CATALOG_ID="urn:vcloud:catalog:11111111-2222-3333-4444-555555555555"

curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/catalogs/$CATALOG_ID/catalogItems
```

Response:
```json
{
  "resultTotal": 3,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
      "name": "rhel9-template",
      "description": "Red Hat Enterprise Linux 9 template",
      "catalogId": "urn:vcloud:catalog:11111111-2222-3333-4444-555555555555",
      "isPublished": true,
      "status": "RESOLVED",
      "entity": {
        "name": "rhel9-template",
        "description": "Red Hat Enterprise Linux 9 template",
        "type": "vAppTemplate",
        "numberOfVMs": 1,
        "numberOfCpus": 2,
        "memoryAllocation": 4096,
        "storageAllocation": 20480
      },
      "owner": {
        "name": "admin",
        "id": "urn:vcloud:user:5becfb1c-ef0f-4946-a304-9dcf0c969841"
      },
      "catalog": {
        "name": "public",
        "id": "urn:vcloud:catalog:11111111-2222-3333-4444-555555555555"
      }
    }
  ]
}
```

### Get Template Details

```bash
export TEMPLATE_ID="urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666"

curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/catalogs/$CATALOG_ID/catalogItems/$TEMPLATE_ID
```

Response:
```json
{
  "id": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
  "name": "rhel9-template",
  "description": "Red Hat Enterprise Linux 9 template",
  "catalogId": "urn:vcloud:catalog:11111111-2222-3333-4444-555555555555",
  "isPublished": true,
  "status": "RESOLVED",
  "entity": {
    "name": "rhel9-template",
    "description": "Red Hat Enterprise Linux 9 template",
    "type": "vAppTemplate",
    "numberOfVMs": 1,
    "numberOfCpus": 2,
    "memoryAllocation": 4096,
    "storageAllocation": 20480
  },
  "owner": {
    "name": "admin",
    "id": "urn:vcloud:user:5becfb1c-ef0f-4946-a304-9dcf0c969841"
  },
  "catalog": {
    "name": "public",
    "id": "urn:vcloud:catalog:11111111-2222-3333-4444-555555555555"
  }
}
```

## Creating Virtual Machines

### Create a vApp from Template

Create a vApp (virtual application) from a catalog template:

```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID/actions/instantiateTemplate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-web-app",
    "description": "Web application environment",
    "catalogItem": {
      "id": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
      "name": "rhel9-template"
    }
  }'
```

Response:
```json
{
  "id": "urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "my-web-app",
  "description": "Web application environment",
  "status": "INSTANTIATING",
  "vdcId": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
  "templateId": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
  "createdAt": "2024-01-15T10:30:00Z",
  "numberOfVMs": 1,
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
}
```

### List vApps in VDC

List all vApps in a specific VDC:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID/vapps
```

Response:
```json
{
  "resultTotal": 2,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "name": "my-web-app",
      "description": "Web application environment",
      "status": "RESOLVED",
      "vdcId": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
      "templateId": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
      "createdAt": "2024-01-15T10:30:00Z",
      "numberOfVMs": 1,
      "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    }
  ]
}
```

### Get vApp Details

Get detailed information about a specific vApp including VMs:

```bash
export VAPP_ID="urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vapps/$VAPP_ID
```

Response:
```json
{
  "id": "urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "my-web-app",
  "description": "Web application environment",
  "status": "RESOLVED",
  "vdcId": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
  "templateId": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:32:00Z",
  "numberOfVMs": 1,
  "vms": [
    {
      "id": "urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj",
      "name": "my-web-app-vm",
      "status": "POWERED_ON",
      "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj"
    }
  ],
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
}
```

## Managing Virtual Machines

### Get VM Details

Get detailed information about a specific virtual machine:

```bash
export VM_ID="urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj"

curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vms/$VM_ID
```

Response:
```json
{
  "id": "urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj",
  "name": "my-web-app-vm",
  "description": "Virtual machine my-web-app-vm",
  "status": "POWERED_ON",
  "vappId": "urn:vcloud:vapp:aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "templateId": "urn:vcloud:catalogitem:22222222-3333-4444-5555-666666666666",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:32:00Z",
  "guestOs": "Ubuntu Linux (64-bit)",
  "vmTools": {
    "status": "RUNNING",
    "version": "12.1.5"
  },
  "hardware": {
    "numCpus": 2,
    "numCoresPerSocket": 1,
    "memoryMB": 4096
  },
  "storageProfile": {
    "name": "default-storage-policy",
    "href": "/cloudapi/1.0.0/storageProfiles/default-storage-policy"
  },
  "networkConnections": [
    {
      "networkName": "default-network",
      "ipAddress": "192.168.1.100",
      "macAddress": "00:50:56:12:34:56",
      "connected": true
    }
  ],
  "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:ffffffff-gggg-hhhh-iiii-jjjjjjjjjjjj"
}
```

### Delete vApp (and its VMs)

To delete virtual machines, you delete the vApp that contains them:

```bash
curl -X DELETE $SSVIRT_URL/cloudapi/1.0.0/vapps/$VAPP_ID \
  -H "Authorization: Bearer $TOKEN"
```

If the vApp contains running VMs, you can force deletion:

```bash
curl -X DELETE "$SSVIRT_URL/cloudapi/1.0.0/vapps/$VAPP_ID?force=true" \
  -H "Authorization: Bearer $TOKEN"
```

## Monitoring and Troubleshooting

### Check VM Status

VM status is available through the VM details endpoint:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vms/$VM_ID
```

The response includes comprehensive VM information including status, hardware configuration, and network details.

### Troubleshooting Common Issues

**vApp Won't Instantiate:**
1. Check catalog item exists and is accessible
2. Verify VDC access permissions  
3. Ensure template name follows DNS-1123 format
4. Check for name conflicts in VDC

```bash
# Check available templates
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/catalogs/$CATALOG_ID/catalogItems

# Check VDC access
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID

# Check for existing vApps with same name
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID/vapps
```

**Access Denied Errors:**
```bash
# Verify organization membership
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/orgs

# Check session details
curl -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/sessions/$SESSION_ID
```

## API Reference

### Base URL Structure

CloudAPI endpoints follow this pattern:
```
https://your-ssvirt-domain.com/cloudapi/1.0.0/{resource}/{id?}/{action?}
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
curl -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/cloudapi/1.0.0/orgs?page=1&pageSize=25"
```

Response format includes pagination metadata:
```json
{
  "resultTotal": 50,
  "pageCount": 2,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [...]
}
```

### Filtering

Some endpoints support filtering:

```bash
# Filter vApps by status  
curl -H "Authorization: Bearer $TOKEN" \
  "$SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID/vapps?filter=status==RESOLVED"
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

### Complete vApp Creation Workflow

```bash
#!/bin/bash
set -e

# Configuration
SSVIRT_URL="https://ssvirt.apps.your-cluster.com"
USERNAME="admin"
PASSWORD="your-password"

# 1. Login
echo "Logging in..."
LOGIN_RESPONSE=$(curl -sk -u "$USERNAME" -X POST $SSVIRT_URL/cloudapi/1.0.0/sessions)
TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.id')

# Extract session info
SESSION_ID=$(echo $LOGIN_RESPONSE | jq -r '.id')
ORG_ID=$(echo $LOGIN_RESPONSE | jq -r '.org.id')

# 2. Get available VDC
echo "Getting VDC info..."
VDC_ID=$(curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vdcs | jq -r '.values[0].id')

# 3. Get available catalog and template
echo "Finding template..."
CATALOG_ID=$(curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/catalogs | jq -r '.values[0].id')

TEMPLATE_ID=$(curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/catalogs/$CATALOG_ID/catalogItems | jq -r '.values[0].id')

# 4. Create vApp from template
echo "Creating vApp from template..."
VAPP_RESPONSE=$(curl -sk -X POST $SSVIRT_URL/cloudapi/1.0.0/vdcs/$VDC_ID/actions/instantiateTemplate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"demo-app\",
    \"description\": \"Demo application\",
    \"catalogItem\": {
      \"id\": \"$TEMPLATE_ID\"
    }
  }")

VAPP_ID=$(echo $VAPP_RESPONSE | jq -r '.id')
echo "Created vApp: $VAPP_ID"

# 5. Wait for vApp to be ready
echo "Waiting for vApp to be ready..."
while true; do
  VAPP_STATUS=$(curl -sk -H "Authorization: Bearer $TOKEN" \
    $SSVIRT_URL/cloudapi/1.0.0/vapps/$VAPP_ID | jq -r '.status')
  
  if [ "$VAPP_STATUS" = "RESOLVED" ]; then
    echo "vApp is ready!"
    break
  fi
  
  echo "Current status: $VAPP_STATUS"
  sleep 10
done

# 6. Get vApp details with VMs
echo "vApp Details:"
curl -sk -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/vapps/$VAPP_ID | jq '.'

# 7. Logout
curl -sk -X DELETE -H "Authorization: Bearer $TOKEN" \
  $SSVIRT_URL/cloudapi/1.0.0/sessions/$SESSION_ID

echo "Demo complete!"
```

For more information and advanced usage examples, consult your system administrator or refer to the SSVIRT API documentation.