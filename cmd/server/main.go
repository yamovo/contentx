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

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/vortexcms/go-cms/internal/auth"
	"github.com/vortexcms/go-cms/internal/config"
	"github.com/vortexcms/go-cms/internal/database"
	"github.com/vortexcms/go-cms/internal/handlers"
	"github.com/vortexcms/go-cms/internal/middleware"
)

func main() {
	// Load .env file (ignore error if not found).
	godotenv.Load()

	// Load configuration.
	cfg := config.Load()

	// Set gin mode.
	gin.SetMode(cfg.Server.Mode)

	// Connect to database.
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations.
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed database.
	if err := database.Seed(db); err != nil {
		log.Printf("Warning: seeding failed: %v", err)
	}

	// Create upload directory.
	os.MkdirAll(cfg.Upload.StoragePath, 0755)

	// Initialize JWT manager.
	jwtMgr := auth.NewJWTManager(cfg.JWT)
	blacklist := auth.NewBlacklist()

	// Setup gin.
	r := gin.New()

	// Global middleware.
	r.Use(middleware.RecoverMiddleware())
	r.Use(middleware.RequestID())
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CORSMiddleware(cfg.CORS))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.ActivityLogger(db))

	// Rate limiting (skip for non-API routes in dev).
	r.Use(middleware.RateLimitMiddleware(cfg.Limits.APIRateLimit))

	// Register all routes.
	handlers.RegisterRoutes(r, db, cfg, jwtMgr, blacklist)

	// Serve frontend static files (if built).
	assets := r.Group("/assets")
	assets.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	})
	assets.Static("/", "./web/dist/assets")

	r.StaticFile("/favicon.ico", "./web/dist/favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		// For SPA: serve index.html for non-API routes.
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.File("./web/dist/index.html")
	})

	// Create HTTP server.
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine.
	go func() {
		log.Printf("╔══════════════════════════════════════════╗")
		log.Printf("║          VortexCMS v1.0.0               ║")
		log.Printf("║  Server running on http://%s:%d  ║", cfg.Server.Host, cfg.Server.Port)
		log.Printf("║  Admin: admin / admin123                 ║")
		log.Printf("║  Mode:  %s                           ║", cfg.Server.Mode)
		log.Printf("║  DB:    %s                              ║", cfg.Database.Driver)
		log.Printf("╚══════════════════════════════════════════╝")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
