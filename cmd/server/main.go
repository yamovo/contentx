package main

import (
	"context"
	"fmt"
	"log/slog"
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
	"github.com/vortexcms/go-cms/internal/logger"
	"github.com/vortexcms/go-cms/internal/middleware"

	_ "github.com/vortexcms/go-cms/docs/api" // swagger docs
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// Load .env file (ignore error if not found).
	godotenv.Load()

	// Load configuration.
	cfg := config.Load()

	// Initialize structured logger.
	logger.Setup(cfg.Log)

	// Set gin mode.
	gin.SetMode(cfg.Server.Mode)

	// Connect to database.
	db, err := database.Connect(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	// Run migrations.
	if err := database.AutoMigrate(db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Seed database.
	if err := database.Seed(db); err != nil {
		slog.Warn("seeding failed", "error", err)
	}

	// Create upload directory.
	os.MkdirAll(cfg.Upload.StoragePath, 0755)

	// Initialize JWT manager.
	jwtMgr := auth.NewJWTManager(cfg.JWT)
	blacklist := auth.NewBlacklist()
	guard := auth.NewLoginGuard()

	// Setup gin.
	r := gin.New()

	// Global middleware.
	r.Use(middleware.RecoverMiddleware())
	r.Use(middleware.RequestID())
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CORSMiddleware(cfg.CORS))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.ContentTypeJSON())
	r.Use(middleware.ActivityLogger(db))

	// Rate limiting (skip for non-API routes in dev).
	r.Use(middleware.RateLimitMiddleware(cfg.Limits.APIRateLimit))

	// Register all routes.
	rateLimiter := handlers.RegisterRoutes(r, db, cfg, jwtMgr, blacklist, guard)

	// Swagger API docs (only in non-release mode).
	if cfg.Server.Mode != "release" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

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
		slog.Info("VortexCMS starting",
			"host", cfg.Server.Host,
			"port", cfg.Server.Port,
			"mode", cfg.Server.Mode,
			"db", cfg.Database.Driver,
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rateLimiter.Shutdown()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}
