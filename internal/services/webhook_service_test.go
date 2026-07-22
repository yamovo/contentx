package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

// ─── WebhookService Tests ───────────────────────────────────────────────────

func TestWebhookService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	wh, err := svc.Create(CreateWebhookRequest{
		Name:   "test-webhook",
		URL:    "https://example.com/webhook",
		Events: []string{"article.created", "article.updated"},
	})
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	if wh.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if wh.Name != "test-webhook" {
		t.Fatalf("expected name 'test-webhook', got '%s'", wh.Name)
	}
	if !wh.IsActive {
		t.Fatal("expected new webhook to be active")
	}
	if len(wh.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(wh.Events))
	}
}

func TestWebhookService_List(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	svc.Create(CreateWebhookRequest{Name: "wh1", URL: "https://a.com/hook", Events: []string{"a"}})
	svc.Create(CreateWebhookRequest{Name: "wh2", URL: "https://b.com/hook", Events: []string{"b"}})

	webhooks, err := svc.List()
	if err != nil {
		t.Fatalf("list webhooks: %v", err)
	}
	if len(webhooks) != 2 {
		t.Fatalf("expected 2 webhooks, got %d", len(webhooks))
	}
}

func TestWebhookService_Get_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	created, _ := svc.Create(CreateWebhookRequest{Name: "get-test", URL: "https://c.com/hook", Events: []string{"x"}})

	wh, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("get webhook: %v", err)
	}
	if wh.Name != "get-test" {
		t.Fatalf("expected name 'get-test', got '%s'", wh.Name)
	}
}

func TestWebhookService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	_, err := svc.Get(99999)
	if err == nil {
		t.Fatal("expected error for non-existent webhook")
	}
}

func TestWebhookService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	created, _ := svc.Create(CreateWebhookRequest{Name: "to-delete", URL: "https://d.com/hook", Events: []string{"y"}})

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete webhook: %v", err)
	}

	var count int64
	db.Model(&models.Webhook{}).Where("id = ?", created.ID).Count(&count)
	if count != 0 {
		t.Fatal("webhook should be deleted")
	}
}

func TestWebhookService_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	err := svc.Delete(99999)
	if err == nil {
		t.Fatal("expected error for deleting non-existent webhook")
	}
}

func TestWebhookService_GetLogs_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	created, _ := svc.Create(CreateWebhookRequest{Name: "logs-test", URL: "https://e.com/hook", Events: []string{"z"}})

	logs, err := svc.GetLogs(created.ID, 10)
	if err != nil {
		t.Fatalf("get logs: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}
}

func TestWebhookService_GetLogs_WithData(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	created, _ := svc.Create(CreateWebhookRequest{Name: "logs-data", URL: "https://f.com/hook", Events: []string{"w"}})

	// Create some logs.
	db.Create(&models.WebhookLog{WebhookID: created.ID, Event: "w", Success: true, Response: 200})
	db.Create(&models.WebhookLog{WebhookID: created.ID, Event: "w", Success: false, Error: "timeout"})
	db.Create(&models.WebhookLog{WebhookID: created.ID, Event: "w", Success: true, Response: 200})

	logs, err := svc.GetLogs(created.ID, 10)
	if err != nil {
		t.Fatalf("get logs: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
}

func TestWebhookService_GetLogs_Limit(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	created, _ := svc.Create(CreateWebhookRequest{Name: "limit-test", URL: "https://g.com/hook", Events: []string{"l"}})

	for i := 0; i < 10; i++ {
		db.Create(&models.WebhookLog{WebhookID: created.ID, Event: "l", Success: true})
	}

	logs, _ := svc.GetLogs(created.ID, 3)
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs with limit, got %d", len(logs))
	}
}

func TestWebhookService_GetLogs_DefaultLimit(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWebhookService(db)

	created, _ := svc.Create(CreateWebhookRequest{Name: "default-limit", URL: "https://h.com/hook", Events: []string{"d"}})

	logs, _ := svc.GetLogs(created.ID, 0)
	if logs == nil {
		t.Fatal("expected non-nil logs slice with default limit")
	}
}
