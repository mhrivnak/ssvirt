# VDC Public API Guide

## Overview

The VDC Public API provides read-only access to Virtual Data Centers (VDCs) for authenticated users. Unlike the admin VDC API which requires System Administrator privileges, this API allows regular users to view VDCs they have access to based on their organization membership.

## Base URL

All VDC Public API endpoints are available under the CloudAPI base path:

```
https://your-ssvirt-instance.com/cloudapi/1.0.0/vdcs
```

## Authentication

All VDC Public API endpoints require authentication using a JWT token obtained through the CloudAPI session endpoints.

### Getting an Authentication Token

1. **Create a session** using Basic Authentication:
```bash
curl -X POST https://your-ssvirt-instance.com/cloudapi/1.0.0/sessions \
  -H "Authorization: Basic $(echo -n 'username:password' | base64)"
```

2. **Extract the JWT token** from the response headers:
The token will be in the `Authorization` header of the response as `Bearer <token>`.

3. **Use the token** in subsequent requests:
```bash
curl -H "Authorization: Bearer <your-jwt-token>" \
  https://your-ssvirt-instance.com/cloudapi/1.0.0/vdcs
```

## Endpoints

### List VDCs

Retrieve a paginated list of VDCs accessible to the authenticated user.

**Request**
```http
GET /cloudapi/1.0.0/vdcs
Authorization: Bearer <jwt-token>
```

**Query Parameters**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | integer | 1 | Page number (minimum: 1) |
| `pageSize` | integer | 25 | Items per page (maximum: 100) |

**Example Request**
```bash
curl -H "Authorization: Bearer <jwt-token>" \
  "https://your-ssvirt-instance.com/cloudapi/1.0.0/vdcs?page=1&pageSize=10"
```

**Success Response** `200 OK`
```json
{
  "resultTotal": 42,
  "pageCount": 5,
  "page": 1,
  "pageSize": 10,
  "values": [
    {
      "id": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
      "name": "Production VDC",
      "description": "Primary production environment for web applications",
      "allocationModel": "PayAsYouGo",
      "computeCapacity": {
        "cpu": {
          "allocated": 2000,
          "limit": 4000,
          "units": "MHz"
        },
        "memory": {
          "allocated": 2048,
          "limit": 4096,
          "units": "MB"
        }
      },
      "providerVdc": {
        "id": "urn:vcloud:providervdc:87654321-4321-4321-4321-cba987654321"
      },
      "nicQuota": 100,
      "networkQuota": 50,
      "vdcStorageProfiles": {},
      "isThinProvision": false,
      "isEnabled": true
    },
    {
      "id": "urn:vcloud:vdc:87654321-4321-4321-4321-cba987654321",
      "name": "Development VDC",
      "description": "Development and testing environment",
      "allocationModel": "AllocationPool",
      "computeCapacity": {
        "cpu": {
          "allocated": 1000,
          "limit": 2000,
          "units": "MHz"
        },
        "memory": {
          "allocated": 1024,
          "limit": 2048,
          "units": "MB"
        }
      },
      "providerVdc": {
        "id": "urn:vcloud:providervdc:87654321-4321-4321-4321-cba987654321"
      },
      "nicQuota": 50,
      "networkQuota": 25,
      "vdcStorageProfiles": {},
      "isThinProvision": true,
      "isEnabled": true
    }
  ]
}
```

### Get Specific VDC

Retrieve details of a specific VDC by its ID.

**Request**
```http
GET /cloudapi/1.0.0/vdcs/{vdc_id}
Authorization: Bearer <jwt-token>
```

**Path Parameters**
| Parameter | Type | Description |
|-----------|------|-------------|
| `vdc_id` | string | VDC URN identifier (e.g., `urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc`) |

**Example Request**
```bash
curl -H "Authorization: Bearer <jwt-token>" \
  "https://your-ssvirt-instance.com/cloudapi/1.0.0/vdcs/urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc"
```

**Success Response** `200 OK`
```json
{
  "id": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "name": "Production VDC",
  "description": "Primary production environment for web applications",
  "allocationModel": "PayAsYouGo",
  "computeCapacity": {
    "cpu": {
      "allocated": 2000,
      "limit": 4000,
      "units": "MHz"
    },
    "memory": {
      "allocated": 2048,
      "limit": 4096,
      "units": "MB"
    }
  },
  "providerVdc": {
    "id": "urn:vcloud:providervdc:87654321-4321-4321-4321-cba987654321"
  },
  "nicQuota": 100,
  "networkQuota": 50,
  "vdcStorageProfiles": {},
  "isThinProvision": false,
  "isEnabled": true
}
```

