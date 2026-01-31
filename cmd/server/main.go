package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/prepmyapp/notification/internal/config"
	"github.com/prepmyapp/notification/internal/database"
	"github.com/prepmyapp/notification/internal/handler"
	"github.com/prepmyapp/notification/internal/handler/middleware"
	"github.com/prepmyapp/notification/internal/infrastructure/firebase"
	"github.com/prepmyapp/notification/internal/infrastructure/sendgrid"
	"github.com/prepmyapp/notification/internal/infrastructure/websocket"
	"github.com/prepmyapp/notification/internal/repository/postgres"
	"github.com/prepmyapp/notification/internal/service"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database (optional - service works without DB for basic health checks)
	var db *database.DB
	var notificationRepo *postgres.NotificationRepository
	var deviceTokenRepo *postgres.DeviceTokenRepository
	var preferencesRepo *postgres.PreferencesRepository

	if cfg.Database.URL != "" {
		dbConfig := database.DefaultConfig(cfg.Database.URL)
		db, err = database.New(ctx, dbConfig)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
		} else {
			log.Println("Connected to database")
			notificationRepo = postgres.NewNotificationRepository(db.Pool)
			deviceTokenRepo = postgres.NewDeviceTokenRepository(db.Pool)
			preferencesRepo = postgres.NewPreferencesRepository(db.Pool)
		}
	}

	// Initialize SendGrid client (optional)
	var emailSender service.EmailSender
	if cfg.SendGrid.APIKey != "" {
		emailSender = sendgrid.NewClient(sendgrid.Config{
			APIKey:    cfg.SendGrid.APIKey,
			FromEmail: cfg.SendGrid.FromEmail,
			FromName:  cfg.SendGrid.FromName,
		})
		log.Println("SendGrid client initialized")
	}

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	log.Println("WebSocket hub started")

	// Initialize Firebase client (optional)
	var pushSender service.PushSender
	if (cfg.Firebase.CredentialsJSON != "" || cfg.Firebase.CredentialsPath != "") && deviceTokenRepo != nil {
		firebaseClient, err := firebase.NewClient(ctx, firebase.Config{
			CredentialsPath: cfg.Firebase.CredentialsPath,
			CredentialsJSON: cfg.Firebase.CredentialsJSON,
		}, deviceTokenRepo)
		if err != nil {
			log.Printf("Warning: Failed to initialize Firebase: %v", err)
		} else {
			pushSender = firebaseClient
			log.Println("Firebase client initialized")
		}
	}

	// Initialize notification service
	var notificationService *service.NotificationService
	if notificationRepo != nil {
		notificationService = service.NewNotificationService(
			notificationRepo,
			deviceTokenRepo,
			preferencesRepo,
			emailSender,
			pushSender,
			wsHub,
		)
		log.Println("Notification service initialized")
	}

	// Create Gin router
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5001", "https://prepmy.com", "https://prepmyapp.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Setup routes
	setupRoutes(router, cfg, notificationService, deviceTokenRepo, preferencesRepo, wsHub)

	// Create HTTP server with timeouts
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting notification service on port %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	gracefulShutdown(srv, db)
}

// setupRoutes configures all API routes.
func setupRoutes(router *gin.Engine, cfg *config.Config, notificationService *service.NotificationService, deviceTokenRepo *postgres.DeviceTokenRepository, preferencesRepo *postgres.PreferencesRepository, wsHub *websocket.Hub) {
	// Root health check for Replit/load balancer
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Health check endpoints (no auth required)
	healthHandler := handler.NewHealthHandler()
	healthHandler.RegisterRoutes(&router.RouterGroup)

	// WebSocket endpoint (JWT auth via query param)
	if cfg.Auth.JWTSecret != "" {
		wsHandler := handler.NewWebSocketHandler(wsHub, cfg.Auth.JWTSecret)
		wsHandler.RegisterRoutes(router)
	}

	// API v1 routes (JWT auth required)
	v1 := router.Group("/api/v1")
	if cfg.Auth.JWTSecret != "" {
		v1.Use(middleware.JWTAuth(cfg.Auth.JWTSecret))
	}

	// Register notification endpoints if service is available
	if notificationService != nil {
		notificationHandler := handler.NewNotificationHandler(notificationService)
		notificationHandler.RegisterRoutes(v1)
	}

	// Register device token endpoints if repository is available
	if deviceTokenRepo != nil {
		deviceTokenHandler := handler.NewDeviceTokenHandler(deviceTokenRepo)
		deviceTokenHandler.RegisterRoutes(v1)
	}

	// Register preferences endpoints if repository is available
	if preferencesRepo != nil {
		preferencesHandler := handler.NewPreferencesHandler(preferencesRepo)
		preferencesHandler.RegisterRoutes(v1)
	}

	// Service info endpoint
	v1.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "notification",
			"version": "1.0.0",
			"status":  "running",
		})
	})

	// Internal API routes (API key auth required)
	internal := router.Group("/internal/v1")
	if len(cfg.Auth.APIKeys) > 0 {
		internal.Use(middleware.APIKeyAuth(cfg.Auth.APIKeys))
	}

	// Register internal endpoints if service is available
	if notificationService != nil {
		internalHandler := handler.NewInternalHandler(notificationService)
		internalHandler.RegisterRoutes(internal)
	}

	// Internal info endpoint
	internal.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Internal API",
			"status":  "ready",
		})
	})
}

// gracefulShutdown handles clean server shutdown on interrupt signals.
func gracefulShutdown(srv *http.Server, db *database.DB) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if db != nil {
		db.Close()
		log.Println("Database connection closed")
	}

	log.Println("Server stopped")
}
