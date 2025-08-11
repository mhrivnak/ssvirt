# SSVirt API Server

This package implements the core API server for SSVirt, providing a VMware Cloud Director compatible REST API.

## Features

- **Gin Framework**: Fast HTTP router and middleware system
- **Authentication**: JWT-based authentication with role-based access control
- **Health Checks**: Health and readiness endpoints for Kubernetes deployments
- **Error Handling**: Structured error responses with consistent format
- **CORS Support**: Cross-origin resource sharing for web clients
- **Graceful Shutdown**: Clean shutdown handling for production environments

## API Endpoints

### Health & Status
- `GET /healthz` - Basic health check
- `GET /readyz` - Readiness check for Kubernetes
- `GET /api/v1/health` - API version health check
- `GET /api/v1/version` - Version information

### Legacy API (v1)
- `GET /api/v1/user/profile` - Get current user profile

### VMware Cloud Director Compatible API (CloudAPI)

#### Authentication
- `POST /cloudapi/1.0.0/sessions` - Create session (login)
- `GET /cloudapi/1.0.0/sessions/{sessionId}` - Get session details
- `DELETE /cloudapi/1.0.0/sessions/{sessionId}` - Delete session (logout)

#### User Management
- `GET /cloudapi/1.0.0/users` - List users with pagination and filtering
- `POST /cloudapi/1.0.0/users` - Create a new user account
- `GET /cloudapi/1.0.0/users/{id}` - Get user details by URN ID
- `PUT /cloudapi/1.0.0/users/{id}` - Update user account
- `DELETE /cloudapi/1.0.0/users/{id}` - Delete user account

#### Organization Management  
- `GET /cloudapi/1.0.0/orgs` - List organizations with pagination and filtering
- `POST /cloudapi/1.0.0/orgs` - Create a new organization
- `GET /cloudapi/1.0.0/orgs/{id}` - Get organization details by URN ID
- `PUT /cloudapi/1.0.0/orgs/{id}` - Update organization
- `DELETE /cloudapi/1.0.0/orgs/{id}` - Delete organization

#### Role Management
- `GET /cloudapi/1.0.0/roles` - List available roles
- `GET /cloudapi/1.0.0/roles/{id}` - Get role details by URN ID

#### Virtual Data Centers
- `GET /cloudapi/1.0.0/vdcs` - List accessible VDCs
- `GET /cloudapi/1.0.0/vdcs/{vdc_id}` - Get VDC details

#### Catalog Management
- `GET /cloudapi/1.0.0/catalogs` - List catalogs
- `POST /cloudapi/1.0.0/catalogs` - Create catalog
- `GET /cloudapi/1.0.0/catalogs/{catalogUrn}` - Get catalog details
- `DELETE /cloudapi/1.0.0/catalogs/{catalogUrn}` - Delete catalog
- `GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems` - List catalog items
- `GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems/{itemId}` - Get catalog item

#### vApp Management
- `GET /cloudapi/1.0.0/vdcs/{vdc_id}/vapps` - List vApps in VDC
- `GET /cloudapi/1.0.0/vapps/{vapp_id}` - Get vApp details
- `DELETE /cloudapi/1.0.0/vapps/{vapp_id}` - Delete vApp
- `POST /cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate` - Create vApp from template

#### Virtual Machine Operations
- `GET /cloudapi/1.0.0/vms/{vm_id}` - Get VM details

#### Admin API (System Administrator Only)
- `GET /api/admin/org/{orgId}/vdcs` - List VDCs in organization
- `POST /api/admin/org/{orgId}/vdcs` - Create VDC
- `GET /api/admin/org/{orgId}/vdcs/{vdcId}` - Get VDC details
- `PUT /api/admin/org/{orgId}/vdcs/{vdcId}` - Update VDC
- `DELETE /api/admin/org/{orgId}/vdcs/{vdcId}` - Delete VDC

## Configuration

The API server uses the following configuration options:

```yaml
api:
  port: 8080                    # HTTP port
  tls_cert: ""                  # TLS certificate file (optional)
  tls_key: ""                   # TLS private key file (optional)

auth:
  jwt_secret: "your-secret"     # JWT signing secret
  token_expiry: "24h"           # Token expiration duration

log:
  level: "info"                 # Log level (debug, info, warn, error)
  format: "json"                # Log format (json, text)
```

## Usage

```go
// Initialize dependencies
cfg, _ := config.Load()
db, _ := database.NewConnection(cfg)

// Initialize repositories
userRepo := repositories.NewUserRepository(db.DB)
roleRepo := repositories.NewRoleRepository(db.DB)
orgRepo := repositories.NewOrganizationRepository(db.DB)
vdcRepo := repositories.NewVDCRepository(db.DB)
catalogRepo := repositories.NewCatalogRepository(db.DB)
templateRepo := repositories.NewVAppTemplateRepository(db.DB)
vappRepo := repositories.NewVAppRepository(db.DB)
vmRepo := repositories.NewVMRepository(db.DB)

// Initialize authentication services
jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
authSvc := auth.NewService(userRepo, jwtManager)

// Create and start server
server := api.NewServer(cfg, db, authSvc, jwtManager, userRepo, roleRepo, 
    orgRepo, vdcRepo, catalogRepo, templateRepo, vappRepo, vmRepo)
server.Start()
```

## Error Response Format

All API errors follow a consistent format:

```json
{
  "code": 400,
  "error": "Bad Request",
  "message": "Invalid input provided",
  "details": "Field 'username' is required"
}
```

Success responses use this format:

```json
{
  "success": true,
  "data": {
    // Response data here
  }
}
```

## Middleware

- **CORS**: Enables cross-origin requests
- **Recovery**: Handles panics gracefully
- **Logging**: Request/response logging
- **JWT Authentication**: Token validation for protected endpoints

## Testing

The package includes comprehensive tests covering:
- Health and readiness endpoints
- Authentication flows
- Error handling
- CORS functionality
- Middleware behavior

Run tests with:
```bash
make test
```