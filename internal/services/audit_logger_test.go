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
// TestAuditLogger_ArticlePublish verifies the transactional transition audit:
// publishing commits exactly one article.publish row together with the status
// change, with full envelope provenance.
func TestAuditLogger_ArticlePublish(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")

	author := createTestUser(t, db, "audit-author", "author")
	article := createTestArticle(t, db, author.ID, "Audit Publish Test")

	if _, err := svc.PublishAs(article.ID, models.DefaultTenantID, author.ID); err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	var logs []models.ActivityLog
	db.Where("action = ? AND entity_id = ?", "article.publish", article.ID).Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("expected exactly 1 article.publish audit row, got %d", len(logs))
	}
	row := logs[0]
	if row.UserID == nil || *row.UserID != author.ID {
		t.Errorf("UserID = %v, want %d", row.UserID, author.ID)
	}
	if row.Source != SourceREST || row.ActorType != ActorUser || row.Outcome != OutcomeSuccess {
		t.Errorf("provenance = source=%q actor=%q outcome=%q", row.Source, row.ActorType, row.Outcome)
	}
	if row.EventID == "" {
		t.Error("EventID must be set on transition audit rows")
	}
	var count int64
	db.Model(&models.ActivityLog{}).Where("entity_id = ? AND action LIKE ?", article.ID, "article.%").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 audit row per operation (no duplicates), got %d", count)
	}
}

// TestAuditLogger_TransitionFailsClosed verifies that a status change cannot
// commit when its audit record is rejected.
func TestAuditLogger_TransitionFailsClosed(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewArticleRepository(db)
	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")

	author := createTestUser(t, db, "failclosed-author", "author")
	// A draft makes the rollback observable: the attempted transition is
	// draft -> published, so a successful (but unaudited) publish would leave
	// a published row behind.
	draft := models.Article{
		Title: "Fail Closed Draft", Slug: "fail-closed-draft",
		Content: "<p>x</p>", AuthorID: author.ID,
		Status: models.StatusDraft, TenantID: models.DefaultTenantID,
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Force the audit insert to fail: drop the audit table. The transition
	// must roll back and surface the failure.
	if err := db.Migrator().DropTable(&models.ActivityLog{}); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	if _, err := svc.PublishAs(draft.ID, models.DefaultTenantID, author.ID); err == nil {
		t.Fatal("publish must fail when its audit record cannot be written")
	}

	// Restore the table and verify the article was NOT transitioned.
	if err := db.AutoMigrate(&models.ActivityLog{}); err != nil {
		t.Fatalf("recreate audit table: %v", err)
	}
	var refreshed models.Article
	if err := db.First(&refreshed, draft.ID).Error; err != nil {
		t.Fatalf("reload article: %v", err)
	}
	if refreshed.Status != models.StatusDraft {
		t.Fatalf("article status must remain draft (transaction rolled back), got %q", refreshed.Status)
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
	}, models.DefaultTenantID, actorID)
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

	tenantID := uint(1)
	actorID := uint(99)
	_, err := svc.Create(CreateWebhookRequest{
		Name:   "audit webhook",
		URL:    "https://hook-user:hook-password@hooks.example.com/events?token=query-secret#fragment",
		Events: []string{"article.published"},
	}, tenantID, actorID)
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

// ─── Versioned audit envelope (RESEARCH-001 §4) ─────────────────────────────

func TestAuditLogger_EnvelopeFieldsPersisted(t *testing.T) {
	db := setupTestDB(t)
	logger := NewAuditLogger(repository.NewActivityLogRepository(db))

	tenantID := uint(7)
	userID := uint(3)
	logger.Log(AuditEvent{
		UserID:    &userID,
		TenantID:  &tenantID,
		Action:    "article.publish",
		Entity:    "article",
		EntityID:  11,
		Source:    SourceREST,
		ActorType: ActorUser,
		Outcome:   OutcomeSuccess,
		RequestID: "req-123",
		TraceID:   "trace-456",
		SpanID:    "span-789",
	})

	var logs []models.ActivityLog
	db.Where("action = ?", "article.publish").Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(logs))
	}
	row := logs[0]
	if row.EventID == "" {
		t.Error("EventID must be generated when the event leaves it empty")
	}
	if row.RequestID != "req-123" || row.TraceID != "trace-456" || row.SpanID != "span-789" {
		t.Errorf("correlation IDs not persisted: %+v", row)
	}
	if row.Source != SourceREST || row.ActorType != ActorUser || row.Outcome != OutcomeSuccess {
		t.Errorf("provenance not persisted: source=%q actor=%q outcome=%q", row.Source, row.ActorType, row.Outcome)
	}
}

func TestAuditLogger_LogReliableFailsClosed(t *testing.T) {
	db := setupTestDB(t)
	logger := NewAuditLogger(repository.NewActivityLogRepository(db))

	tenantID := uint(7)
	// A rejected write must surface as an error for the caller instead of
	// being silently swallowed like best-effort Log.
	if err := db.Migrator().DropTable(&models.ActivityLog{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if err := logger.LogReliable(AuditEvent{Action: "article.unpublish", Entity: "article", TenantID: &tenantID}); err == nil {
		t.Fatal("LogReliable must return an error when the write fails")
	}
}

func TestAuditLogger_LogReliablePersistsOnHealthyDB(t *testing.T) {
	db := setupTestDB(t)
	logger := NewAuditLogger(repository.NewActivityLogRepository(db))

	tenantID := uint(9)
	if err := logger.LogReliable(AuditEvent{
		TenantID:  &tenantID,
		Action:    "article.unpublish",
		Entity:    "article",
		EntityID:  5,
		Source:    SourceREST,
		ActorType: ActorUser,
		Outcome:   OutcomeSuccess,
	}); err != nil {
		t.Fatalf("LogReliable on healthy DB: %v", err)
	}

	var count int64
	db.Model(&models.ActivityLog{}).Where("action = ? AND source = ?", "article.unpublish", SourceREST).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 reliable audit row, got %d", count)
	}
}

func TestSystemService_ActivityLogEnvelopeFilters(t *testing.T) {
	db := setupTestDB(t)
	svc := NewSystemService(db)

	seed := func(action, requestID, traceID, source, outcome string) {
		t.Helper()
		if err := db.Create(&models.ActivityLog{
			Action: action, Entity: "article",
			RequestID: requestID, TraceID: traceID,
			Source: source, ActorType: ActorUser, Outcome: outcome,
		}).Error; err != nil {
			t.Fatalf("seed log: %v", err)
		}
	}
	seed("rest.event", "req-a", "trace-a", SourceREST, OutcomeSuccess)
	seed("mcp.event", "req-b", "trace-b", SourceMCP, OutcomeDenied)
	seed("bg.event", "", "", SourceBackground, OutcomeSuccess)

	logs, total, err := svc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 10, Source: SourceMCP, Outcome: OutcomeDenied})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if total != 1 || len(logs) != 1 || logs[0].RequestID != "req-b" {
		t.Fatalf("source+outcome filter should match only the MCP denial, got total=%d logs=%+v", total, logs)
	}

	_, total, err = svc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 10, RequestID: "req-a"})
	if err != nil {
		t.Fatalf("query by request id: %v", err)
	}
	if total != 1 {
		t.Fatalf("request_id filter should match 1 row, got %d", total)
	}

	_, total, err = svc.ActivityLog(ActivityLogParams{Page: 1, PageSize: 10, TraceID: "trace-b"})
	if err != nil {
		t.Fatalf("query by trace id: %v", err)
	}
	if total != 1 {
		t.Fatalf("trace_id filter should match 1 row, got %d", total)
	}
}
