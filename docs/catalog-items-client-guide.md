# Catalog Items API Client Integration Guide

## Overview

This guide provides all the necessary information for client applications to
integrate with the new catalogItems API endpoints. The catalogItems API extends
the existing catalog functionality by providing access to individual catalog
items backed by OpenShift Templates.

## Prerequisites

- Client has already integrated with the existing catalog API endpoints (`/cloudapi/1.0.0/catalogs`)
- Client is familiar with VCD-style authentication and URN-based entity references
- Client can handle paginated API responses

## New API Endpoints

### 1. List Catalog Items

**Endpoint**: `GET /cloudapi/1.0.0/catalogs/{catalog_id}/catalogItems`

**Description**: Returns a paginated list of catalog items for the specified catalog.

**Path Parameters**:
- `catalog_id`: The URN of the catalog (e.g., `urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc`)

**Query Parameters**:
- `page` (optional): Page number, defaults to 1
- `pageSize` (optional): Number of items per page, defaults to 25, maximum 100

**Authentication**: Requires Bearer token in Authorization header

**Example Request**:
```http
GET /cloudapi/1.0.0/catalogs/urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc/catalogItems?page=1&pageSize=10
Authorization: Bearer <your-jwt-token>
```

**Example Response**:
```json
{
  "resultTotal": 15,
  "pageCount": 2,
  "page": 1,
  "pageSize": 10,
  "associations": null,
  "values": [
    {
      "id": "urn:vcloud:catalogitem:87654321-4321-4321-4321-210987654321",
      "name": "ubuntu-20.04-vm",
      "description": "Ubuntu 20.04 LTS Virtual Machine Template",
      "catalogId": "urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc",
      "isPublished": true,
      "isExpired": false,
      "creationDate": "2024-01-15T10:30:00Z",
      "size": 2147483648,
      "status": "RESOLVED",
      "entity": {
        "name": "ubuntu-20.04-vm",
        "description": "Ubuntu 20.04 LTS Virtual Machine Template",
        "type": "application/vnd.vmware.vcloud.vAppTemplate+xml",
        "numberOfVMs": 1,
        "numberOfCpus": 2,
        "memoryAllocation": 4294967296,
        "storageAllocation": 21474836480
      },
      "owner": {
        "name": "System",
        "id": ""
      },
      "catalog": {
        "name": "Public Templates",
        "id": "urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc"
      }
    }
  ]
}
```

### 2. Get Catalog Item

**Endpoint**: `GET /cloudapi/1.0.0/catalogs/{catalog_id}/catalogItems/{item_id}`

**Description**: Returns detailed information about a specific catalog item.

**Path Parameters**:
- `catalog_id`: The URN of the catalog
- `item_id`: The URN of the catalog item (e.g., `urn:vcloud:catalogitem:87654321-4321-4321-4321-210987654321`)

**Authentication**: Requires Bearer token in Authorization header

**Example Request**:
```http
GET /cloudapi/1.0.0/catalogs/urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc/catalogItems/urn:vcloud:catalogitem:87654321-4321-4321-4321-210987654321
Authorization: Bearer <your-jwt-token>
```

**Example Response**:
```json
{
  "id": "urn:vcloud:catalogitem:87654321-4321-4321-4321-210987654321",
  "name": "ubuntu-20.04-vm",
  "description": "Ubuntu 20.04 LTS Virtual Machine Template with KubeVirt integration",
  "catalogId": "urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc",
  "isPublished": true,
  "isExpired": false,
  "creationDate": "2024-01-15T10:30:00Z",
  "size": 2147483648,
  "status": "RESOLVED",
  "entity": {
    "name": "ubuntu-20.04-vm",
    "description": "Ubuntu 20.04 LTS Virtual Machine Template with KubeVirt integration",
    "type": "application/vnd.vmware.vcloud.vAppTemplate+xml",
    "numberOfVMs": 1,
    "numberOfCpus": 2,
    "memoryAllocation": 4294967296,
    "storageAllocation": 21474836480
  },
  "owner": {
    "name": "System",
    "id": ""
  },
  "catalog": {
    "name": "Public Templates",
    "id": "urn:vcloud:catalog:12345678-1234-1234-1234-123456789abc"
  }
}
```

## Data Schema

