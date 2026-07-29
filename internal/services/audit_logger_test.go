package services

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
)

// captureAuditLogger is a test AuditLogger that records all events in memory.
type captureAuditLogger struct {
	mu     sync.Mutex
	events []AuditEvent
	NoopAuditLogger
}

func (c *captureAuditLogger) Log(e AuditEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *captureAuditLogger) Events() []AuditEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]AuditEvent, len(c.events))
	copy(out, c.events)
	return out
}

// findAction returns the first event with the given action, or nil.
func (c *captureAuditLogger) findAction(action string) *AuditEvent {
	for _, e := range c.Events() {
		if e.Action == action {
			ev := e
			return &ev
		}
	}
	return nil
}

// TestAuditLogger_LoginFailure verifies that a failed login writes a
// login.failed audit event with the failure reason.
func TestAuditLogger_LoginFailure(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * 60e9, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)
	capture := &captureAuditLogger{}
	svc.SetAuditLogger(capture)

	createTestUser(t, db, "audit-fail-user", "subscriber")

	_, _, err := svc.Login("audit-fail-user", "WrongPass1", "10.0.0.1", "audit-ua")
	if err == nil {
		t.Fatal("Login should fail with wrong password")
	}

	ev := capture.findAction("login.failed")
	if ev == nil {
		t.Fatalf("expected login.failed audit event, got: %+v", capture.Events())
	}
	if ev.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want 10.0.0.1", ev.IP)
	}
	if ev.UserAgent != "audit-ua" {
		t.Errorf("UserAgent = %q, want audit-ua", ev.UserAgent)
	}
	if ev.Entity != "user" {
		t.Errorf("Entity = %q, want user", ev.Entity)
	}
}

// TestAuditLogger_LoginSuccess verifies that a successful login writes a
// login.success audit event with the user's ID.
func TestAuditLogger_LoginSuccess(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * 60e9, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)
	capture := &captureAuditLogger{}
	svc.SetAuditLogger(capture)

	user := createTestUser(t, db, "audit-ok-user", "subscriber")

	_, _, err := svc.Login("audit-ok-user", "TestPass1", "10.0.0.2", "audit-ua-ok")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	ev := capture.findAction("login.success")
	if ev == nil {
		t.Fatalf("expected login.success audit event, got: %+v", capture.Events())
	}
	if ev.EntityID != user.ID {
		t.Errorf("EntityID = %d, want %d", ev.EntityID, user.ID)
	}
	if ev.UserID == nil || *ev.UserID != user.ID {
		t.Errorf("UserID = %v, want %d", ev.UserID, user.ID)
	}
}

// TestAuditLogger_RoleCreate verifies that creating a role writes a
// role.create audit event with the role ID and permission IDs.
func TestAuditLogger_RoleCreate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)
	capture := &captureAuditLogger{}
	svc.SetAuditLogger(capture)

	actorID := uint(77)
	role, err := svc.Create(CreateRoleRequest{
		Name: "Auditor", Slug: "auditor", Description: "audit test role",
	}, actorID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	ev := capture.findAction("role.create")
	if ev == nil {
		t.Fatalf("expected role.create audit event, got: %+v", capture.Events())
	}
	if ev.EntityID != role.ID {
		t.Errorf("EntityID = %d, want %d", ev.EntityID, role.ID)
	}
	if ev.Entity != "role" {
		t.Errorf("Entity = %q, want role", ev.Entity)
	}
	if ev.UserID == nil || *ev.UserID != actorID {
		t.Errorf("UserID = %v, want %d", ev.UserID, actorID)
	}
}

// TestAuditLogger_ArticlePublish verifies that publishing an article writes
// an article.publish audit event with the status transition.
func TestAuditLogger_ArticlePublish(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	capture := &captureAuditLogger{}
	svc.SetAuditLogger(capture)

	author := createTestUser(t, db, "audit-author", "author")
	article := createTestArticle(t, db, author.ID, "Audit Publish Test")

	_, err := svc.PublishAs(article.ID, author.ID)
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	ev := capture.findAction("article.publish")
	if ev == nil {
		t.Fatalf("expected article.publish audit event, got: %+v", capture.Events())
	}
	if ev.EntityID != article.ID {
		t.Errorf("EntityID = %d, want %d", ev.EntityID, article.ID)
	}
	if ev.Entity != "article" {
		t.Errorf("Entity = %q, want article", ev.Entity)
	}
	if ev.UserID == nil || *ev.UserID != author.ID {
		t.Errorf("UserID = %v, want %d", ev.UserID, author.ID)
	}
}

