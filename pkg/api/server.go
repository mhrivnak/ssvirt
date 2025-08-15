package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/api/handlers"
	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
	"github.com/mhrivnak/ssvirt/pkg/services"
)

// Server represents the API server
type Server struct {
	config          *config.Config
	db              *database.DB
	authSvc         *auth.Service
	jwtManager      *auth.JWTManager
	userRepo        *repositories.UserRepository
	roleRepo        *repositories.RoleRepository
	orgRepo         *repositories.OrganizationRepository
	vdcRepo         *repositories.VDCRepository
	catalogRepo     *repositories.CatalogRepository
	templateRepo    *repositories.VAppTemplateRepository
	vappRepo        *repositories.VAppRepository
	vmRepo          *repositories.VMRepository
	catalogItemRepo *repositories.CatalogItemRepository
	templateService services.TemplateServiceInterface
	k8sService      services.KubernetesService
	// CloudAPI handlers
	userHandlers        *handlers.UserHandlers
	roleHandlers        *handlers.RoleHandlers
	orgHandlers         *handlers.OrgHandlers
	vdcHandlers         *handlers.VDCHandlers
	vdcPublicHandlers   *handlers.VDCPublicHandlers
	catalogHandlers     *handlers.CatalogHandlers
	catalogItemHandlers *handlers.CatalogItemHandler
	sessionHandlers     *handlers.SessionHandlers
	vmCreationHandlers  *handlers.VMCreationHandlers
	vappHandlers        *handlers.VAppHandlers
	vmHandlers          *handlers.VMHandlers
	powerMgmtHandlers   *handlers.PowerManagementHandler
	router              *gin.Engine
	httpServer          *http.Server
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config, db *database.DB, authSvc *auth.Service, jwtManager *auth.JWTManager, userRepo *repositories.UserRepository, roleRepo *repositories.RoleRepository, orgRepo *repositories.OrganizationRepository, vdcRepo *repositories.VDCRepository, catalogRepo *repositories.CatalogRepository, templateRepo *repositories.VAppTemplateRepository, vappRepo *repositories.VAppRepository, vmRepo *repositories.VMRepository, templateService services.TemplateServiceInterface, k8sService services.KubernetesService) *Server {
	// Validate required parameters
	if templateService == nil {
		panic("templateService cannot be nil")
	}
	if userRepo == nil {
		panic("userRepo cannot be nil")
	}

	// Create catalog item repository
	catalogItemRepo := repositories.NewCatalogItemRepository(templateService, catalogRepo)

	server := &Server{
		config:          cfg,
		db:              db,
		authSvc:         authSvc,
		jwtManager:      jwtManager,
		userRepo:        userRepo,
		roleRepo:        roleRepo,
		orgRepo:         orgRepo,
		vdcRepo:         vdcRepo,
		catalogRepo:     catalogRepo,
		templateRepo:    templateRepo,
		vappRepo:        vappRepo,
		vmRepo:          vmRepo,
		catalogItemRepo: catalogItemRepo,
		templateService: templateService,
		k8sService:      k8sService,
		// Initialize CloudAPI handlers
		userHandlers:        handlers.NewUserHandlers(userRepo, orgRepo, roleRepo),
		roleHandlers:        handlers.NewRoleHandlers(roleRepo),
		orgHandlers:         handlers.NewOrgHandlers(orgRepo),
		vdcHandlers:         handlers.NewVDCHandlers(vdcRepo, orgRepo, userRepo, k8sService),
		vdcPublicHandlers:   handlers.NewVDCPublicHandlers(vdcRepo),
		catalogHandlers:     handlers.NewCatalogHandlers(catalogRepo, catalogItemRepo, orgRepo, k8sService),
		catalogItemHandlers: handlers.NewCatalogItemHandler(catalogItemRepo),
		sessionHandlers:     handlers.NewSessionHandlers(userRepo, authSvc, jwtManager, cfg),
		vmCreationHandlers:  handlers.NewVMCreationHandlers(vdcRepo, vappRepo, catalogItemRepo, catalogRepo, k8sService),
		vappHandlers:        handlers.NewVAppHandlers(vappRepo, vdcRepo, vmRepo),
		vmHandlers:          handlers.NewVMHandlers(vmRepo, vappRepo, vdcRepo),
		powerMgmtHandlers:   createPowerManagementHandler(vmRepo, k8sService),
	}

	// Configure gin mode based on log level
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	server.setupRoutes()
	return server
}

