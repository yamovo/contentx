package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/backup"
	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/database/migrations"
	"github.com/yamovo/contentx/internal/handlers"
	"github.com/yamovo/contentx/internal/logger"
	"github.com/yamovo/contentx/internal/mcp"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/observability"
	"github.com/yamovo/contentx/internal/services"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/yamovo/contentx/docs/api" // swagger docs
)

// @title           ContentX API
// @version         1.3.0
// @description     ContentX API-first Headless CMS REST API
// @host            localhost:8080
// @BasePath        /api/v1
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT token. Format: Bearer {token}

// version 在构建时通过 -ldflags="-X main.version=..." 注入。
// 默认值 "dev" 用于本地开发；CI release job 会注入 git tag 版本号。
var version = "dev"

func main() {
	// CLI flags for database operations.
	migrateFlag := flag.Bool("migrate", false, "run pending database migrations and exit")
	migrateDownFlag := flag.Int("migrate-down", 0, "roll back the last N database migrations and exit")
	migrateStatusFlag := flag.Bool("migrate-status", false, "show database migration status and exit")
	seedFlag := flag.Bool("seed", false, "seed the database and exit")
	restoreFlag := flag.String("restore", "", "restore database from a backup file (bypasses HTTP/auth; for disaster recovery)")
	mcpFlag := flag.Bool("mcp", false, "run as an MCP (Model Context Protocol) server over stdio and exit")
	flag.Parse()

	// Load .env file (ignore error if not found).
	_ = godotenv.Load()

	// Load configuration.
	cfg := config.Load()

	// Startup security audit.
	if !cfg.Validate() {
		slog.Error("security audit failed — fix issues above before running in production")
		os.Exit(1)
	}

	// Initialize structured logger.
	logger.Setup(cfg.Log)

	traceShutdown, err := observability.InitTracing(context.Background(), observability.TraceOptions{
		Enabled:     cfg.Tracing.Enabled,
		Endpoint:    cfg.Tracing.Endpoint,
		Insecure:    cfg.Tracing.Insecure,
		SampleRatio: cfg.Tracing.SampleRatio,
		ServiceName: cfg.Tracing.ServiceName,
		Version:     version,
		Environment: cfg.Server.Mode,
	})
	if err != nil {
		slog.Error("failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(ctx); err != nil {
			slog.Warn("OpenTelemetry shutdown failed", "error", err)
		}
	}()

	// Set gin mode.
	gin.SetMode(cfg.Server.Mode)

	// Connect to database.
	db, err := database.Connect(cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	if err := observability.InstrumentGORM(db, cfg.Database.Driver); err != nil {
		slog.Error("failed to instrument database tracing", "error", err)
		os.Exit(1)
	}

	// Handle migration-only modes.
	switch {
	case *migrateStatusFlag:
		statuses, err := database.MigrationStatuses(db, migrations.All())
		if err != nil {
			slog.Error("failed to get migration status", "error", err)
			os.Exit(1)
		}
		fmt.Println("Migration status:")
		for _, s := range statuses {
			mark := "  "
			if s.Applied {
				mark = "✓ "
			}
			fmt.Printf("  %sv%d  %s\n", mark, s.Version, s.Description)
		}
		return

	case *migrateDownFlag > 0:
		if err := database.RollbackMigration(db, migrations.All(), *migrateDownFlag); err != nil {
			slog.Error("migration rollback failed", "error", err)
			os.Exit(1)
		}
		return

	case *migrateFlag:
		if err := database.RunMigrations(db, migrations.All()); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		return

	case *seedFlag:
		if err := database.Seed(db); err != nil {
			slog.Error("seeding failed", "error", err)
			os.Exit(1)
		}
		slog.Info("seeding completed")
		return

	case *restoreFlag != "":
		// Disaster recovery: restore database from a backup file via CLI,
		// bypassing HTTP/auth (Round 6 / F3). This eliminates the auth-DB
		// circular dependency that makes the HTTP restore endpoint unusable
		// when the database is completely lost.
		mgr := backup.NewManager(cfg.Backup, cfg.Database, "", db)
		path := *restoreFlag
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Try relative to the backup directory.
			path = filepath.Join(mgr.Dir(), *restoreFlag)
		}
		slog.Info("starting CLI restore", "file", path, "driver", cfg.Database.Driver)
		if err := mgr.Restore(path); err != nil {
			slog.Error("restore failed", "error", err)
			os.Exit(1)
		}
		// Verify row counts (pg/mysql only; SQLite requires restart).
		if cfg.Database.Driver != "sqlite" {
			if counts, err := mgr.RowCounts(); err == nil {
				slog.Info("restore completed", "row_counts", counts)
			} else {
				slog.Warn("restore completed but row count check failed", "error", err)
			}
		} else {
			slog.Info("sqlite restore completed; restart the application to verify and rebuild search index")
		}
		return

	case *mcpFlag:
		// Run as an MCP server over stdio (AI-agent content backend). Reuses the
		// read-only services in-process; assumes the DB is already migrated/seeded
		// (same convention as --restore).
		//
		// MCP speaks JSON-RPC over stdout, so logs are redirected to stderr here to
		// avoid corrupting the protocol stream.
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
		mcpArticleSvc := services.NewArticleService(db, cfg.Server.BaseURL)
		mcpArticleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
		if _, err := mcpArticleSvc.ReindexAll(context.Background()); err != nil {
			slog.Warn("mcp: search index warmup failed", "error", err)
		}
		mcpServer := mcp.NewServer(mcp.Deps{
			Article:       mcpArticleSvc,
			ContentType:   services.NewContentTypeService(db),
			BaseURL:       cfg.Server.BaseURL,
			IncludeDrafts: cfg.MCP.IncludeDrafts,
		}, version)
		if err := mcpServer.Run(context.Background(), &mcpsdk.StdioTransport{}); err != nil {
			slog.Error("mcp server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// Normal startup: run migrations (idempotent) then seed.
	if err := database.RunMigrations(db, migrations.All()); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Seed database.
	if err := database.Seed(db); err != nil {
		slog.Warn("seeding failed", "error", err)
	}

	// Create upload directory.
	_ = os.MkdirAll(cfg.Upload.StoragePath, 0755)

	// Initialize cache (memory or redis based on config). Fall back to the
	// in-memory cache if the configured backend cannot be reached at startup.
	cacheDriver, err := cache.New(cache.Config{
		Driver: cfg.Cache.Driver,
		Redis: cache.RedisConfig{
			Addr:     cfg.Redis.Addr(),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			Prefix:   cfg.Redis.Prefix,
		},
		Memory: cache.MemoryConfig{
			MaxEntries: cfg.Cache.MaxEntries,
			DefaultTTL: cfg.Cache.DefaultTTL,
		},
	})
	if err != nil {
		slog.Warn("cache backend unavailable, falling back to memory", "driver", cfg.Cache.Driver, "error", err)
		cacheDriver = cache.NewMemoryDriver(cfg.Cache.MaxEntries)
	}
	slog.Info("cache initialized", "driver", cfg.Cache.Driver)
	baseCacheDriver := cacheDriver

	// Initialize JWT manager and token store.
	// 优先使用 Redis-backed 黑名单（多实例共享、重启不丢失）；
	// 若 Redis 不可用或配置为内存缓存，回退到内存版 Blacklist。
	jwtMgr := auth.NewJWTManager(cfg.JWT)
	tokenStore := initTokenStore(cfg, cacheDriver)
	guard := initLoginGuard(cfg, cacheDriver)

	// Setup gin.
	r := gin.New()

	// 配置 validator：让 ValidationErrors 中的字段名返回 JSON tag 名
	// （而非默认的结构体字段名 PascalCase），便于脱敏后的错误消息直接对前端友好。
	handlers.RegisterJSONTagNameFunc()

	// Prometheus 指标采集器（全局共享，/metrics 端点输出）。
	// 业务指标快照由 SystemService.SnapshotMetrics 提供，在 RegisterRoutes 中注入。
	promCollector := middleware.NewPrometheusCollector()
	observability.SetMetricsRecorder(promCollector)
	cacheDriver = cache.NewMeteredDriver(cacheDriver)

	// Global middleware.
	r.Use(middleware.RecoverMiddleware())
	r.Use(middleware.RequestID())
	r.Use(middleware.TracingMiddleware(cfg.Tracing.ServiceName))
	r.Use(middleware.PrometheusMiddleware(promCollector))
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CORSMiddleware(cfg.CORS))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.ContentTypeJSON())
	r.Use(middleware.ActivityLogger(db))

	// Rate limiting: apply only to /api/ routes so static assets, swagger,
	// /metrics, and SPA fallback routes are not throttled. The RateLimiter is
	// stoppable so its cleanup goroutine doesn't leak on shutdown.
	apiRateLimiter := middleware.NewRateLimiter(cfg.Limits.APIRateLimit)
	apiRateLimitHandler := apiRateLimiter.Handler()
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			apiRateLimitHandler(c)
			return
		}
		c.Next()
	})

	// Create the backup manager (shared between HTTP handler and scheduler so
	// that the TryLock serializes concurrent backup/restore requests).
	backupMgr := backup.NewManager(cfg.Backup, cfg.Database, cfg.Upload.StoragePath, db)

	// Register all routes.
	rateLimiter := handlers.RegisterRoutes(r, db, cfg, jwtMgr, tokenStore, guard, cacheDriver, backupMgr, promCollector)

	// Prometheus /metrics 端点（无需认证）。
	if cfg.Metrics.Enabled {
		path := cfg.Metrics.Path
		if path == "" {
			path = "/metrics"
		}
		r.GET(path, gin.WrapH(promCollector.MetricsHandler()))
		slog.Info("prometheus metrics endpoint enabled", "path", path)
	}

	// Start the scheduled-publish worker. It periodically scans for articles
	// whose ScheduledAt has passed and flips them to published. The scheduler
	// uses its own ArticleService instance (sharing the same db + webhook
	// wiring) so it is decoupled from the HTTP request path.
	schedulerArticleSvc := services.NewArticleService(db, cfg.Server.BaseURL)
	schedulerArticleSvc.SetWebhookDispatcher(services.NewWebhookService(db))
	publishScheduler := services.NewPublishScheduler(schedulerArticleSvc, time.Minute, slog.Default())

	// 多实例部署时注入分布式锁，防止多实例重复执行定时发布。
	// Redis 可用时用 RedisLock；不可用时降级为 MemoryLock（仅保护同进程）。
	if redisDrv, ok := baseCacheDriver.(*cache.RedisDriver); ok {
		publishScheduler.SetDistributedLock(cache.NewRedisLock(redisDrv.Client(), cfg.Redis.Prefix))
		slog.Info("publish scheduler: using redis distributed lock")
	} else {
		publishScheduler.SetDistributedLock(cache.NewMemoryLock())
		slog.Info("publish scheduler: using in-memory lock (single instance)")
	}

	publishScheduler.Start()
	defer publishScheduler.Stop()

	// Start the webhook delivery worker. Dispatch (wired into content/media/
	// comment/user services in RegisterRoutes) only enqueues persistent rows;
	// this worker drains them with bounded concurrency and backoff-scheduled
	// retries, so deliveries survive restarts and bursts can't fan out
	// unbounded goroutines. Queue behaviour reuses cfg.Queue (QUEUE_*).
	webhookWorker := services.NewWebhookWorker(db, cfg.Queue)
	webhookWorker.Start()
	defer webhookWorker.Stop()
	slog.Info("webhook worker started",
		"concurrency", cfg.Queue.MaxWorkers,
		"max_retries", cfg.Queue.MaxRetries,
		"retry_delay", cfg.Queue.RetryDelay,
	)

	// Start the scheduled-backup worker. It runs BackupAll on the cron
	// schedule from cfg.Backup.Schedule (default "0 3 * * *" = 3am daily).
	// Retention is handled by the Manager's cleanup (MaxBackups). Like the
	// publish scheduler, it uses a distributed lock in multi-instance
	// deployments to prevent duplicate backups.
	backupScheduler := backup.NewBackupScheduler(backupMgr, cfg.Backup.Schedule, slog.Default())
	if redisDrv, ok := baseCacheDriver.(*cache.RedisDriver); ok {
		backupScheduler.SetDistributedLock(cache.NewRedisLock(redisDrv.Client(), cfg.Redis.Prefix))
		slog.Info("backup scheduler: using redis distributed lock")
	} else {
		backupScheduler.SetDistributedLock(cache.NewMemoryLock())
		slog.Info("backup scheduler: using in-memory lock (single instance)")
	}
	if err := backupScheduler.Start(); err != nil {
		slog.Error("failed to start backup scheduler", "error", err)
	} else {
		defer backupScheduler.Stop()
	}

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
		// index.html 绝对不能被浏览器缓存，否则用户会拿到引用旧 hash JS 的旧 HTML，
		// 导致前端更新后浏览器仍加载 web/dist/assets 里的旧 JS（hash 文件名防缓存
		// 失效）。no-store 强制每次都拉新 HTML；hash 化的 /assets/* 才走 immutable。
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
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
		slog.Info("ContentX starting",
			"version", version,
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

	// Stop background cleanup goroutines to prevent leaks.
	rateLimiter.Shutdown()
	apiRateLimiter.Stop()
	guard.Stop()
	if closer, ok := baseCacheDriver.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}

