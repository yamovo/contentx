package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnvStr(t *testing.T) {
	os.Unsetenv("TEST_STR")
	if got := envStr("TEST_STR", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}
	os.Setenv("TEST_STR", "custom")
	defer os.Unsetenv("TEST_STR")
	if got := envStr("TEST_STR", "default"); got != "custom" {
		t.Fatalf("expected custom, got %q", got)
	}
}

func TestLoadTracingConfig(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://tempo:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "0.25")
	t.Setenv("OTEL_SERVICE_NAME", "contentx-test")

	cfg := Load()
	if !cfg.Tracing.Enabled || cfg.Tracing.Endpoint != "http://tempo:4318" || cfg.Tracing.SampleRatio != 0.25 {
		t.Fatalf("unexpected tracing config: %+v", cfg.Tracing)
	}
	if cfg.Tracing.ServiceName != "contentx-test" || !cfg.Tracing.Insecure {
		t.Fatalf("unexpected tracing identity/transport config: %+v", cfg.Tracing)
	}
}

func TestEnvInt(t *testing.T) {
	os.Unsetenv("TEST_INT")
	if got := envInt("TEST_INT", 42); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	os.Setenv("TEST_INT", "100")
	defer os.Unsetenv("TEST_INT")
	if got := envInt("TEST_INT", 42); got != 100 {
		t.Fatalf("expected 100, got %d", got)
	}
	os.Setenv("TEST_INT", "not-a-number")
	if got := envInt("TEST_INT", 42); got != 42 {
		t.Fatalf("expected fallback 42 on parse error, got %d", got)
	}
}

func TestEnvBool(t *testing.T) {
	os.Unsetenv("TEST_BOOL")
	if envBool("TEST_BOOL", true) != true {
		t.Fatal("expected default true")
	}
	os.Setenv("TEST_BOOL", "false")
	defer os.Unsetenv("TEST_BOOL")
	if envBool("TEST_BOOL", true) != false {
		t.Fatal("expected false")
	}
}

func TestEnvFloat(t *testing.T) {
	os.Unsetenv("TEST_FLOAT")
	if envFloat("TEST_FLOAT", 1.5) != 1.5 {
		t.Fatal("expected default 1.5")
	}
	os.Setenv("TEST_FLOAT", "2.5")
	defer os.Unsetenv("TEST_FLOAT")
	if envFloat("TEST_FLOAT", 1.5) != 2.5 {
		t.Fatal("expected 2.5")
	}
}

func TestEnvDuration(t *testing.T) {
	os.Unsetenv("TEST_DUR")
	if envDuration("TEST_DUR", time.Second) != time.Second {
		t.Fatal("expected default 1s")
	}
	os.Setenv("TEST_DUR", "5m")
	defer os.Unsetenv("TEST_DUR")
	if envDuration("TEST_DUR", time.Second) != 5*time.Minute {
		t.Fatal("expected 5m")
	}
}

func TestEnvSlice(t *testing.T) {
	os.Unsetenv("TEST_SLICE")
	def := []string{"a", "b"}
	if got := envSlice("TEST_SLICE", def); len(got) != 2 {
		t.Fatalf("expected default slice, got %v", got)
	}
	os.Setenv("TEST_SLICE", "x,y,z")
	defer os.Unsetenv("TEST_SLICE")
	got := envSlice("TEST_SLICE", def)
	if len(got) != 3 || got[0] != "x" {
		t.Fatalf("expected [x y z], got %v", got)
	}
}

func TestRedisConfig_Addr(t *testing.T) {
	r := RedisConfig{Host: "localhost", Port: 6379}
	if r.Addr() != "localhost:6379" {
		t.Fatalf("expected localhost:6379, got %q", r.Addr())
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	pg := DatabaseConfig{Driver: "postgres", Host: "h", Port: 5432, User: "u", Password: "p", Name: "db", SSLMode: "disable", Timezone: "UTC"}
	if !strings.Contains(pg.DSN(), "host=h") {
		t.Fatalf("postgres DSN missing host: %q", pg.DSN())
	}
	my := DatabaseConfig{Driver: "mysql", Host: "h", Port: 3306, User: "u", Password: "p", Name: "db", Charset: "utf8mb4"}
	if !strings.Contains(my.DSN(), "tcp(h:3306)") {
		t.Fatalf("mysql DSN wrong: %q", my.DSN())
	}
	lite := DatabaseConfig{Driver: "sqlite", Name: "test"}
	if lite.DSN() != "test.db" {
		t.Fatalf("sqlite DSN wrong: %q", lite.DSN())
	}
	if unknown := (DatabaseConfig{Driver: "oracle"}); unknown.DSN() != "" {
		t.Fatalf("unknown driver should return empty DSN, got %q", unknown.DSN())
	}
}

func TestConfig_Validate_WeakSecret(t *testing.T) {
	os.Unsetenv("ADMIN_PASSWORD")
	c := &Config{}
	c.JWT.Secret = "secret" // known weak
	c.Server.Mode = "debug"
	c.Database.Driver = "sqlite"
	if c.Validate() {
		t.Fatal("expected validation to fail for known weak secret")
	}
}

func TestConfig_Validate_ShortSecret(t *testing.T) {
	c := &Config{}
	c.JWT.Secret = "abc123" // < 16 chars
	c.Server.Mode = "debug"
	c.Database.Driver = "sqlite"
	if c.Validate() {
		t.Fatal("expected validation to fail for short secret")
	}
}

func TestConfig_Validate_ValidDebug(t *testing.T) {
	os.Unsetenv("ADMIN_PASSWORD")
	c := &Config{}
	c.JWT.Secret = "a-sufficiently-long-secret-value"
	c.Server.Mode = "debug"
	c.Database.Driver = "sqlite"
	if !c.Validate() {
		t.Fatal("expected validation to pass for strong secret in debug mode")
	}
}

func TestConfig_Validate_ReleaseRequiresAdminPassword(t *testing.T) {
	os.Unsetenv("ADMIN_PASSWORD")
	c := &Config{}
	c.JWT.Secret = "a-sufficiently-long-secret-value"
	c.Server.Mode = "release"
	c.Database.Driver = "sqlite"
	if c.Validate() {
		t.Fatal("expected release validation to fail without ADMIN_PASSWORD")
	}
}