// createPowerManagementHandler creates a power management handler, handling nil k8sService case
func createPowerManagementHandler(vmRepo *repositories.VMRepository, k8sService services.KubernetesService) *handlers.PowerManagementHandler {
	if k8sService == nil {
		// For tests without k8s service, create with nil client
		return handlers.NewPowerManagementHandler(vmRepo, nil, slog.Default())
	}
	return handlers.NewPowerManagementHandler(vmRepo, k8sService.GetClient(), slog.Default())
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	s.router = gin.New()

	// Global middleware
	s.router.Use(gin.Logger())
	s.router.Use(gin.Recovery())
	s.router.Use(s.corsMiddleware())
	s.router.Use(s.errorHandlerMiddleware())

	// Health endpoints
	s.router.GET("/healthz", s.healthHandler)
	s.router.GET("/readyz", s.readinessHandler)

	// API version 1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Public endpoints (no authentication required)
		v1.GET("/health", s.healthHandler)
		v1.GET("/version", s.versionHandler)

		// Protected endpoints (authentication required)
		protected := v1.Group("/")
		protected.Use(auth.JWTMiddleware(s.jwtManager))
		{
			// User endpoints
			protected.GET("/user/profile", s.userProfileHandler)
		}
	}

	// CloudAPI endpoints (VMware Cloud Director compatible)
	cloudAPIRoot := s.router.Group("/cloudapi/1.0.0")
	{
		// Public session endpoint (Basic Auth for login)
		cloudAPIRoot.POST("/sessions", s.sessionHandlers.CreateSession) // POST /cloudapi/1.0.0/sessions - create session (login)

		// Protected CloudAPI endpoints (require JWT middleware)
		cloudAPI := cloudAPIRoot.Group("/")
		cloudAPI.Use(auth.JWTMiddleware(s.jwtManager))
		{
			// Session management
			cloudAPI.GET("/sessions/:sessionId", s.sessionHandlers.GetCurrentSession) // GET /cloudapi/1.0.0/sessions/{sessionId} - get session
			cloudAPI.DELETE("/sessions/:sessionId", s.sessionHandlers.DeleteSession)  // DELETE /cloudapi/1.0.0/sessions/{sessionId} - delete session

			// Users API
			cloudAPI.GET("/users", s.userHandlers.ListUsers)         // GET /cloudapi/1.0.0/users - list users
			cloudAPI.POST("/users", s.userHandlers.CreateUser)       // POST /cloudapi/1.0.0/users - create user
			cloudAPI.GET("/users/:id", s.userHandlers.GetUser)       // GET /cloudapi/1.0.0/users/{id} - get user
			cloudAPI.PUT("/users/:id", s.userHandlers.UpdateUser)    // PUT /cloudapi/1.0.0/users/{id} - update user
			cloudAPI.DELETE("/users/:id", s.userHandlers.DeleteUser) // DELETE /cloudapi/1.0.0/users/{id} - delete user

			// Roles API
			cloudAPI.GET("/roles", s.roleHandlers.ListRoles)   // GET /cloudapi/1.0.0/roles - list roles
			cloudAPI.GET("/roles/:id", s.roleHandlers.GetRole) // GET /cloudapi/1.0.0/roles/{id} - get role

			// Organizations API
			cloudAPI.GET("/orgs", s.orgHandlers.ListOrgs)         // GET /cloudapi/1.0.0/orgs - list organizations
			cloudAPI.POST("/orgs", s.orgHandlers.CreateOrg)       // POST /cloudapi/1.0.0/orgs - create organization
			cloudAPI.GET("/orgs/:id", s.orgHandlers.GetOrg)       // GET /cloudapi/1.0.0/orgs/{id} - get organization
			cloudAPI.PUT("/orgs/:id", s.orgHandlers.UpdateOrg)    // PUT /cloudapi/1.0.0/orgs/{id} - update organization
			cloudAPI.DELETE("/orgs/:id", s.orgHandlers.DeleteOrg) // DELETE /cloudapi/1.0.0/orgs/{id} - delete organization

			// VDCs API (Public - read-only access for authenticated users)
			cloudAPI.GET("/vdcs", s.vdcPublicHandlers.ListVDCs)       // GET /cloudapi/1.0.0/vdcs - list accessible VDCs
			cloudAPI.GET("/vdcs/:vdc_id", s.vdcPublicHandlers.GetVDC) // GET /cloudapi/1.0.0/vdcs/{vdc_id} - get VDC

			// Catalogs API
			cloudAPI.GET("/catalogs", s.catalogHandlers.ListCatalogs)                 // GET /cloudapi/1.0.0/catalogs - list catalogs
			cloudAPI.POST("/catalogs", s.catalogHandlers.CreateCatalog)               // POST /cloudapi/1.0.0/catalogs - create catalog
			cloudAPI.GET("/catalogs/:catalogUrn", s.catalogHandlers.GetCatalog)       // GET /cloudapi/1.0.0/catalogs/{catalogUrn} - get catalog
			cloudAPI.DELETE("/catalogs/:catalogUrn", s.catalogHandlers.DeleteCatalog) // DELETE /cloudapi/1.0.0/catalogs/{catalogUrn} - delete catalog

			// Catalog Items API
			cloudAPI.GET("/catalogs/:catalogUrn/catalogItems", s.catalogItemHandlers.ListCatalogItems)       // GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems - list catalog items
			cloudAPI.GET("/catalogs/:catalogUrn/catalogItems/:itemId", s.catalogItemHandlers.GetCatalogItem) // GET /cloudapi/1.0.0/catalogs/{catalogUrn}/catalogItems/{itemId} - get catalog item

			// VM Creation API
			cloudAPI.POST("/vdcs/:vdc_id/actions/instantiateTemplate", s.vmCreationHandlers.InstantiateTemplate) // POST /cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate - create vApp from template

			// vApps API
			cloudAPI.GET("/vdcs/:vdc_id/vapps", s.vappHandlers.ListVApps) // GET /cloudapi/1.0.0/vdcs/{vdc_id}/vapps - list vApps in VDC
			cloudAPI.GET("/vapps/:vapp_id", s.vappHandlers.GetVApp)       // GET /cloudapi/1.0.0/vapps/{vapp_id} - get vApp
			cloudAPI.DELETE("/vapps/:vapp_id", s.vappHandlers.DeleteVApp) // DELETE /cloudapi/1.0.0/vapps/{vapp_id} - delete vApp

			// VMs API
			cloudAPI.GET("/vms/:vm_id", s.vmHandlers.GetVM) // GET /cloudapi/1.0.0/vms/{vm_id} - get VM

			// VM Power Management API (only register if k8sService is available)
			if s.k8sService != nil {
				cloudAPI.POST("/vms/:id/actions/powerOn", s.powerMgmtHandlers.PowerOn)   // POST /cloudapi/1.0.0/vms/{id}/actions/powerOn - power on VM
				cloudAPI.POST("/vms/:id/actions/powerOff", s.powerMgmtHandlers.PowerOff) // POST /cloudapi/1.0.0/vms/{id}/actions/powerOff - power off VM
			}
		}

	}

	// Admin API endpoints (System Administrator only)
	adminAPIRoot := s.router.Group("/api/admin")
	adminAPIRoot.Use(auth.JWTMiddleware(s.jwtManager))
	adminAPIRoot.Use(handlers.RequireSystemAdmin(s.userRepo))
	{
		// VDC Management API (System Administrator only)
		adminAPIRoot.GET("/org/:orgId/vdcs", s.vdcHandlers.ListVDCs)            // GET /api/admin/org/{orgId}/vdcs - list VDCs in organization
		adminAPIRoot.POST("/org/:orgId/vdcs", s.vdcHandlers.CreateVDC)          // POST /api/admin/org/{orgId}/vdcs - create VDC
		adminAPIRoot.GET("/org/:orgId/vdcs/:vdcId", s.vdcHandlers.GetVDC)       // GET /api/admin/org/{orgId}/vdcs/{vdcId} - get VDC
		adminAPIRoot.PUT("/org/:orgId/vdcs/:vdcId", s.vdcHandlers.UpdateVDC)    // PUT /api/admin/org/{orgId}/vdcs/{vdcId} - update VDC
		adminAPIRoot.DELETE("/org/:orgId/vdcs/:vdcId", s.vdcHandlers.DeleteVDC) // DELETE /api/admin/org/{orgId}/vdcs/{vdcId} - delete VDC
	}

	// Legacy API endpoints (DEPRECATED - use CloudAPI endpoints instead)
	apiRoot := s.router.Group("/api")
	{
		// Protected legacy endpoints (require JWT middleware)
		protected := apiRoot.Group("/")
		protected.Use(auth.JWTMiddleware(s.jwtManager))
		{
			// Legacy endpoints (DEPRECATED - use CloudAPI endpoints instead)
			// TODO: These legacy endpoints have been temporarily commented out during
			// the migration to VMware Cloud Director API spec with URN-based IDs.
			// They will be updated or permanently removed in future iterations.

			// Organization endpoints
			// protected.GET("/org", s.organizationsHandler)                        // GET /api/org - list organizations
			// protected.GET("/org/:org-id", s.organizationHandler)                 // GET /api/org/{org-id} - get organization
			// protected.GET("/org/:org-id/vdcs/query", s.vdcQueryHandler)          // GET /api/org/{org-id}/vdcs/query - list VDCs in organization

			// VDC endpoints
			// protected.GET("/vdc/:vdc-id", s.vdcHandler)                                                     // GET /api/vdc/{vdc-id} - get VDC
			// protected.GET("/vdc/:vdc-id/vApps/query", s.vAppsQueryHandler)                                  // GET /api/vdc/{vdc-id}/vApps/query - list vApps in VDC
			// protected.POST("/vdc/:vdc-id/action/instantiateVAppTemplate", s.instantiateVAppTemplateHandler) // POST /api/vdc/{vdc-id}/action/instantiateVAppTemplate - instantiate vApp template

			// vApp endpoints
			// protected.GET("/vApp/:vapp-id", s.vAppHandler)          // GET /api/vApp/{vapp-id} - get vApp
			// protected.DELETE("/vApp/:vapp-id", s.deleteVAppHandler) // DELETE /api/vApp/{vapp-id} - delete vApp

			// VM endpoints (vApp-centric following VMware Cloud Director API spec)
			// protected.GET("/vApp/:vapp-id/vms/query", s.vappVMsQueryHandler) // GET /api/vApp/{vapp-id}/vms/query - list VMs in vApp
			// protected.POST("/vApp/:vapp-id/vms", s.createVMInVAppHandler)    // POST /api/vApp/{vapp-id}/vms - create VM in vApp
			// protected.GET("/vm/:vm-id", s.vmHandler)                         // GET /api/vm/{vm-id} - get VM
			// protected.PUT("/vm/:vm-id", s.updateVMHandler)                   // PUT /api/vm/{vm-id} - update VM
			// protected.DELETE("/vm/:vm-id", s.deleteVMHandler)                // DELETE /api/vm/{vm-id} - delete VM

			// VM power operation endpoints
			// protected.POST("/vm/:vm-id/power/action/powerOn", s.powerOnVMHandler)   // POST /api/vm/{vm-id}/power/action/powerOn - power on VM
			// protected.POST("/vm/:vm-id/power/action/powerOff", s.powerOffVMHandler) // POST /api/vm/{vm-id}/power/action/powerOff - power off VM
			// protected.POST("/vm/:vm-id/power/action/suspend", s.suspendVMHandler)   // POST /api/vm/{vm-id}/power/action/suspend - suspend VM
			// protected.POST("/vm/:vm-id/power/action/reset", s.resetVMHandler)       // POST /api/vm/{vm-id}/power/action/reset - reset VM
		}
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	address := fmt.Sprintf(":%d", s.config.API.Port)
	log.Printf("Starting API server on %s", address)

	s.httpServer = &http.Server{
		Addr:         address,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if s.config.API.TLSCert != "" && s.config.API.TLSKey != "" {
		// Verify TLS certificate and key files exist and are readable
		if _, err := os.Stat(s.config.API.TLSCert); err != nil {
			log.Printf("TLS certificate file not found or unreadable: %v", err)
			return fmt.Errorf("TLS certificate file error: %w", err)
		}
		if _, err := os.Stat(s.config.API.TLSKey); err != nil {
			log.Printf("TLS key file not found or unreadable: %v", err)
			return fmt.Errorf("TLS key file error: %w", err)
		}

		log.Println("Starting HTTPS server")
		return s.httpServer.ListenAndServeTLS(s.config.API.TLSCert, s.config.API.TLSKey)
	}

	log.Println("Starting HTTP server")
	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	log.Println("Shutting down API server...")
	return s.httpServer.Shutdown(ctx)
}

// GetRouter returns the gin router (useful for testing)
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"version":   "1.0.0",
		"database":  "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// readinessHandler handles readiness check requests
func (s *Server) readinessHandler(c *gin.Context) {
	services := gin.H{
		"database": "ready",
		"auth":     "ready",
	}

	// Check Kubernetes service status
	if s.k8sService == nil {
		services["k8s"] = "disabled"
	} else {
		ctx := c.Request.Context()
		if err := s.k8sService.HealthCheck(ctx); err != nil {
			services["k8s"] = "unavailable"
		} else {
			services["k8s"] = "ready"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ready":     true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"services":  services,
	})
}

// versionHandler handles version requests
func (s *Server) versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    "1.0.0",
		"build_time": "dev",
		"go_version": runtime.Version(),
		"git_commit": "dev",
	})
}

// userProfileHandler handles user profile requests
func (s *Server) userProfileHandler(c *gin.Context) {
	// Extract user claims from JWT middleware
	claims, exists := c.Get(auth.ClaimsContextKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	userClaims, ok := claims.(*auth.Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
		return
	}

	// Get user details from repository
	user, err := s.userRepo.GetByID(userClaims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Error retrieving user %s: %v", userClaims.UserID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"full_name": user.FullName,
		},
	})
}
