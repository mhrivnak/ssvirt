# Sessions API VMware Cloud Director Compliance

## Status
- **Status**: Draft
- **Created**: 2025-08-09
- **Tracking Issue**: TBD

## Summary

This enhancement updates the existing session/login API to conform exactly to the VMware Cloud Director (VCD) API specification for session management. The current implementation uses a custom session API structure that needs to be replaced with the standard VCD CloudAPI session endpoints.

## Background

The current session API uses custom endpoints (`/api/sessions`) with a non-standard response format. To achieve full VCD API compatibility, we need to implement the standard CloudAPI session endpoints with the exact response structure defined in the VCD OpenAPI specification.

Current API endpoints to be replaced:
- `POST /api/sessions` → `POST /cloudapi/1.0.0/sessions`
- `DELETE /api/sessions` → `DELETE /cloudapi/1.0.0/sessions/{id}`
- `GET /api/session` → `GET /cloudapi/1.0.0/sessions`

## Motivation

1. **VCD API Compatibility**: Full compliance with VMware Cloud Director API specification
2. **Standard Session Management**: Use industry-standard session handling patterns
3. **Client Compatibility**: Enable existing VCD client tools and SDKs to work with SSVirt
4. **Future-Proofing**: Align with VMware's authentication and session patterns

## Design

### API Endpoints

#### 1. Create Session (Login)
```
POST /cloudapi/1.0.0/sessions
```

**Authentication**: Basic Authentication with username/password in Authorization header
- Format: `Authorization: Basic base64(username:password)`

**Response (200 OK)**:
```json
{
  "id": "urn:vcloud:session:12345678-1234-1234-1234-123456789abc",
  "site": {
    "name": "SSVirt Provider",
    "id": "urn:vcloud:site:12345678-1234-1234-1234-123456789abc"
  },
  "user": {
    "name": "admin",
    "id": "urn:vcloud:user:12345678-1234-1234-1234-123456789abc"
  },
  "org": {
    "name": "Provider",
    "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc"
  },
  "operatingOrg": {
    "name": "Provider",
    "id": "urn:vcloud:org:12345678-1234-1234-1234-123456789abc"
  },
  "location": "us-west-1",
  "roles": [
    "System Administrator"
  ],
  "roleRefs": [
    {
      "name": "System Administrator",
      "id": "urn:vcloud:role:12345678-1234-1234-1234-123456789abc"
    }
  ],
  "sessionIdleTimeoutMinutes": 30
}
```

**Error Responses**:
- `401 Unauthorized`: Invalid credentials
- `403 Forbidden`: User account disabled

#### 2. Get Current Session
```
GET /cloudapi/1.0.0/sessions/{sessionId}
```

**Authentication**: Bearer token from session creation

**Security**: Users can only retrieve sessions that belong to them. The sessionId in the URL must match the session ID from the bearer token.

**Response**: Same structure as session creation response

**Error Responses**:
- `401 Unauthorized`: Invalid or missing bearer token
- `403 Forbidden`: Attempting to access another user's session

#### 3. Delete Session (Logout)
```
DELETE /cloudapi/1.0.0/sessions/{sessionId}
```

**Authentication**: Bearer token from session creation

**Security**: Users can only delete sessions that belong to them. The sessionId in the URL must match the session ID from the bearer token.

**Response (204 No Content)**: Empty body on successful logout

**Error Responses**:
- `401 Unauthorized`: Invalid or missing bearer token
- `403 Forbidden`: Attempting to delete another user's session

### Data Models

#### Session Model
```go
type Session struct {
    ID                        string             `json:"id"`
    Site                     EntityRef          `json:"site"`
    User                     EntityRef          `json:"user"`
    Org                      EntityRef          `json:"org"`
    OperatingOrg             EntityRef          `json:"operatingOrg"`
    Location                 string             `json:"location"`
    Roles                    []string           `json:"roles"`
    RoleRefs                 []EntityRef        `json:"roleRefs"`
    SessionIdleTimeoutMinutes int               `json:"sessionIdleTimeoutMinutes"`
}

```

#### EntityRef (already exists)
```go
type EntityRef struct {
    Name string `json:"name"`
    ID   string `json:"id"`
}
```

### Implementation Plan

#### Phase 1: Session Handler Implementation
1. **Create new session handlers** in `pkg/api/handlers/sessions.go`:
   - `CreateSession()` - Handle POST /cloudapi/1.0.0/sessions
   - `GetCurrentSession()` - Handle GET /cloudapi/1.0.0/sessions/{id}
   - `DeleteSession()` - Handle DELETE /cloudapi/1.0.0/sessions/{id}

2. **Authentication parsing**:
   - Parse Basic Authentication from Authorization header
   - Extract username/password from base64 encoded credentials
   - Validate credentials using existing auth service

3. **Session management**:
   - Generate session URN with format `urn:vcloud:session:{uuid}`
   - Create JWT token for session authentication
   - Store session metadata for later retrieval

