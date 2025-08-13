# SSVirt API Reference

This document provides a comprehensive reference for all SSVirt API endpoints, organized by functional area.

## Table of Contents

- [Base URLs](#base-urls)
- [Authentication](#authentication)
- [Health & Status](#health--status)
- [Session Management](#session-management)
- [User Management](#user-management)
- [Organization Management](#organization-management)
- [Role Management](#role-management)
- [Virtual Data Centers (VDCs)](#virtual-data-centers-vdcs)
- [Catalog Management](#catalog-management)
- [vApp Management](#vapp-management)
- [Virtual Machine Operations](#virtual-machine-operations)
- [Admin API](#admin-api)
- [Legacy Endpoints](#legacy-endpoints)
- [Error Responses](#error-responses)
- [Data Types](#data-types)

## Base URLs

- **Development:** `http://localhost:8080`
- **Production:** `https://your-ssvirt-instance.com`

**Setting the Base URL:**
```bash
# For development
export SSVIRT_URL="http://localhost:8080"

# For production
export SSVIRT_URL="https://ssvirt.apps.your-cluster.com"
```

## Authentication

SSVirt uses JWT-based authentication. Most endpoints require authentication via the `Authorization: Bearer <token>` header.

### Getting a Token

Use the session creation endpoint to obtain a JWT token:

```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/sessions \
  -H "Authorization: Basic $(echo -n 'username:password' | base64)" \
  -H "Content-Type: application/json"
```

After successful authentication, extract and store the token:
```bash
# Store the JWT token from the Authorization header
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Health & Status

### Health Check
```bash
curl -X GET $SSVIRT_URL/healthz
```
Basic health check endpoint.

**Response:** `200 OK`
```json
{
  "status": "ok",
  "version": "1.0.0",
  "database": "ok",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Readiness Check
```bash
curl -X GET $SSVIRT_URL/readyz
```
Kubernetes readiness probe endpoint.

**Response:** `200 OK`
```json
{
  "ready": true,
  "timestamp": "2024-01-15T10:30:00Z",
  "services": {
    "database": "ready",
    "auth": "ready",
    "k8s": "ready"
  }
}
```

### Version Information
```bash
curl -X GET $SSVIRT_URL/api/v1/version
```
Returns version information.

**Response:** `200 OK`
```json
{
  "version": "1.0.0",
  "build_time": "dev",
  "go_version": "go1.24.1",
  "git_commit": "dev"
}
```

## Session Management

### Create Session (Login)
```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/sessions \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)" \
  -H "Content-Type: application/json"
```

**Request Body:** None required

**Response:** `200 OK`
```json
{
  "id": "urn:vcloud:session:12345678-1234-1234-1234-123456789abc",
  "site": {
    "name": "SSVirt",
    "id": "urn:vcloud:site:12345678-1234-1234-1234-123456789abc"
  },
  "user": {
    "name": "admin",
    "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc"
  },
  "org": {
    "name": "Provider",
    "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
  },
  "operatingOrg": {
    "name": "Provider",
    "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
  },
  "location": "",
  "roles": ["System Administrator"],
  "roleRefs": [
    {
      "name": "System Administrator",
      "id": "urn:vcloud:role:87654321-4321-4321-4321-210987654321"
    }
  ],
  "sessionIdleTimeoutMinutes": 30
}
```

**Response Headers:**
- `Authorization: Bearer <jwt_token>` - Use this token for subsequent authenticated requests

### Get Session Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/sessions/urn:vcloud:session:12345678-1234-1234-1234-123456789abc \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `sessionId` (string) - Session URN ID

**Response:** `200 OK`
```json
{
  "id": "urn:vcloud:session:12345678-1234-1234-1234-123456789abc",
  "site": {
    "name": "SSVirt",
    "id": "urn:vcloud:site:12345678-1234-1234-1234-123456789abc"
  },
  "user": {
    "name": "admin",
    "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc"
  },
  "org": {
    "name": "Provider",
    "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
  },
  "operatingOrg": {
    "name": "Provider",
    "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
  },
  "location": "",
  "roles": ["System Administrator"],
  "roleRefs": [
    {
      "name": "System Administrator",
      "id": "urn:vcloud:role:87654321-4321-4321-4321-210987654321"
    }
  ],
  "sessionIdleTimeoutMinutes": 30
}
```

### Delete Session (Logout)
```bash
curl -X DELETE $SSVIRT_URL/cloudapi/1.0.0/sessions/urn:vcloud:session:12345678-1234-1234-1234-123456789abc \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `204 No Content`

## User Management

### List Users
```bash
curl -X GET "$SSVIRT_URL/cloudapi/1.0.0/users?page=1&pageSize=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25, max: 100) - Items per page

**Response:** `200 OK`
```json
{
  "resultTotal": 100,
  "pageCount": 4,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc",
      "username": "john.doe",
      "fullName": "John Doe",
      "email": "john.doe@example.com",
      "description": "System Administrator",
      "enabled": true,
      "deployedVmQuota": 10,
      "storedVmQuota": 20,
      "nameInSource": "john.doe",
      "providerType": "LOCAL",
      "isGroupRole": false,
      "locked": false,
      "stranded": false,
      "roleEntityRefs": [
        {
          "name": "System Administrator",
          "id": "urn:vcloud:role:87654321-4321-4321-4321-210987654321"
        }
      ],
      "orgEntityRef": {
        "name": "Provider",
        "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
      }
    }
  ]
}
```

### Create User
```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "jane.smith",
    "fullName": "Jane Smith",
    "email": "jane.smith@example.com",
    "password": "securepassword123",
    "description": "Organization Administrator",
    "organizationId": "urn:vcloud:org:11111111-1111-1111-1111-111111111111",
    "deployedVmQuota": 5,
    "storedVmQuota": 10,
    "enabled": true,
    "providerType": "LOCAL"
  }'
```

**Request Body:**
```json
{
  "username": "jane.smith",
  "fullName": "Jane Smith",
  "email": "jane.smith@example.com",
  "password": "securepassword123",
  "description": "Organization Administrator",
  "organizationId": "urn:vcloud:org:11111111-1111-1111-1111-111111111111",
  "deployedVmQuota": 5,
  "storedVmQuota": 10,
  "enabled": true,
  "providerType": "LOCAL"
}
```

**Response:** `201 Created`
```json
{
  "id": "urn:vcloud:user:22222222-2222-2222-2222-222222222222",
  "username": "jane.smith",
  "fullName": "Jane Smith",
  "email": "jane.smith@example.com",
  "description": "Organization Administrator",
  "enabled": true,
  "deployedVmQuota": 5,
  "storedVmQuota": 10,
  "nameInSource": "jane.smith",
  "providerType": "LOCAL",
  "orgEntityRef": {
    "name": "Provider",
    "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
  }
}
```

### Get User Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/users/urn:vcloud:user:12345678-1234-1234-1234-123456789abc \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `id` (string) - User URN ID

**Response:** `200 OK` - Same format as user object in list response

### Update User
```bash
curl -X PUT $SSVIRT_URL/cloudapi/1.0.0/users/urn:vcloud:user:12345678-1234-1234-1234-123456789abc \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "fullName": "Jane Doe Smith",
    "email": "jane.doe@example.com",
    "description": "Senior Administrator",
    "deployedVmQuota": 15,
    "storedVmQuota": 25,
    "enabled": false
  }'
```

**Request Body:** (All fields optional for partial updates)
```json
{
  "username": "jane.doe",
  "fullName": "Jane Doe Smith",
  "email": "jane.doe@example.com",
  "password": "newpassword123",
  "description": "Senior Administrator",
  "organizationId": "urn:vcloud:org:33333333-3333-3333-3333-333333333333",
  "deployedVmQuota": 15,
  "storedVmQuota": 25,
  "enabled": false,
  "providerType": "LDAP"
}
```

**Response:** `200 OK` - Updated user object

### Delete User
```bash
curl -X DELETE $SSVIRT_URL/cloudapi/1.0.0/users/urn:vcloud:user:12345678-1234-1234-1234-123456789abc \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `204 No Content`

## Organization Management

### List Organizations
```bash
curl -X GET "$SSVIRT_URL/cloudapi/1.0.0/orgs?page=1&pageSize=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25, max: 100) - Items per page

**Response:** `200 OK`
```json
{
  "resultTotal": 5,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111",
      "name": "Provider",
      "displayName": "Provider Organization",
      "description": "Default provider organization",
      "isEnabled": true,
      "orgVdcCount": 2,
      "catalogCount": 3,
      "vappCount": 5,
      "runningVMCount": 8,
      "userCount": 12,
      "diskCount": 0,
      "canManageOrgs": true,
      "canPublish": false,
      "maskedEventTaskUsername": "",
      "directlyManagedOrgCount": 0,
      "managedBy": {
        "name": "System Administrator",
        "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc"
      }
    }
  ]
}
```

### Create Organization
```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/orgs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Engineering",
    "displayName": "Engineering Department",
    "description": "Engineering team organization",
    "isEnabled": true,
    "canManageOrgs": false,
    "canPublish": true,
    "maskedEventTaskUsername": "system"
  }'
```

**Request Body:**
```json
{
  "name": "Engineering",
  "displayName": "Engineering Department",
  "description": "Engineering team organization",
  "isEnabled": true,
  "canManageOrgs": false,
  "canPublish": true,
  "maskedEventTaskUsername": "system"
}
```

**Response:** `201 Created`
```json
{
  "id": "urn:vcloud:org:33333333-3333-3333-3333-333333333333",
  "name": "Engineering",
  "displayName": "Engineering Department",
  "description": "Engineering team organization",
  "isEnabled": true,
  "orgVdcCount": 0,
  "catalogCount": 0,
  "vappCount": 0,
  "runningVMCount": 0,
  "userCount": 0,
  "diskCount": 0,
  "canManageOrgs": false,
  "canPublish": true,
  "maskedEventTaskUsername": "system",
  "directlyManagedOrgCount": 0
}
```

### Get Organization Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/orgs/urn:vcloud:org:11111111-1111-1111-1111-111111111111 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `id` (string) - Organization URN ID

**Response:** `200 OK` - Same format as organization object in list response

### Update Organization
```bash
curl -X PUT $SSVIRT_URL/cloudapi/1.0.0/orgs/urn:vcloud:org:11111111-1111-1111-1111-111111111111 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "displayName": "Engineering & DevOps",
    "description": "Combined Engineering and DevOps teams",
    "isEnabled": false
  }'
```

**Request Body:** (All fields optional for partial updates)
```json
{
  "name": "Engineering-DevOps",
  "displayName": "Engineering & DevOps",
  "description": "Combined Engineering and DevOps teams",
  "isEnabled": false,
  "canManageOrgs": true,
  "canPublish": false,
  "maskedEventTaskUsername": "admin"
}
```

**Response:** `200 OK` - Updated organization object

### Delete Organization
```bash
curl -X DELETE $SSVIRT_URL/cloudapi/1.0.0/orgs/urn:vcloud:org:33333333-3333-3333-3333-333333333333 \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `204 No Content`

**Note:** The Provider organization cannot be deleted.

## Role Management

### List Roles
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/roles \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `200 OK`
```json
{
  "resultTotal": 3,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:role:87654321-4321-4321-4321-210987654321",
      "name": "System Administrator",
      "description": "Full system access",
      "bundleKey": "",
      "readOnly": true
    },
    {
      "id": "urn:vcloud:role:12345678-1234-1234-1234-123456789abc",
      "name": "Organization Administrator",
      "description": "Full access within assigned organization",
      "bundleKey": "",
      "readOnly": true
    },
    {
      "id": "urn:vcloud:role:11111111-1111-1111-1111-111111111111",
      "name": "vApp User",
      "description": "Basic user access to assigned vApps",
      "bundleKey": "",
      "readOnly": true
    }
  ]
}
```

### Get Role Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/roles/urn:vcloud:role:87654321-4321-4321-4321-210987654321 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `id` (string) - Role URN ID

**Response:** `200 OK` - Same format as role object in list response

## Virtual Data Centers (VDCs)

### List VDCs
```bash
curl -X GET "$SSVIRT_URL/cloudapi/1.0.0/vdcs?page=1&pageSize=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25, max: 100) - Items per page

**Response:** `200 OK`
```json
{
  "resultTotal": 2,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:vdc:44444444-4444-4444-4444-444444444444",
      "name": "production-vdc",
      "description": "Production environment VDC",
      "allocationModel": "Flex",
      "computeCapacity": {
        "cpu": {
          "allocated": 10000,
          "limit": 20000,
          "units": "MHz"
        },
        "memory": {
          "allocated": 16384,
          "limit": 32768,
          "units": "MB"
        }
      },
      "providerVdc": {
        "id": "urn:vcloud:providervdc:67890"
      },
      "nicQuota": 100,
      "networkQuota": 50,
      "vdcStorageProfiles": {},
      "isThinProvision": false,
      "isEnabled": true
    }
  ]
}
```

### Get VDC Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/vdcs/urn:vcloud:vdc:44444444-4444-4444-4444-444444444444 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `vdc_id` (string) - VDC URN ID

**Response:** `200 OK` - Same format as VDC object in list response

## Catalog Management

### List Catalogs
```bash
curl -X GET "$SSVIRT_URL/cloudapi/1.0.0/catalogs?page=1&pageSize=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25, max: 100) - Items per page

**Response:** `200 OK`
```json
{
  "resultTotal": 3,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:catalog:55555555-5555-5555-5555-555555555555",
      "name": "Public Templates",
      "description": "Public template catalog",
      "org": {
        "name": "Provider",
        "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
      },
      "isPublished": true,
      "isSubscribed": false,
      "creationDate": "2024-01-15T10:30:00Z",
      "numberOfVAppTemplates": 5,
      "numberOfMedia": 0,
      "isLocal": true,
      "version": 1
    }
  ]
}
```

### Create Catalog
```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/catalogs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Templates",
    "description": "Private template catalog",
    "orgId": "urn:vcloud:org:11111111-1111-1111-1111-111111111111",
    "isPublished": false
  }'
```

**Request Body:**
```json
{
  "name": "My Templates",
  "description": "Private template catalog",
  "orgId": "urn:vcloud:org:11111111-1111-1111-1111-111111111111",
  "isPublished": false
}
```

**Response:** `201 Created` - Same format as catalog object in list response

### Get Catalog Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/catalogs/urn:vcloud:catalog:55555555-5555-5555-5555-555555555555 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `catalogUrn` (string) - Catalog URN ID

**Response:** `200 OK` - Same format as catalog object in list response

### Delete Catalog
```bash
curl -X DELETE $SSVIRT_URL/cloudapi/1.0.0/catalogs/urn:vcloud:catalog:55555555-5555-5555-5555-555555555555 \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `204 No Content`

### List Catalog Items
```bash
curl -X GET "$SSVIRT_URL/cloudapi/1.0.0/catalogs/urn:vcloud:catalog:55555555-5555-5555-5555-555555555555/catalogItems?page=1&pageSize=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25) - Items per page

**Response:** `200 OK`
```json
{
  "resultTotal": 5,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
      "name": "Ubuntu Server 22.04",
      "description": "Ubuntu Server 22.04 LTS template",
      "catalog": {
        "name": "Public Templates",
        "id": "urn:vcloud:catalog:55555555-5555-5555-5555-555555555555"
      },
      "osType": "ubuntu64Guest",
      "cpuCount": 2,
      "memoryMB": 4096,
      "diskSizeGB": 20,
      "creationDate": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### Get Catalog Item Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/catalogs/urn:vcloud:catalog:55555555-5555-5555-5555-555555555555/catalogItems/urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `catalogUrn` (string) - Catalog URN ID
- `itemId` (string) - Catalog Item URN ID

**Response:** `200 OK` - Same format as catalog item object in list response

## vApp Management

### List vApps in VDC
```bash
curl -X GET "$SSVIRT_URL/cloudapi/1.0.0/vdcs/urn:vcloud:vdc:44444444-4444-4444-4444-444444444444/vapps?page=1&pageSize=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `vdc_id` (string) - VDC URN ID

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25) - Items per page

**Response:** `200 OK`
```json
{
  "resultTotal": 2,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:vapp:77777777-7777-7777-7777-777777777777",
      "name": "web-servers",
      "description": "Web application servers",
      "status": "RESOLVED",
      "vdcId": "urn:vcloud:vdc:44444444-4444-4444-4444-444444444444",
      "templateId": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
      "createdAt": "2024-01-15T10:30:00Z",
      "numberOfVMs": 2
    }
  ]
}
```

### Get vApp Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/vapps/urn:vcloud:vapp:77777777-7777-7777-7777-777777777777 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `vapp_id` (string) - vApp URN ID

**Response:** `200 OK`
```json
{
  "id": "urn:vcloud:vapp:77777777-7777-7777-7777-777777777777",
  "name": "web-servers",
  "description": "Web application servers",
  "status": "RESOLVED",
  "vdcId": "urn:vcloud:vdc:44444444-4444-4444-4444-444444444444",
  "templateId": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T11:00:00Z",
  "numberOfVMs": 2,
  "vms": [
    {
      "name": "web-01",
      "id": "urn:vcloud:vm:88888888-8888-8888-8888-888888888888"
    },
    {
      "name": "web-02",
      "id": "urn:vcloud:vm:99999999-9999-9999-9999-999999999999"
    }
  ]
}
```

### Delete vApp
```bash
curl -X DELETE $SSVIRT_URL/cloudapi/1.0.0/vapps/urn:vcloud:vapp:77777777-7777-7777-7777-777777777777 \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `204 No Content`

### Instantiate Template (Create vApp)
```bash
curl -X POST $SSVIRT_URL/cloudapi/1.0.0/vdcs/urn:vcloud:vdc:44444444-4444-4444-4444-444444444444/actions/instantiateTemplate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-application",
    "description": "My web application",
    "catalogItem": {
      "id": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
      "name": "Ubuntu Server 22.04"
    }
  }'
```

**Parameters:**
- `vdc_id` (string) - VDC URN ID where the vApp will be created

**Request Body:**
```json
{
  "name": "my-application",
  "description": "My web application",
  "catalogItem": {
    "id": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
    "name": "Ubuntu Server 22.04"
  }
}
```

**Response:** `201 Created`
```json
{
  "id": "urn:vcloud:vapp:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "name": "my-application",
  "description": "My web application",
  "status": "RESOLVED",
  "vdcId": "urn:vcloud:vdc:44444444-4444-4444-4444-444444444444",
  "templateId": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
  "createdAt": "2024-01-15T15:30:00Z",
  "numberOfVMs": 1,
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
}
```

## Virtual Machine Operations

### Get VM Details
```bash
curl -X GET $SSVIRT_URL/cloudapi/1.0.0/vms/urn:vcloud:vm:88888888-8888-8888-8888-888888888888 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `vm_id` (string) - VM URN ID

**Response:** `200 OK`
```json
{
  "id": "urn:vcloud:vm:88888888-8888-8888-8888-888888888888",
  "name": "web-01",
  "description": "Web server VM",
  "status": "POWERED_ON",
  "vappId": "urn:vcloud:vapp:77777777-7777-7777-7777-777777777777",
  "templateId": "urn:vcloud:catalogitem:66666666-6666-6666-6666-666666666666",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T11:00:00Z",
  "guestOs": "ubuntu64Guest",
  "vmTools": {
    "status": "guestToolsRunning",
    "version": "12.0.0"
  },
  "hardware": {
    "numCpus": 2,
    "coresPerSocket": 1,
    "memoryMB": 4096
  },
  "storageProfile": {
    "name": "Default",
    "id": "urn:vcloud:storageprofile:default"
  },
  "network": {
    "networkConnectionSection": {
      "primaryNetworkConnectionIndex": 0,
      "networkConnection": [
        {
          "network": "default-net",
          "networkConnectionIndex": 0,
          "ipAddress": "192.168.1.100",
          "isConnected": true,
          "macAddress": "00:50:56:01:02:03",
          "ipAddressAllocationMode": "DHCP"
        }
      ]
    }
  }
}
```

## Admin API

The Admin API endpoints require System Administrator role.

### List VDCs in Organization
```bash
curl -X GET "$SSVIRT_URL/api/admin/org/urn:vcloud:org:11111111-1111-1111-1111-111111111111/vdcs?page=1&page_size=25" \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `orgId` (string) - Organization URN ID

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `page_size` (integer, default: 25, max: 100) - Items per page

**Response:** `200 OK` - Same pagination format as other VDC endpoints

### Create VDC
```bash
curl -X POST $SSVIRT_URL/api/admin/org/urn:vcloud:org:11111111-1111-1111-1111-111111111111/vdcs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "development-vdc",
    "description": "Development Virtual Data Center",
    "allocationModel": "Flex",
    "computeCapacity": {
      "cpu": {
        "allocated": 10000,
        "limit": 20000,
        "units": "MHz"
      },
      "memory": {
        "allocated": 16384,
        "limit": 32768,
        "units": "MB"
      }
    },
    "providerVdc": {
      "id": "urn:vcloud:providervdc:67890"
    },
    "nicQuota": 100,
    "networkQuota": 50,
    "isThinProvision": false,
    "isEnabled": true
  }'
```

**Request Body:**
```json
{
  "name": "development-vdc",
  "description": "Development Virtual Data Center",
  "allocationModel": "Flex",
  "computeCapacity": {
    "cpu": {
      "allocated": 10000,
      "limit": 20000,
      "units": "MHz"
    },
    "memory": {
      "allocated": 16384,
      "limit": 32768,
      "units": "MB"
    }
  },
  "providerVdc": {
    "id": "urn:vcloud:providervdc:67890"
  },
  "nicQuota": 100,
  "networkQuota": 50,
  "isThinProvision": false,
  "isEnabled": true
}
```

**Response:** `201 Created` - VDC object with generated ID

### Get VDC Details (Admin)
```bash
curl -X GET $SSVIRT_URL/api/admin/org/urn:vcloud:org:11111111-1111-1111-1111-111111111111/vdcs/urn:vcloud:vdc:44444444-4444-4444-4444-444444444444 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `orgId` (string) - Organization URN ID
- `vdcId` (string) - VDC URN ID

**Response:** `200 OK` - Complete VDC object with all administrative details

### Update VDC
```bash
curl -X PUT $SSVIRT_URL/api/admin/org/urn:vcloud:org:11111111-1111-1111-1111-111111111111/vdcs/urn:vcloud:vdc:44444444-4444-4444-4444-444444444444 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated Development Virtual Data Center",
    "computeCapacity": {
      "cpu": {
        "allocated": 15000,
        "limit": 25000,
        "units": "MHz"
      },
      "memory": {
        "allocated": 20480,
        "limit": 40960,
        "units": "MB"
      }
    },
    "isEnabled": false
  }'
```

**Request Body:** (All fields optional for partial updates)
```json
{
  "name": "new-vdc-name",
  "description": "Updated description",
  "allocationModel": "AllocationPool",
  "computeCapacity": {
    "cpu": {
      "allocated": 15000,
      "limit": 25000,
      "units": "MHz"
    },
    "memory": {
      "allocated": 20480,
      "limit": 40960,
      "units": "MB"
    }
  },
  "providerVdc": {
    "id": "urn:vcloud:providervdc:newprovider"
  },
  "nicQuota": 150,
  "networkQuota": 75,
  "isThinProvision": true,
  "isEnabled": false
}
```

**Response:** `200 OK` - Updated VDC object

### Delete VDC
```bash
curl -X DELETE $SSVIRT_URL/api/admin/org/urn:vcloud:org:11111111-1111-1111-1111-111111111111/vdcs/urn:vcloud:vdc:44444444-4444-4444-4444-444444444444 \
  -H "Authorization: Bearer $TOKEN"
```

**Parameters:**
- `orgId` (string) - Organization URN ID
- `vdcId` (string) - VDC URN ID

**Response:** `204 No Content`

**Error Responses:**
- `409 Conflict` - VDC contains vApps that must be deleted first

## Legacy Endpoints

### User Profile
```bash
curl -X GET $SSVIRT_URL/api/v1/user/profile \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `200 OK`
```json
{
  "success": true,
  "data": {
    "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc",
    "username": "admin",
    "email": "admin@example.com",
    "full_name": "System Administrator"
  }
}
```

**Note:** This is a legacy endpoint. Use the CloudAPI session endpoints for new integrations.

## Error Responses

### Standard Error Format

All API errors return JSON responses with the following format:

```json
{
  "code": 400,
  "error": "Bad Request",
  "message": "Descriptive error message",
  "details": "Additional error details (optional)"
}
```

### HTTP Status Codes

- `200 OK` - Successful GET or PUT request
- `201 Created` - Successful POST request  
- `204 No Content` - Successful DELETE request
- `400 Bad Request` - Invalid request format, parameters, or business logic violation
- `401 Unauthorized` - Missing, invalid, or expired authentication
- `403 Forbidden` - Insufficient permissions for the requested operation
- `404 Not Found` - Requested resource does not exist
- `409 Conflict` - Resource already exists or conflict with current state
- `500 Internal Server Error` - Unexpected server error

### Common Error Examples

**Invalid URN Format:**
```json
{
  "code": 400,
  "error": "Bad Request",
  "message": "Invalid user ID format",
  "details": "User ID must be a valid URN with prefix 'urn:vcloud:user:'"
}
```

**Authentication Required:**
```json
{
  "code": 401,
  "error": "Unauthorized",
  "message": "Authentication required"
}
```

**Resource Not Found:**
```json
{
  "code": 404,
  "error": "Not Found",
  "message": "User not found"
}
```

**Duplicate Resource:**
```json
{
  "code": 409,
  "error": "Conflict",
  "message": "Username already exists",
  "details": "A user with username 'john.doe' already exists"
}
```

**Insufficient Permissions:**
```json
{
  "code": 403,
  "error": "Forbidden",
  "message": "System Administrator role required",
  "details": "User management requires System Administrator privileges"
}
```

## Data Types

### URN Format

All entity IDs use the URN format: `urn:vcloud:{type}:{uuid}`

**Types:**
- `user` - User accounts
- `org` - Organizations  
- `role` - Roles
- `vdc` - Virtual Data Centers
- `catalog` - Catalogs
- `catalogitem` - Catalog items
- `vapp` - vApps
- `vm` - Virtual machines
- `session` - User sessions
- `site` - Site references
- `providervdc` - Provider VDCs

**Example:** `urn:vcloud:user:12345678-1234-1234-1234-123456789abc`

### Entity Reference

Many API responses include entity references to related objects:

```json
{
  "name": "Provider",
  "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111"
}
```

### Pagination Response

Collection endpoints return paginated responses:

```json
{
  "resultTotal": 150,
  "pageCount": 6,
  "page": 1,
  "pageSize": 25,
  "associations": [],
  "values": [
    // Array of entities
  ]
}
```

**Pagination Fields:**
- `resultTotal` - Total number of items matching the query
- `pageCount` - Total number of pages available
- `page` - Current page number (1-based)
- `pageSize` - Number of items per page
- `associations` - Related entity links (typically empty)
- `values` - Array containing the actual entity data

This API reference provides comprehensive documentation for all SSVirt endpoints. For detailed usage examples and workflows, see the [User and Organization API Guide](user-organization-api-guide.md).