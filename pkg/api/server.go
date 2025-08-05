package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
)

// Server represents the API server
type Server struct {
	config       *config.Config
	db           *database.DB
	authSvc      *auth.Service
	jwtManager   *auth.JWTManager
	userRepo     *repositories.UserRepository
	orgRepo      *repositories.OrganizationRepository
	vdcRepo      *repositories.VDCRepository
	catalogRepo  *repositories.CatalogRepository
	templateRepo *repositories.VAppTemplateRepository
	vappRepo     *repositories.VAppRepository
	vmRepo       *repositories.VMRepository
	router       *gin.Engine
	httpServer   *http.Server
}

// NewServer creates a new API server instance
func NewServer(cfg *config.Config, db *database.DB, authSvc *auth.Service, jwtManager *auth.JWTManager, userRepo *repositories.UserRepository, orgRepo *repositories.OrganizationRepository, vdcRepo *repositories.VDCRepository, catalogRepo *repositories.CatalogRepository, templateRepo *repositories.VAppTemplateRepository, vappRepo *repositories.VAppRepository, vmRepo *repositories.VMRepository) *Server {
	server := &Server{
		config:       cfg,
		db:           db,
		authSvc:      authSvc,
		jwtManager:   jwtManager,
		userRepo:     userRepo,
		orgRepo:      orgRepo,
		vdcRepo:      vdcRepo,
		catalogRepo:  catalogRepo,
		templateRepo: templateRepo,
		vappRepo:     vappRepo,
		vmRepo:       vmRepo,
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

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	s.router = gin.New()

	// Global middleware
	s.router.Use(gin.Logger())
	s.router.Use(gin.Recovery())
	s.router.Use(s.corsMiddleware())
	s.router.Use(s.errorHandlerMiddleware())

	// Health endpoints
	s.router.GET("/health", s.healthHandler)
	s.router.GET("/ready", s.readinessHandler)

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

	// Authentication endpoints (at root level for VMware Cloud Director compatibility)
	apiRoot := s.router.Group("/api")
	{
		// Public authentication endpoint (no JWT middleware)
		apiRoot.POST("/sessions", s.createSessionHandler) // POST /api/sessions - login

		// Protected authentication endpoints (require JWT middleware)
		protected := apiRoot.Group("/")
		protected.Use(auth.JWTMiddleware(s.jwtManager))
		{
			// Session management
			protected.DELETE("/sessions", s.deleteSessionHandler) // DELETE /api/sessions - logout
			protected.GET("/session", s.getSessionHandler)        // GET /api/session - get current session

			// Organization endpoints
			protected.GET("/org", s.organizationsHandler)        // GET /api/org - list organizations
			protected.GET("/org/:org-id", s.organizationHandler) // GET /api/org/{org-id} - get organization

			// VDC endpoints
			protected.GET("/vdc/:vdc-id", s.vdcHandler)                                                     // GET /api/vdc/{vdc-id} - get VDC
			protected.POST("/vdc/:vdc-id/action/instantiateVAppTemplate", s.instantiateVAppTemplateHandler) // POST /api/vdc/{vdc-id}/action/instantiateVAppTemplate - instantiate vApp template

			// Catalog endpoints
			protected.GET("/catalogs/query", s.catalogsQueryHandler)                             // GET /api/catalogs/query - list catalogs
			protected.GET("/catalog/:catalog-id", s.catalogHandler)                              // GET /api/catalog/{catalog-id} - get catalog
			protected.GET("/catalog/:catalog-id/catalogItems/query", s.catalogItemsQueryHandler) // GET /api/catalog/{catalog-id}/catalogItems/query - list catalog items

			// vApp endpoints
			protected.GET("/vApps/query", s.vAppsQueryHandler)              // GET /api/vApps/query - list vApps
			protected.GET("/vApp/:vapp-id", s.vAppHandler)                  // GET /api/vApp/{vapp-id} - get vApp
			protected.DELETE("/vApp/:vapp-id", s.deleteVAppHandler)         // DELETE /api/vApp/{vapp-id} - delete vApp
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
