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

	"github.com/ai-atl-dev/HeyAI-backend/configs"
	"github.com/ai-atl-dev/HeyAI-backend/internal/handlers"
	"github.com/ai-atl-dev/HeyAI-backend/internal/middleware"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"github.com/ai-atl-dev/HeyAI-backend/internal/services"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	config, err := configs.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode
	if config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize context
	ctx := context.Background()

	// Initialize services
	log.Println("Initializing services...")

	firestoreService, err := services.NewFirestoreService(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize Firestore: %v", err)
	}
	defer firestoreService.Close()

	bigQueryService, err := services.NewBigQueryService(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize BigQuery: %v", err)
	}
	defer bigQueryService.Close()

	// Create BigQuery tables if they don't exist
	if err := bigQueryService.CreateTables(ctx); err != nil {
		log.Printf("Warning: Failed to create BigQuery tables: %v", err)
	}

	vertexAIService, err := services.NewVertexAIService(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize Vertex AI: %v", err)
	}
	defer vertexAIService.Close()

	twilioService := services.NewTwilioService(config)
	
	sesameAIService := services.NewSesameAIService(config)
	_ = sesameAIService // Will be used for voice synthesis

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(config.JWTSecret)
	rateLimiter := middleware.NewRateLimiter(config.RateLimitRequests, parseDuration(config.RateLimitWindow))

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(firestoreService, authMiddleware, config)
	adminHandler := handlers.NewAdminHandler(firestoreService, bigQueryService)
	
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", config.Port)
	}
	agentHandler := handlers.NewAgentHandler(firestoreService, twilioService, vertexAIService, bigQueryService, baseURL)

	// Initialize Gin router
	router := gin.Default()

	// Apply global middleware
	router.Use(middleware.CORS(config.AllowedOrigins))
	router.Use(rateLimiter.Limit())

	// Health check endpoint
	router.GET("/health", healthCheck(config))
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "HeyAI Backend",
			"status":  "running",
			"version": "1.0.0",
		})
	})

	// Public routes
	public := router.Group("/")
	{
		// Twilio webhook (no auth required)
		public.POST("/webhook/voice", agentHandler.VoiceWebhook)
	}

	// Auth routes
	auth := router.Group("/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.GET("/callback", authHandler.Callback)
		auth.POST("/logout", authHandler.Logout)
	}

	// Protected routes (require authentication)
	api := router.Group("/api")
	api.Use(authMiddleware.RequireAuth())
	{
		// User routes
		api.GET("/me", authHandler.Me)
		api.POST("/refresh", authHandler.RefreshToken)

		// Agent routes
		agents := api.Group("/agents")
		{
			agents.POST("", adminHandler.CreateAgent)
			agents.GET("", adminHandler.ListAgents)
			agents.GET("/:id", adminHandler.GetAgent)
			agents.PUT("/:id", adminHandler.UpdateAgent)
			agents.DELETE("/:id", adminHandler.DeleteAgent)
			
			// Agent analytics
			agents.GET("/:id/stats", adminHandler.GetAgentStats)
			agents.GET("/:id/calls", adminHandler.GetCallHistory)
			agents.GET("/:id/trends", adminHandler.GetCallTrends)
			agents.GET("/:id/top-callers", adminHandler.GetTopCallers)
		}

		// Usage routes
		api.GET("/usage-history", adminHandler.GetUsageHistory)
		api.GET("/live-usage", adminHandler.StreamLiveUsage)

		// Payment routes
		api.POST("/payments", adminHandler.CreatePayment)
		api.GET("/payments", adminHandler.GetPaymentHistory)

		// Dashboard
		api.GET("/dashboard", adminHandler.GetDashboardSummary)
	}

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on port %s", config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// healthCheck returns a health check handler
func healthCheck(config *models.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		health := gin.H{
			"status":      "healthy",
			"timestamp":   time.Now().Format(time.RFC3339),
			"environment": config.Environment,
			"services": gin.H{
				"firestore": "ok",
				"bigquery":  "ok",
				"vertexai":  "ok",
				"twilio":    "ok",
			},
		}

		c.JSON(http.StatusOK, health)
	}
}

// parseDuration parses duration string
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Minute
	}
	return d
}