### CatalogItem Object

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `id` | string | URN identifier for the catalog item | Generated from OpenShift Template UID |
| `name` | string | Display name of the catalog item | Template `metadata.name` |
| `description` | string | Detailed description | Template `metadata.annotations["description"]` or `metadata.annotations["template.openshift.io/long-description"]` |
| `catalogId` | string | URN of the parent catalog | From request context |
| `isPublished` | boolean | Whether the item is published | Based on template labels |
| `isExpired` | boolean | Whether the item has expired | Always `false` for Templates |
| `creationDate` | string | ISO-8601 creation timestamp | Template `metadata.creationTimestamp` |
| `size` | integer | Estimated size in bytes | Computed from template specifications |
| `status` | string | Item status | Always `"RESOLVED"` for Templates |
| `entity` | CatalogItemEntity | Detailed entity information | Computed from template |
| `owner` | EntityRef | Owner reference | Parent catalog's owner |
| `catalog` | EntityRef | Parent catalog reference | Parent catalog info |

### CatalogItemEntity Object

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Entity name |
| `description` | string | Entity description |
| `type` | string | Always `"application/vnd.vmware.vcloud.vAppTemplate+xml"` |
| `numberOfVMs` | integer | Number of VMs in the template |
| `numberOfCpus` | integer | Total CPU count across all VMs |
| `memoryAllocation` | integer | Total memory in bytes |
| `storageAllocation` | integer | Total storage in bytes |

### EntityRef Object

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name |
| `id` | string | URN identifier |

## Error Responses

### 400 Bad Request
```json
{
  "error": "Bad Request",
  "message": "Invalid catalog ID format"
}
```

### 401 Unauthorized
```json
{
  "error": "Unauthorized",
  "message": "Authentication required"
}
```

### 404 Not Found
```json
{
  "error": "Not Found",
  "message": "Catalog not found"
}
```

### 503 Service Unavailable
```json
{
  "error": "Service Unavailable",
  "message": "Kubernetes API temporarily unavailable"
}
```

## Implementation Behavior

### Important Notes

1. **Read-Only Access**: The API provides read-only access. Catalog items cannot be created, updated, or deleted through these endpoints.

## Client Integration Recommendations

### Caching Strategy

- Cache catalog item lists for reasonable periods (5-15 minutes) as the underlying Templates don't change frequently
- Cache individual catalog items based on their ID
- Implement cache invalidation when Templates are known to have been updated

### Error Handling

- Handle 404 errors gracefully when catalog items might have been removed
- Implement retry logic for 503 errors with exponential backoff
- Validate URN formats before making requests

### Pagination

- Use the same pagination logic as the existing catalog API
- Default page size is 25, maximum is 100
- Always check `pageCount` to determine if more pages are available

### Example Integration Code

```javascript
class CatalogItemsClient {
  constructor(baseUrl, authToken) {
    this.baseUrl = baseUrl;
    this.authToken = authToken;
  }

  async listCatalogItems(catalogId, page = 1, pageSize = 25) {
    const url = `${this.baseUrl}/cloudapi/1.0.0/catalogs/${catalogId}/catalogItems?page=${page}&pageSize=${pageSize}`;
    
    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
        'Accept': 'application/json'
      }
    });

    if (!response.ok) {
      throw new Error(`Failed to list catalog items: ${response.status}`);
    }

    return await response.json();
  }

  async getCatalogItem(catalogId, itemId) {
    const url = `${this.baseUrl}/cloudapi/1.0.0/catalogs/${catalogId}/catalogItems/${itemId}`;
    
    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
        'Accept': 'application/json'
      }
    });

    if (!response.ok) {
      if (response.status === 404) {
        return null; // Item not found
      }
      throw new Error(`Failed to get catalog item: ${response.status}`);
    }

    return await response.json();
  }

  async getAllCatalogItems(catalogId) {
    const allItems = [];
    let page = 1;
    let hasMore = true;

    while (hasMore) {
      const result = await this.listCatalogItems(catalogId, page);
      allItems.push(...result.values);
      
      hasMore = page < result.pageCount;
      page++;
    }

    return allItems;
  }
}
```

## Migration from Existing Integration

If your client currently uses the catalog API, extending to support catalog items requires minimal changes:

1. **Authentication**: Use the same authentication mechanism (Bearer tokens)
2. **Error Handling**: Extend existing error handling patterns
3. **Pagination**: Use the same pagination structure
4. **URN Handling**: Extend existing URN validation to include `urn:vcloud:catalogitem:` prefix

The catalog items API is designed to be a natural extension of the existing catalog API, maintaining consistency in authentication, error responses, and data structures.