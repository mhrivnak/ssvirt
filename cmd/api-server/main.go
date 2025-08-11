package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mhrivnak/ssvirt/pkg/api"
	"github.com/mhrivnak/ssvirt/pkg/auth"
	"github.com/mhrivnak/ssvirt/pkg/config"
	"github.com/mhrivnak/ssvirt/pkg/database"
	"github.com/mhrivnak/ssvirt/pkg/database/repositories"
	"github.com/mhrivnak/ssvirt/pkg/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.NewConnection(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run database migrations
	if err := db.AutoMigrate(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database connection: %v", closeErr)
		}
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Bootstrap default data (roles and organizations)
	if err := db.BootstrapDefaultData(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database connection: %v", closeErr)
		}
		log.Fatalf("Failed to bootstrap default data: %v", err)
	}

	// Bootstrap initial admin user if configured
	if err := db.BootstrapInitialAdmin(cfg); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database connection: %v", closeErr)
		}
		log.Fatalf("Failed to bootstrap initial admin: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

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

	// Initialize template service
	templateService, err := services.NewTemplateService()
	if err != nil {
		log.Fatalf("Failed to create template service: %v", err)
	}

	// Initialize Kubernetes service
	k8sService, err := services.NewKubernetesService("openshift")
	if err != nil {
		log.Printf("Warning: Failed to initialize Kubernetes service: %v", err)
		log.Println("Continuing without Kubernetes integration...")
	}

	// Create cancelable context for services
	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	defer serviceCancel()

	// Start template service cache in background
	go func() {
		if err := templateService.Start(serviceCtx); err != nil {
			log.Printf("Template service cache error: %v", err)
		}
	}()

	// Start Kubernetes service if available
	if k8sService != nil {
		go func() {
			if err := k8sService.Start(serviceCtx); err != nil {
				log.Printf("Kubernetes service error: %v", err)
			}
		}()
	}

	// Initialize API server with service interfaces
	var templateServiceInterface services.TemplateServiceInterface = templateService
	server := api.NewServer(cfg, db, authSvc, jwtManager, userRepo, roleRepo, orgRepo, vdcRepo, catalogRepo, templateRepo, vappRepo, vmRepo, templateServiceInterface, k8sService)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received")

	// Cancel service contexts first
	serviceCancel()

	// Give the server 30 seconds to finish current requests
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