// initTokenStore 根据配置创建 JWT token 黑名单存储：
//   - cache driver 为 Redis 时，返回 *auth.ResilientTokenStore（Redis + 内存双写）：
//     Redis 提供多实例共享与重启不丢失；内存层在 Redis 故障窗口内提供兜底，
//     避免 fail-open（SEC-2：故障期间已吊销 token 仍可用）
//   - 否则回退到内存版 *auth.Blacklist
//
// Redis 连接复用 cache.RedisDriver 的 client，避免重复建连。
func initTokenStore(cfg *config.Config, cacheDriver cache.Driver) auth.TokenStore {
	redisDrv, ok := cacheDriver.(*cache.RedisDriver)
	if !ok {
		slog.Info("token store: using in-memory blacklist (cache driver is not redis)")
		return auth.NewBlacklist()
	}

	store := auth.NewRedisTokenStore(redisDrv.Client(), cfg.Redis.Prefix+":jwt:blacklist:")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		slog.Warn("token store: redis unreachable, falling back to in-memory blacklist", "error", err)
		return auth.NewBlacklist()
	}
	slog.Info("token store: using redis-backed blacklist with in-memory fallback")
	return auth.NewResilientTokenStore(store, nil)
}

// initLoginGuard 根据配置创建登录失败限流器（SEC-6）：
//   - cache driver 为 Redis 时，返回 *auth.RedisLoginGuard（多实例共享锁定，
//     防分布式撞库；Redis 故障时内部回退内存版）
//   - 否则回退到内存版 *auth.LoginGuard（单实例）
func initLoginGuard(cfg *config.Config, cacheDriver cache.Driver) auth.LoginLimiter {
	redisDrv, ok := cacheDriver.(*cache.RedisDriver)
	if !ok {
		slog.Info("login guard: using in-memory guard (cache driver is not redis)")
		return auth.NewLoginGuard()
	}

	guard := auth.NewRedisLoginGuard(redisDrv.Client(), cfg.Redis.Prefix+":login:")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := guard.Ping(ctx); err != nil {
		slog.Warn("login guard: redis unreachable, falling back to in-memory guard", "error", err)
		guard.Stop() // 释放内嵌 fallback 的清理 goroutine
		return auth.NewLoginGuard()
	}
	slog.Info("login guard: using redis-backed guard with in-memory fallback")
	return guard
}
