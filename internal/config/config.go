package config

import (
	"fmt"
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Upload   UploadConfig
	Mail     MailConfig
	Search   SearchConfig
	Cache    CacheConfig
	Queue    QueueConfig
	Plugin   PluginConfig
	Theme    ThemeConfig
	Backup   BackupConfig
	Analytics AnalyticsConfig
	Limits   LimitsConfig
	CORS     CORSConfig
	Log      LogConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Mode         string // debug, release, test
	BaseURL      string
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Driver          string // postgres, mysql, sqlite
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Charset         string // mysql only
	Timezone        string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	Prefix   string
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret           string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer           string
}

// UploadConfig holds file upload settings.
type UploadConfig struct {
	MaxSize      int64
	AllowedTypes []string
	StoragePath  string
	URLPrefix    string
	ThumbnailMax int
	ImageQuality int
}

// MailConfig holds SMTP mail settings.
type MailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	FromName string
	UseTLS   bool
}

// SearchConfig holds search engine settings.
type SearchConfig struct {
	Engine   string // builtin, elasticsearch, meilisearch
	ESURL    string
	ESIndex  string
	MeiliURL string
	MeiliKey string
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	Driver      string // memory, redis
	DefaultTTL  time.Duration
	MaxEntries  int
}

// QueueConfig holds job queue settings.
type QueueConfig struct {
	Driver     string // memory, redis
	MaxWorkers int
	MaxRetries int
	RetryDelay time.Duration
}

// PluginConfig holds plugin system settings.
type PluginConfig struct {
	Dir     string
	Enabled []string
}

// ThemeConfig holds theme engine settings.
type ThemeConfig struct {
	Dir     string
	Default string
}

// BackupConfig holds backup settings.
type BackupConfig struct {
	Dir          string
	MaxBackups   int
	CompressType string // gzip, zstd
	Schedule     string // cron expression for auto-backup
}

// AnalyticsConfig holds analytics settings.
type AnalyticsConfig struct {
	Enabled    bool
	Retention  int // days
	SampleRate float64
}