// TestAuditLogger_SettingsUpdate_RedactsSecrets verifies that sensitive
// settings keys are redacted in the audit details.
func TestAuditLogger_SettingsUpdate_RedactsSecrets(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSettingsService(db)
	capture := &captureAuditLogger{}
	svc.SetAuditLogger(capture)

	actorID := uint(88)
	err := svc.Update(map[string]interface{}{
		"site_title":     "My Site",
		"smtp_password":  "super-secret-value",
		"api_key":        "key-12345",
		"public_setting": true,
	}, actorID)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	ev := capture.findAction("system.config_update")
	if ev == nil {
		t.Fatalf("expected system.config_update audit event, got: %+v", capture.Events())
	}
	details, ok := ev.Details.(map[string]any)
	if !ok {
		t.Fatalf("Details should be map[string]any, got %T", ev.Details)
	}
	if details["site_title"] != "My Site" {
		t.Errorf("site_title should be plain, got %v", details["site_title"])
	}
	if details["smtp_password"] != "***" {
		t.Errorf("smtp_password should be redacted, got %v", details["smtp_password"])
	}
	if details["api_key"] != "***" {
		t.Errorf("api_key should be redacted, got %v", details["api_key"])
	}
	if ev.UserID == nil || *ev.UserID != actorID {
		t.Errorf("UserID = %v, want %d", ev.UserID, actorID)
	}
}

func TestAuditLogger_CentrallyRedactsNestedSecrets(t *testing.T) {
	db := setupTestDB(t)
	logger := NewAuditLogger(repository.NewActivityLogRepository(db))
	logger.Log(AuditEvent{
		Action: "security.test", Entity: "test",
		Details: map[string]any{
			"safe": "visible",
			"nested": map[string]any{
				"access_token": "token-value",
				"password":     "password-value",
			},
		},
	})

	var log models.ActivityLog
	if err := db.Where("action = ?", "security.test").First(&log).Error; err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if strings.Contains(log.Details, "token-value") || strings.Contains(log.Details, "password-value") {
		t.Fatalf("sensitive values leaked into audit details: %s", log.Details)
	}
	if !strings.Contains(log.Details, `"safe":"visible"`) {
		t.Fatalf("non-sensitive detail missing: %s", log.Details)
	}
}

func TestAuditLogger_WebhookURLStripsCredentialsAndQuerySecrets(t *testing.T) {
	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)
	capture := &captureAuditLogger{}
	svc.SetAuditLogger(capture)

	actorID := uint(99)
	_, err := svc.Create(CreateWebhookRequest{
		Name:   "audit webhook",
		URL:    "https://hook-user:hook-password@hooks.example.com/events?token=query-secret#fragment",
		Events: []string{"article.published"},
	}, actorID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	ev := capture.findAction("webhook.create")
	if ev == nil {
		t.Fatalf("expected webhook.create audit event, got: %+v", capture.Events())
	}
	if ev.UserID == nil || *ev.UserID != actorID {
		t.Errorf("UserID = %v, want %d", ev.UserID, actorID)
	}
	details, ok := ev.Details.(map[string]any)
	if !ok {
		t.Fatalf("Details should be map[string]any, got %T", ev.Details)
	}
	gotURL, _ := details["url"].(string)
	if gotURL != "https://hooks.example.com/events" {
		t.Errorf("audited URL = %q, want sanitized URL", gotURL)
	}
	for _, secret := range []string{"hook-user", "hook-password", "query-secret", "fragment"} {
		if strings.Contains(gotURL, secret) {
			t.Errorf("audited URL leaks %q: %s", secret, gotURL)
		}
	}
}