## Data Model

### VDC Object

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique VDC identifier in URN format |
| `name` | string | Human-readable VDC name |
| `description` | string | Optional VDC description |
| `allocationModel` | string | Resource allocation model (`PayAsYouGo`, `AllocationPool`, `ReservationPool`, `Flex`) |
| `computeCapacity` | object | CPU and memory capacity information |
| `providerVdc` | object | Reference to the underlying provider VDC |
| `nicQuota` | integer | Maximum number of network interface cards |
| `networkQuota` | integer | Maximum number of networks |
| `vdcStorageProfiles` | object | Storage profiles (currently empty) |
| `isThinProvision` | boolean | Whether thin provisioning is enabled |
| `isEnabled` | boolean | Whether the VDC is enabled and available |

### Compute Capacity Object

| Field | Type | Description |
|-------|------|-------------|
| `cpu` | object | CPU resource information |
| `memory` | object | Memory resource information |

### Compute Resource Object

| Field | Type | Description |
|-------|------|-------------|
| `allocated` | integer | Currently allocated resources |
| `limit` | integer | Maximum allowed resources |
| `units` | string | Resource units (`MHz` for CPU, `MB` for memory) |

### Provider VDC Object

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Provider VDC identifier in URN format |

## Error Responses

All endpoints return consistent error responses following the CloudAPI standard:

### Error Object
```json
{
  "code": 400,
  "error": "Bad Request",
  "message": "Invalid VDC URN format",
  "details": "VDC ID must be a valid URN with prefix 'urn:vcloud:vdc:'"
}
```

### Common Error Codes

| Code | Error | Description |
|------|-------|-------------|
| 400 | Bad Request | Invalid request parameters or URN format |
| 401 | Unauthorized | Missing or invalid authentication token |
| 403 | Forbidden | User does not have access to the requested VDC |
| 404 | Not Found | VDC does not exist |
| 500 | Internal Server Error | Server or database error |

### Detailed Error Scenarios

#### 400 Bad Request
- Invalid VDC URN format
- Invalid pagination parameters
- Malformed request

**Example**
```json
{
  "code": 400,
  "error": "Bad Request",
  "message": "Invalid VDC URN format",
  "details": "VDC ID must be a valid URN with prefix 'urn:vcloud:vdc:'"
}
```

#### 401 Unauthorized
- Missing Authorization header
- Invalid or expired JWT token
- Token signature verification failure

**Example**
```json
{
  "code": 401,
  "error": "Unauthorized",
  "message": "Invalid or expired authentication token"
}
```

#### 403 Forbidden
- User does not belong to the organization that owns the VDC
- User's organization membership does not grant access to the VDC

**Example**
```json
{
  "code": 403,
  "error": "Forbidden",
  "message": "Access denied to VDC",
  "details": "User does not have permission to access this VDC"
}
```

#### 404 Not Found
- VDC with the specified ID does not exist
- VDC exists but user has no access (may appear as 404 for security)

**Example**
```json
{
  "code": 404,
  "error": "Not Found",
  "message": "VDC not found"
}
```

## Access Control

### Organization-Based Access
Users can access VDCs based on their organization membership:

- **Single Organization**: Users see VDCs belonging to their organization
- **Multiple Organizations**: Users see VDCs from all organizations they belong to
- **No Organization**: Users with no organization membership cannot access any VDCs

### Access Determination
The system determines VDC access through:
1. User authentication and JWT token validation
2. Organization membership lookup
3. VDC organization ownership verification
4. Access permission grant or denial

## Usage Examples

### Basic VDC Listing

```bash
#!/bin/bash

# 1. Authenticate and get session
SESSION_RESPONSE=$(curl -s -i -X POST \
  -H "Authorization: Basic $(echo -n 'myuser:mypassword' | base64)" \
  https://ssvirt.example.com/cloudapi/1.0.0/sessions)

# 2. Extract JWT token from Authorization header
JWT_TOKEN=$(echo "$SESSION_RESPONSE" | grep -i "authorization:" | sed 's/Authorization: Bearer //')

# 3. List VDCs
curl -H "Authorization: Bearer $JWT_TOKEN" \
  https://ssvirt.example.com/cloudapi/1.0.0/vdcs
```

### Paginated VDC Retrieval

```bash
#!/bin/bash

JWT_TOKEN="your-jwt-token-here"

# Get first page with 5 items
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ssvirt.example.com/cloudapi/1.0.0/vdcs?page=1&pageSize=5"

# Get second page
curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ssvirt.example.com/cloudapi/1.0.0/vdcs?page=2&pageSize=5"
```