// LimitsConfig holds rate limiting and resource limits.
type LimitsConfig struct {
	APIRateLimit   int // requests per minute
	UploadRateLimit int
	MaxPageSize    int
	DefaultPageSize int
	MaxCommentDepth int
	MaxMenuDepth    int
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	AllowCredentials bool
	MaxAge         int
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level      string // debug, info, warn, error
	Format     string // text, json
	Output     string // stdout, file
	FilePath   string
	MaxSize    int // MB
	MaxBackups int
	MaxAge     int // days
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         envStr("SERVER_HOST", "0.0.0.0"),
			Port:         envInt("SERVER_PORT", 8080),
			ReadTimeout:  envDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: envDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  envDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
			Mode:         envStr("SERVER_MODE", "debug"),
			BaseURL:      envStr("SERVER_BASE_URL", "http://localhost:8080"),
		},
		Database: DatabaseConfig{
			Driver:          envStr("DB_DRIVER", "sqlite"),
			Host:            envStr("DB_HOST", "localhost"),
			Port:            envInt("DB_PORT", 5432),
			User:            envStr("DB_USER", "vortexcms"),
			Password:        envStr("DB_PASSWORD", ""),
			Name:            envStr("DB_NAME", "vortexcms"),
			SSLMode:         envStr("DB_SSL_MODE", "disable"),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			Charset:         envStr("DB_CHARSET", "utf8mb4"),
			Timezone:        envStr("DB_TIMEZONE", "Asia/Shanghai"),
		},
		Redis: RedisConfig{
			Host:     envStr("REDIS_HOST", "localhost"),
			Port:     envInt("REDIS_PORT", 6379),
			Password: envStr("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
			Prefix:   envStr("REDIS_PREFIX", "vortex:"),
		},
		JWT: JWTConfig{
			Secret:           loadJWTSecret(),
			AccessTokenTTL:  envDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			Issuer:           envStr("JWT_ISSUER", "vortexcms"),
		},
		Upload: UploadConfig{
			MaxSize:      int64(envInt("UPLOAD_MAX_SIZE", 20<<20)), // 20MB
			AllowedTypes: envSlice("UPLOAD_ALLOWED_TYPES", []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml", "application/pdf", "video/mp4"}),
			StoragePath:  envStr("UPLOAD_STORAGE_PATH", "./uploads"),
			URLPrefix:    envStr("UPLOAD_URL_PREFIX", "/uploads"),
			ThumbnailMax: envInt("UPLOAD_THUMBNAIL_MAX", 400),
			ImageQuality: envInt("UPLOAD_IMAGE_QUALITY", 85),
		},
		Mail: MailConfig{
			Host:     envStr("SMTP_HOST", "localhost"),
			Port:     envInt("SMTP_PORT", 587),
			User:     envStr("SMTP_USER", ""),
			Password: envStr("SMTP_PASSWORD", ""),
			From:     envStr("SMTP_FROM", "noreply@vortexcms.local"),
			FromName: envStr("SMTP_FROM_NAME", "VortexCMS"),
			UseTLS:   envBool("SMTP_USE_TLS", true),
		},
		Search: SearchConfig{
			Engine:   envStr("SEARCH_ENGINE", "builtin"),
			ESURL:    envStr("ELASTICSEARCH_URL", "http://localhost:9200"),
			ESIndex:  envStr("ELASTICSEARCH_INDEX", "vortexcms"),
			MeiliURL: envStr("MEILISEARCH_URL", "http://localhost:7700"),
			MeiliKey: envStr("MEILISEARCH_KEY", ""),
		},
		Cache: CacheConfig{
			Driver:     envStr("CACHE_DRIVER", "memory"),
			DefaultTTL: envDuration("CACHE_DEFAULT_TTL", 10*time.Minute),
			MaxEntries: envInt("CACHE_MAX_ENTRIES", 10000),
		},
		Queue: QueueConfig{
			Driver:     envStr("QUEUE_DRIVER", "memory"),
			MaxWorkers: envInt("QUEUE_MAX_WORKERS", 4),
			MaxRetries: envInt("QUEUE_MAX_RETRIES", 3),
			RetryDelay: envDuration("QUEUE_RETRY_DELAY", 5*time.Second),
		},
		Plugin: PluginConfig{
			Dir:     envStr("PLUGIN_DIR", "./plugins"),
			Enabled: envSlice("PLUGIN_ENABLED", []string{}),
		},
		Theme: ThemeConfig{
			Dir:     envStr("THEME_DIR", "./themes"),
			Default: envStr("THEME_DEFAULT", "default"),
		},
		Backup: BackupConfig{
			Dir:          envStr("BACKUP_DIR", "./backups"),
			MaxBackups:   envInt("BACKUP_MAX", 10),
			CompressType: envStr("BACKUP_COMPRESS", "gzip"),
			Schedule:     envStr("BACKUP_SCHEDULE", "0 3 * * *"), // 3am daily
		},
		Analytics: AnalyticsConfig{
			Enabled:    envBool("ANALYTICS_ENABLED", true),
			Retention:  envInt("ANALYTICS_RETENTION", 90),
			SampleRate: envFloat("ANALYTICS_SAMPLE_RATE", 1.0),
		},
		Limits: LimitsConfig{
			APIRateLimit:    envInt("LIMITS_API_RATE", 300),
			UploadRateLimit: envInt("LIMITS_UPLOAD_RATE", 10),
			MaxPageSize:     envInt("LIMITS_MAX_PAGE_SIZE", 100),
			DefaultPageSize: envInt("LIMITS_DEFAULT_PAGE_SIZE", 20),
			MaxCommentDepth: envInt("LIMITS_MAX_COMMENT_DEPTH", 5),
			MaxMenuDepth:    envInt("LIMITS_MAX_MENU_DEPTH", 4),
		},
		CORS: CORSConfig{
			AllowedOrigins:   envSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:8080"}),
			AllowedMethods:   envSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
			AllowedHeaders:   envSlice("CORS_ALLOWED_HEADERS", []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"}),
			AllowCredentials: envBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge:           envInt("CORS_MAX_AGE", 86400),
		},
		Log: LogConfig{
			Level:      envStr("LOG_LEVEL", "info"),
			Format:     envStr("LOG_FORMAT", "text"),
			Output:     envStr("LOG_OUTPUT", "stdout"),
			FilePath:   envStr("LOG_FILE_PATH", "./logs/app.log"),
			MaxSize:    envInt("LOG_MAX_SIZE", 100),
			MaxBackups: envInt("LOG_MAX_BACKUPS", 3),
			MaxAge:     envInt("LOG_MAX_AGE", 28),
		},
	}
}

// DSN returns the database connection string.
func (d DatabaseConfig) DSN() string {
	switch d.Driver {
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode, d.Timezone)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			d.User, d.Password, d.Host, d.Port, d.Name, d.Charset)
	case "sqlite":
		return d.Name + ".db"
	default:
		return ""
	}
}

// RedisAddr returns the Redis address.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// Helper functions for environment variables.

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envSlice(key string, def []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return def
}

// loadJWTSecret loads JWT secret from env or generates one in dev mode.
func loadJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret != "" {
		return secret
	}
	// In release mode, require explicit secret.
	mode := os.Getenv("SERVER_MODE")
	if mode == "release" {
		log.Fatal("[SECURITY] FATAL: JWT_SECRET must be set in production mode. " +
			"Generate one with: openssl rand -base64 32")
	}
	// Development mode: generate random secret and warn.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatal("[SECURITY] Failed to generate JWT secret")
	}
	secret = base64.StdEncoding.EncodeToString(b)
	log.Printf("[SECURITY] WARNING: Using auto-generated JWT secret. Set JWT_SECRET env var for persistence.")
	return secret
}