4. **Session security**:
   - Validate that users can only access their own sessions
   - Compare session ID from URL path with session ID from JWT token
   - Return 403 Forbidden if session ownership doesn't match
   - Ensure session isolation between different users

#### Phase 2: Response Structure Implementation
1. **Session response builder**:
   - Populate user information from authenticated user
   - Get user's organization and roles
   - Build EntityRef structures for all referenced entities
   - Apply default/hardcoded values for site, location, operatingOrg

2. **Default values** (as requested):
   - **Site**: 
     - Name: "SSVirt Provider"
     - ID: Generate static URN `urn:vcloud:site:` + fixed UUID
   - **Location**: "us-west-1" (simple string value)
   - **OperatingOrg**: Same as user's primary organization
   - **SessionIdleTimeoutMinutes**: 30 (configurable)

#### Phase 3: Route Updates
1. **Remove existing session routes** from `pkg/api/server.go`:
   - Remove `/api/sessions` POST handler
   - Remove `/api/sessions` DELETE handler  
   - Remove `/api/session` GET handler

2. **Add new CloudAPI session routes**:
   - `POST /cloudapi/1.0.0/sessions` → `sessionHandlers.CreateSession`
   - `GET /cloudapi/1.0.0/sessions/:id` → `sessionHandlers.GetCurrentSession`
   - `DELETE /cloudapi/1.0.0/sessions/:id` → `sessionHandlers.DeleteSession`

#### Phase 4: Middleware Updates
1. **JWT authentication middleware** updates:
   - Extract session ID from JWT token
   - Validate session is still active
   - Populate request context with session information

2. **CORS and content-type handling**:
   - Ensure proper CORS headers for CloudAPI endpoints
   - Handle both JSON and XML content types (VCD supports both)

#### Phase 5: Testing and Documentation
1. **Update API tests**:
   - Replace existing session tests with VCD-compliant tests
   - Test Basic Authentication parsing
   - Test session response structure compliance
   - Test error scenarios (401, 403)

2. **Update documentation**:
   - Update API documentation to reflect new endpoints
   - Document authentication flow
   - Provide examples of client usage

### Breaking Changes

This is a **breaking change** that will affect existing clients:

1. **Endpoint paths changed**:
   - `/api/sessions` → `/cloudapi/1.0.0/sessions`
   - `/api/session` → `/cloudapi/1.0.0/sessions`

2. **Authentication method changed**:
   - JSON body with username/password → Basic Authentication header

3. **Response structure completely changed**:
   - Custom response format → VCD Session object format

4. **Session deletion requires session ID**:
   - `/api/sessions` DELETE → `/cloudapi/1.0.0/sessions/{id}` DELETE

### Migration Strategy

**No migration or backward compatibility will be provided.** The old session API will be completely removed and replaced with the new VCD-compliant API:

1. **Direct replacement**: Remove all existing session endpoints (`/api/sessions`, `/api/session`) immediately
2. **No deprecation period**: The old API will be removed in the same change that implements the new API
3. **Clean break**: No attempt to maintain compatibility with the old API format
4. **Full replacement**: All session-related functionality will use the new VCD-compliant endpoints only

This approach ensures:
- Clean, maintainable codebase without legacy API baggage
- Full VCD API compliance from day one
- No confusion between old and new authentication methods
- Simplified testing and documentation

### Configuration

Add new configuration options:

```yaml
session:
  idle_timeout_minutes: 30
  site:
    name: "SSVirt Provider"
    id: "urn:vcloud:site:00000000-0000-0000-0000-000000000001"
  location: "us-west-1"
```

### Files to be Modified

1. **New files**:
   - `pkg/api/handlers/sessions.go` - Session handlers
   - `pkg/database/models/session.go` - Session model (if persistent sessions needed)

2. **Modified files**:
   - `pkg/api/server.go` - Route registration
   - `pkg/api/middleware.go` - Authentication middleware updates
   - `test/unit/api_test.go` - Updated session tests
   - `docs/api/` - API documentation updates

### Success Criteria

1. All new session endpoints return responses that exactly match VCD API specification
2. Basic Authentication parsing works correctly
3. Session management (create, get, delete) functions properly
4. **Session security**: Users can only access and delete their own sessions, not others'
5. All tests pass with new session API
6. Error responses match VCD API error format
7. 403 Forbidden responses are properly returned when users attempt to access other users' sessions

### Risks and Mitigation

1. **Risk**: Breaking existing clients
   - **Mitigation**: Provide clear migration documentation and temporary backward compatibility

2. **Risk**: Authentication security issues
   - **Mitigation**: Thorough testing of Basic Auth parsing, secure credential handling

3. **Risk**: Performance impact from additional response building
   - **Mitigation**: Optimize entity reference population, consider caching

### Future Considerations

1. **Persistent sessions**: Consider storing session metadata in database for scalability
2. **Session management**: Implement session timeout and cleanup
3. **Multi-tenancy**: Support for organization-specific sessions
4. **SSO integration**: Prepare for future SAML/OIDC authentication methods