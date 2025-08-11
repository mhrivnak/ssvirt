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
- [Error Responses](#error-responses)
- [Data Types](#data-types)

## Base URLs

- **Development:** `http://localhost:8080`
- **Production:** `https://your-ssvirt-instance.com`

## Authentication

SSVirt uses JWT-based authentication. Most endpoints require authentication via the `Authorization: Bearer <token>` header.

### Getting a Token

Use the session creation endpoint to obtain a JWT token:

```http
POST /cloudapi/1.0.0/sessions
Authorization: Basic <base64(username:password)>
```

## Health & Status

### Health Check
```http
GET /healthz
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
```http
GET /readyz
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
```http
GET /api/v1/version
```
Returns version information.

**Response:** `200 OK`
```json
{
  "version": "1.0.0",
  "build_time": "2024-01-15T09:00:00Z",
  "go_version": "go1.24.1",
  "git_commit": "abc123def456"
}
```

## Session Management

### Create Session (Login)
```http
POST /cloudapi/1.0.0/sessions
Authorization: Basic <base64(username:password)>
Content-Type: application/json
```

**Request Body:**
```json
{}
```

**Response:** `200 OK`
```json
{
  "sessionId": "urn:vcloud:session:12345678-1234-1234-1234-123456789abc",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc",
    "username": "admin",
    "fullName": "System Administrator"
  },
  "expires": "2024-01-16T10:30:00Z"
}
```

### Get Session Details
```http
GET /cloudapi/1.0.0/sessions/{sessionId}
Authorization: Bearer <token>
```

**Parameters:**
- `sessionId` (string) - Session URN ID

**Response:** `200 OK`
```json
{
  "sessionId": "urn:vcloud:session:12345678-1234-1234-1234-123456789abc",
  "user": {
    "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc",
    "username": "admin",
    "fullName": "System Administrator"
  },
  "expires": "2024-01-16T10:30:00Z"
}
```

### Delete Session (Logout)
```http
DELETE /cloudapi/1.0.0/sessions/{sessionId}
Authorization: Bearer <token>
```

**Response:** `204 No Content`

## User Management

### List Users
```http
GET /cloudapi/1.0.0/users
Authorization: Bearer <token>
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
```http
POST /cloudapi/1.0.0/users
Authorization: Bearer <token>
Content-Type: application/json
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
```http
GET /cloudapi/1.0.0/users/{id}
Authorization: Bearer <token>
```

**Parameters:**
- `id` (string) - User URN ID

**Response:** `200 OK` - Same format as user object in list response

### Update User
```http
PUT /cloudapi/1.0.0/users/{id}
Authorization: Bearer <token>
Content-Type: application/json
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
```http
DELETE /cloudapi/1.0.0/users/{id}
Authorization: Bearer <token>
```

**Response:** `204 No Content`

## Organization Management

### List Organizations
```http
GET /cloudapi/1.0.0/orgs
Authorization: Bearer <token>
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
```http
POST /cloudapi/1.0.0/orgs
Authorization: Bearer <token>
Content-Type: application/json
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
```http
GET /cloudapi/1.0.0/orgs/{id}
Authorization: Bearer <token>
```

**Parameters:**
- `id` (string) - Organization URN ID

**Response:** `200 OK` - Same format as organization object in list response

### Update Organization
```http
PUT /cloudapi/1.0.0/orgs/{id}
Authorization: Bearer <token>
Content-Type: application/json
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
```http
DELETE /cloudapi/1.0.0/orgs/{id}
Authorization: Bearer <token>
```

**Response:** `204 No Content`

**Note:** The Provider organization cannot be deleted.

## Role Management

### List Roles
```http
GET /cloudapi/1.0.0/roles
Authorization: Bearer <token>
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
```http
GET /cloudapi/1.0.0/roles/{id}
Authorization: Bearer <token>
```

**Parameters:**
- `id` (string) - Role URN ID

**Response:** `200 OK` - Same format as role object in list response

## Virtual Data Centers (VDCs)

### List VDCs
```http
GET /cloudapi/1.0.0/vdcs
Authorization: Bearer <token>
```

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
      "orgEntityRef": {
        "name": "Engineering",
        "id": "urn:vcloud:org:33333333-3333-3333-3333-333333333333"
      }
    }
  ]
}
```

### Get VDC Details
```http
GET /cloudapi/1.0.0/vdcs/{vdc_id}
Authorization: Bearer <token>
```

**Parameters:**
- `vdc_id` (string) - VDC URN ID

**Response:** `200 OK` - Same format as VDC object in list response

## Catalog Management

### List Catalogs
```http
GET /cloudapi/1.0.0/catalogs
Authorization: Bearer <token>
```

### Create Catalog
```http
POST /cloudapi/1.0.0/catalogs
Authorization: Bearer <token>
Content-Type: application/json
```

### Get Catalog Details
```http
GET /cloudapi/1.0.0/catalogs/{catalogUrn}
Authorization: Bearer <token>
```

### Delete Catalog
```http
DELETE /cloudapi/1.0.0/catalogs/{catalogUrn}
Authorization: Bearer <token>
```

### List Catalog Items
```http
GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems
Authorization: Bearer <token>
```

### Get Catalog Item Details
```http
GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems/{itemId}
Authorization: Bearer <token>
```

## vApp Management

### List vApps in VDC
```http
GET /cloudapi/1.0.0/vdcs/{vdc_id}/vapps
Authorization: Bearer <token>
```

### Get vApp Details
```http
GET /cloudapi/1.0.0/vapps/{vapp_id}
Authorization: Bearer <token>
```

### Delete vApp
```http
DELETE /cloudapi/1.0.0/vapps/{vapp_id}
Authorization: Bearer <token>
```

### Instantiate Template (Create vApp)
```http
POST /cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate
Authorization: Bearer <token>
Content-Type: application/json
```

## Virtual Machine Operations

### Get VM Details
```http
GET /cloudapi/1.0.0/vms/{vm_id}
Authorization: Bearer <token>
```

**Parameters:**
- `vm_id` (string) - VM URN ID

## Admin API

The Admin API endpoints require System Administrator role.

### List VDCs in Organization
```http
GET /api/admin/org/{orgId}/vdcs
Authorization: Bearer <token>
```

### Create VDC
```http
POST /api/admin/org/{orgId}/vdcs
Authorization: Bearer <token>
Content-Type: application/json
```

### Get VDC Details (Admin)
```http
GET /api/admin/org/{orgId}/vdcs/{vdcId}
Authorization: Bearer <token>
```

### Update VDC
```http
PUT /api/admin/org/{orgId}/vdcs/{vdcId}
Authorization: Bearer <token>
Content-Type: application/json
```

### Delete VDC
```http
DELETE /api/admin/org/{orgId}/vdcs/{vdcId}
Authorization: Bearer <token>
```

## Error Responses

### Standard Error Format

All API errors return JSON responses with the following format:

```json
{
  "error": "Descriptive error message"
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
  "error": "Invalid user ID format"
}
```

**Authentication Required:**
```json
{
  "error": "Authentication required"
}
```

**Resource Not Found:**
```json
{
  "error": "User not found"
}
```

**Duplicate Resource:**
```json
{
  "error": "Username already exists"
}
```

**Insufficient Permissions:**
```json
{
  "error": "System Administrator role required"
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