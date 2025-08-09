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
- `GET /health` - Basic health check
- `GET /ready` - Readiness check for Kubernetes
- `GET /api/v1/health` - API version health check
- `GET /api/v1/version` - Version information

### Authentication Required
- `GET /api/v1/user/profile` - Get current user profile

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