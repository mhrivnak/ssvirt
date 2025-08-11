# User and Organization Management API Guide

This guide provides comprehensive documentation for managing users and organizations through the SSVirt CloudAPI endpoints.

## Table of Contents

- [Authentication](#authentication)
- [User Management](#user-management)
- [Organization Management](#organization-management)
- [Error Handling](#error-handling)
- [Examples](#examples)

## Authentication

All CloudAPI endpoints require JWT authentication. First, create a session:

```bash
# Login to get session token
curl -X POST http://localhost:8080/cloudapi/1.0.0/sessions \
  -H "Content-Type: application/json" \
  -H "Authorization: Basic $(echo -n 'username:password' | base64)" \
  -d '{}'
```

Use the returned token in the `Authorization: Bearer <token>` header for subsequent requests.

## User Management

### List Users

Retrieve a paginated list of users with entity references.

**Endpoint:** `GET /cloudapi/1.0.0/users`

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25, max: 100) - Items per page

**Example Request:**
```bash
curl -X GET "http://localhost:8080/cloudapi/1.0.0/users?page=1&pageSize=10" \
  -H "Authorization: Bearer <your-token>"
```

**Example Response:**
```json
{
  "resultTotal": 25,
  "pageCount": 3,
  "page": 1,
  "pageSize": 10,
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

### Get User Details

Retrieve detailed information about a specific user.

**Endpoint:** `GET /cloudapi/1.0.0/users/{id}`

**Parameters:**
- `id` (string, required) - User URN ID in format `urn:vcloud:user:<uuid>`

**Example Request:**
```bash
curl -X GET "http://localhost:8080/cloudapi/1.0.0/users/urn:vcloud:user:12345678-1234-1234-1234-123456789abc" \
  -H "Authorization: Bearer <your-token>"
```

**Example Response:**
```json
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
```

### Create User

Create a new user account.

**Endpoint:** `POST /cloudapi/1.0.0/users`

**Request Body Schema:**
```json
{
  "username": "string (required)",
  "fullName": "string (required)",
  "email": "string (required, valid email)",
  "password": "string (required, min 6 characters)",
  "description": "string (optional)",
  "organizationId": "string (optional, org URN)",
  "deployedVmQuota": "integer (optional, default: 0)",
  "storedVmQuota": "integer (optional, default: 0)",
  "enabled": "boolean (optional, default: true)",
  "providerType": "string (optional, default: LOCAL)"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8080/cloudapi/1.0.0/users" \
  -H "Authorization: Bearer <your-token>" \
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
    "enabled": true
  }'
```

**Success Response (201 Created):**
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

### Update User

Update an existing user account.

**Endpoint:** `PUT /cloudapi/1.0.0/users/{id}`

**Parameters:**
- `id` (string, required) - User URN ID

**Request Body Schema:** (All fields optional for partial updates)
```json
{
  "username": "string",
  "fullName": "string", 
  "email": "string",
  "password": "string (min 6 characters)",
  "description": "string",
  "organizationId": "string (org URN)",
  "deployedVmQuota": "integer",
  "storedVmQuota": "integer",
  "enabled": "boolean",
  "providerType": "string"
}
```

**Example Request:**
```bash
curl -X PUT "http://localhost:8080/cloudapi/1.0.0/users/urn:vcloud:user:22222222-2222-2222-2222-222222222222" \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "fullName": "Jane Doe Smith",
    "deployedVmQuota": 10,
    "enabled": false
  }'
```

### Delete User

Delete a user account.

**Endpoint:** `DELETE /cloudapi/1.0.0/users/{id}`

**Parameters:**
- `id` (string, required) - User URN ID

**Example Request:**
```bash
curl -X DELETE "http://localhost:8080/cloudapi/1.0.0/users/urn:vcloud:user:22222222-2222-2222-2222-222222222222" \
  -H "Authorization: Bearer <your-token>"
```

**Success Response:** `204 No Content`

## Organization Management

### List Organizations

Retrieve a paginated list of organizations.

**Endpoint:** `GET /cloudapi/1.0.0/orgs`

**Query Parameters:**
- `page` (integer, default: 1) - Page number
- `pageSize` (integer, default: 25, max: 100) - Items per page

**Example Request:**
```bash
curl -X GET "http://localhost:8080/cloudapi/1.0.0/orgs?page=1&pageSize=10" \
  -H "Authorization: Bearer <your-token>"
```

**Example Response:**
```json
{
  "resultTotal": 3,
  "pageCount": 1,
  "page": 1,
  "pageSize": 10,
  "associations": [],
  "values": [
    {
      "id": "urn:vcloud:org:11111111-1111-1111-1111-111111111111",
      "name": "Provider",
      "displayName": "Provider Organization",
      "description": "Default provider organization",
      "isEnabled": true,
      "orgVdcCount": 0,
      "catalogCount": 0,
      "vappCount": 0,
      "runningVMCount": 0,
      "userCount": 0,
      "diskCount": 0,
      "canManageOrgs": true,
      "canPublish": false,
      "maskedEventTaskUsername": "",
      "directlyManagedOrgCount": 0
    }
  ]
}
```

### Get Organization Details

Retrieve detailed information about a specific organization.

**Endpoint:** `GET /cloudapi/1.0.0/orgs/{id}`

**Parameters:**
- `id` (string, required) - Organization URN ID in format `urn:vcloud:org:<uuid>`

**Example Request:**
```bash
curl -X GET "http://localhost:8080/cloudapi/1.0.0/orgs/urn:vcloud:org:11111111-1111-1111-1111-111111111111" \
  -H "Authorization: Bearer <your-token>"
```

### Create Organization

Create a new organization.

**Endpoint:** `POST /cloudapi/1.0.0/orgs`

**Request Body Schema:**
```json
{
  "name": "string (required)",
  "displayName": "string (optional)",
  "description": "string (optional)",
  "isEnabled": "boolean (optional, default: true)",
  "canManageOrgs": "boolean (optional, default: false)",
  "canPublish": "boolean (optional, default: false)",
  "maskedEventTaskUsername": "string (optional)"
}
```

**Example Request:**
```bash
curl -X POST "http://localhost:8080/cloudapi/1.0.0/orgs" \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Engineering",
    "displayName": "Engineering Department",
    "description": "Engineering team organization",
    "isEnabled": true,
    "canManageOrgs": false,
    "canPublish": true
  }'
```

**Success Response (201 Created):**
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
  "maskedEventTaskUsername": "",
  "directlyManagedOrgCount": 0
}
```

### Update Organization

Update an existing organization.

**Endpoint:** `PUT /cloudapi/1.0.0/orgs/{id}`

**Parameters:**
- `id` (string, required) - Organization URN ID

**Request Body Schema:** (All fields optional for partial updates)
```json
{
  "name": "string",
  "displayName": "string",
  "description": "string",
  "isEnabled": "boolean",
  "canManageOrgs": "boolean",
  "canPublish": "boolean",
  "maskedEventTaskUsername": "string"
}
```

**Example Request:**
```bash
curl -X PUT "http://localhost:8080/cloudapi/1.0.0/orgs/urn:vcloud:org:33333333-3333-3333-3333-333333333333" \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "displayName": "Engineering & DevOps",
    "description": "Combined Engineering and DevOps teams",
    "canPublish": false
  }'
```

### Delete Organization

Delete an organization.

**Endpoint:** `DELETE /cloudapi/1.0.0/orgs/{id}`

**Parameters:**
- `id` (string, required) - Organization URN ID

**Example Request:**
```bash
curl -X DELETE "http://localhost:8080/cloudapi/1.0.0/orgs/urn:vcloud:org:33333333-3333-3333-3333-333333333333" \
  -H "Authorization: Bearer <your-token>"
```

**Success Response:** `204 No Content`

**Note:** The Provider organization cannot be deleted and will return a 400 Bad Request error.

## Error Handling

### HTTP Status Codes

- `200 OK` - Successful GET or PUT request
- `201 Created` - Successful POST request
- `204 No Content` - Successful DELETE request
- `400 Bad Request` - Invalid request format or parameters
- `401 Unauthorized` - Missing or invalid authentication
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource already exists (username, email, or organization name)
- `500 Internal Server Error` - Server error

### Error Response Format

```json
{
  "error": "Descriptive error message"
}
```

### Common Error Scenarios

**Invalid URN Format:**
```json
{
  "error": "Invalid user ID format"
}
```

**Duplicate Username:**
```json
{
  "error": "Username already exists"
}
```

**User Not Found:**
```json
{
  "error": "User not found"
}
```

**Cannot Delete Provider Organization:**
```json
{
  "error": "Cannot delete the Provider organization"
}
```

## Examples

### Complete User Management Workflow

```bash
# 1. Create session
SESSION_RESPONSE=$(curl -s -X POST http://localhost:8080/cloudapi/1.0.0/sessions \
  -H "Content-Type: application/json" \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)" \
  -d '{}')

TOKEN=$(echo $SESSION_RESPONSE | jq -r '.token')

# 2. Create organization
ORG_RESPONSE=$(curl -s -X POST http://localhost:8080/cloudapi/1.0.0/orgs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Development",
    "displayName": "Development Team",
    "description": "Development organization"
  }')

ORG_ID=$(echo $ORG_RESPONSE | jq -r '.id')

# 3. Create user in the organization
USER_RESPONSE=$(curl -s -X POST http://localhost:8080/cloudapi/1.0.0/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"username\": \"developer1\",
    \"fullName\": \"Developer One\",
    \"email\": \"dev1@example.com\",
    \"password\": \"devpassword123\",
    \"organizationId\": \"$ORG_ID\",
    \"deployedVmQuota\": 5
  }")

USER_ID=$(echo $USER_RESPONSE | jq -r '.id')

# 4. Update user quota
curl -X PUT http://localhost:8080/cloudapi/1.0.0/users/$USER_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "deployedVmQuota": 10,
    "storedVmQuota": 15
  }'

# 5. List users in organization
curl -X GET "http://localhost:8080/cloudapi/1.0.0/users?pageSize=50" \
  -H "Authorization: Bearer $TOKEN"
```

### Bulk Operations Example

```bash
# Create multiple users via script
for i in {1..5}; do
  curl -X POST http://localhost:8080/cloudapi/1.0.0/users \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"username\": \"user$i\",
      \"fullName\": \"User Number $i\",
      \"email\": \"user$i@example.com\",
      \"password\": \"password123\",
      \"organizationId\": \"$ORG_ID\"
    }"
done
```

This guide provides comprehensive coverage of user and organization management through the SSVirt CloudAPI. For additional API endpoints and features, see the [API Reference](api-reference.md).