### Get Specific VDC Details

```bash
#!/bin/bash

JWT_TOKEN="your-jwt-token-here"
VDC_ID="urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc"

curl -H "Authorization: Bearer $JWT_TOKEN" \
  "https://ssvirt.example.com/cloudapi/1.0.0/vdcs/$VDC_ID"
```

### Python Example

```python
import requests
import base64
import json

class SSVirtVDCClient:
    def __init__(self, base_url, username, password):
        self.base_url = base_url
        self.username = username
        self.password = password
        self.jwt_token = None
        
    def authenticate(self):
        """Authenticate and get JWT token"""
        credentials = base64.b64encode(f"{self.username}:{self.password}".encode()).decode()
        headers = {"Authorization": f"Basic {credentials}"}
        
        response = requests.post(f"{self.base_url}/cloudapi/1.0.0/sessions", headers=headers)
        response.raise_for_status()
        
        # Extract JWT token from Authorization header
        auth_header = response.headers.get("Authorization", "")
        if auth_header.startswith("Bearer "):
            self.jwt_token = auth_header[7:]  # Remove "Bearer " prefix
        else:
            raise Exception("Failed to extract JWT token from response")
    
    def list_vdcs(self, page=1, page_size=25):
        """List VDCs with pagination"""
        if not self.jwt_token:
            self.authenticate()
            
        headers = {"Authorization": f"Bearer {self.jwt_token}"}
        params = {"page": page, "pageSize": page_size}
        
        response = requests.get(f"{self.base_url}/cloudapi/1.0.0/vdcs", 
                              headers=headers, params=params)
        response.raise_for_status()
        return response.json()
    
    def get_vdc(self, vdc_id):
        """Get specific VDC by ID"""
        if not self.jwt_token:
            self.authenticate()
            
        headers = {"Authorization": f"Bearer {self.jwt_token}"}
        
        response = requests.get(f"{self.base_url}/cloudapi/1.0.0/vdcs/{vdc_id}", 
                              headers=headers)
        response.raise_for_status()
        return response.json()

# Usage example
client = SSVirtVDCClient("https://ssvirt.example.com", "myuser", "mypassword")

# List all accessible VDCs
vdcs = client.list_vdcs()
print(f"Found {vdcs['resultTotal']} VDCs")

for vdc in vdcs['values']:
    print(f"- {vdc['name']} ({vdc['id']})")
    
# Get details of first VDC
if vdcs['values']:
    first_vdc_id = vdcs['values'][0]['id']
    vdc_details = client.get_vdc(first_vdc_id)
    print(f"VDC '{vdc_details['name']}' has {vdc_details['computeCapacity']['cpu']['limit']} MHz CPU limit")
```

## Best Practices

### 1. Token Management
- Cache JWT tokens to avoid frequent authentication
- Implement token refresh logic for long-running applications
- Handle token expiration gracefully

### 2. Pagination
- Use appropriate page sizes (default 25, max 100)
- Implement proper pagination loops for large datasets
- Consider the total count for progress tracking

### 3. Error Handling
- Always check HTTP status codes
- Parse error responses for detailed information
- Implement retry logic for transient errors (5xx)
- Don't retry on authentication errors (4xx)

### 4. Performance
- Cache VDC information when appropriate
- Use specific VDC queries instead of listing when possible
- Implement reasonable request timeouts

### 5. Security
- Never log or expose JWT tokens
- Use HTTPS for all API communication
- Validate VDC URN formats before making requests
- Handle access denied responses appropriately

## Limitations

### Read-Only Access
This API provides read-only access to VDCs. For VDC management operations (create, update, delete), use the admin API endpoints with appropriate System Administrator privileges.

### Organization Scope
Users can only see VDCs from organizations they belong to. Cross-organization VDC access requires appropriate organization membership.

### No Advanced Filtering
The current API version does not support filtering by VDC properties (allocation model, status, etc.). Use client-side filtering if needed.

### Storage Profiles
VDC storage profiles are currently empty (`{}`) in the response as per the current system design.

## Related Documentation

- [CloudAPI Session Management](./session-api-guide.md)
- [Admin VDC API](./admin-vdc-api-guide.md)
- [Organization API](./organization-api-guide.md)
- [Error Handling Guide](./error-handling-guide.md)

## Support

For API support and questions:
- Review the error responses for detailed information
- Check authentication and authorization setup
- Verify VDC URN formats and organization membership
- Consult the enhancement proposal for detailed technical specifications