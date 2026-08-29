package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

func TestMockWebhook_NewWithRepo(t *testing.T) {
	svc := NewWebhookServiceWithRepo(&MockWebhookRepository{})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------- CRUD ----------

func TestMockWebhook_Create_Success(t *testing.T) {
	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	wh, err := svc.Create(CreateWebhookRequest{
		Name: "test", URL: "https://example.com/hook", Events: []string{"article.created"},
	}, 1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !wh.IsActive {
		t.Error("expected IsActive=true by default")
	}
	if len(repo.CreatedWebhooks) != 1 {
		t.Errorf("expected 1 created webhook, got %d", len(repo.CreatedWebhooks))
	}
}

func TestMockWebhook_Create_Error(t *testing.T) {
	repo := &MockWebhookRepository{CreateErr: gorm.ErrInvalidDB}
	svc := NewWebhookServiceWithRepo(repo)

	_, err := svc.Create(CreateWebhookRequest{
		Name: "test", URL: "https://example.com/hook", Events: []string{"article.created"},
	}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockWebhook_List_Success(t *testing.T) {
	repo := &MockWebhookRepository{
		WebhooksList: []models.Webhook{
			{Name: "hook1"},
			{Name: "hook2"},
		},
	}
	svc := NewWebhookServiceWithRepo(repo)

	result, err := svc.List(1)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(result))
	}
}

func TestMockWebhook_Get_Success(t *testing.T) {
	repo := &MockWebhookRepository{
		Webhook: &models.Webhook{ID: 1, Name: "hook1"},
	}
	svc := NewWebhookServiceWithRepo(repo)

	wh, err := svc.Get(1, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if wh.Name != "hook1" {
		t.Errorf("expected name hook1, got %s", wh.Name)
	}
}

func TestMockWebhook_Get_NotFound(t *testing.T) {
	repo := &MockWebhookRepository{GetByIDErr: gorm.ErrRecordNotFound}
	svc := NewWebhookServiceWithRepo(repo)

	_, err := svc.Get(99, 1)
	if err == nil || err.Error() != "webhook not found" {
		t.Errorf("expected 'webhook not found', got %v", err)
	}
}

func TestMockWebhook_Delete_Success(t *testing.T) {
	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	if err := svc.Delete(1, 1); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestMockWebhook_Delete_NotFound(t *testing.T) {
	// zeroDeleteWebhookRepo returns 0 rows affected → service returns "webhook not found".
	svc := NewWebhookServiceWithRepo(&zeroDeleteWebhookRepo{})

	err := svc.Delete(99, 1)
	if err == nil || err.Error() != "webhook not found" {
		t.Errorf("expected 'webhook not found', got %v", err)
	}
}

func TestMockWebhook_Delete_Error(t *testing.T) {
	repo := &MockWebhookRepository{DeleteErr: gorm.ErrInvalidDB}
	svc := NewWebhookServiceWithRepo(repo)

	if err := svc.Delete(1, 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestMockWebhook_GetLogs_Success(t *testing.T) {
	repo := &MockWebhookRepository{
		Logs: []models.WebhookLog{{ID: 1, Event: "article.created"}},
	}
	svc := NewWebhookServiceWithRepo(repo)

	logs, err := svc.GetLogs(1, 50, 1)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(logs))
	}
}

// ---------- Dispatch ----------

func TestMockWebhook_Dispatch_ListActiveError(t *testing.T) {
	repo := &MockWebhookRepository{ListActiveErr: gorm.ErrInvalidDB}
	svc := NewWebhookServiceWithRepo(repo)

	// Dispatch is fire-and-forget; should not panic, just log and return.
	svc.Dispatch("article.created", map[string]string{"id": "1"}, 1)

	if repo.ListActiveCalls != 1 {
		t.Errorf("expected 1 ListActive call, got %d", repo.ListActiveCalls)
	}
}

func TestMockWebhook_Dispatch_NoActiveWebhooks(t *testing.T) {
	repo := &MockWebhookRepository{ActiveWebhooks: nil}
	svc := NewWebhookServiceWithRepo(repo)

	svc.Dispatch("article.created", map[string]string{"id": "1"}, 1)

	if repo.ListActiveCalls != 1 {
		t.Errorf("expected 1 ListActive call, got %d", repo.ListActiveCalls)
	}
	if len(repo.EnqueuedDeliveries) != 0 {
		t.Errorf("expected 0 enqueued deliveries, got %d", len(repo.EnqueuedDeliveries))
	}
}

func TestMockWebhook_Dispatch_EventNotMatching(t *testing.T) {
	repo := &MockWebhookRepository{
		ActiveWebhooks: []models.Webhook{
			{ID: 1, URL: "http://localhost:1/hook", Events: models.StringSlice{"comment.created"}},
		},
	}
	svc := NewWebhookServiceWithRepo(repo)

	svc.Dispatch("article.created", map[string]string{"id": "1"}, 1)

	// Dispatch is synchronous and only enqueues for matching events.
	if len(repo.EnqueuedDeliveries) != 0 {
		t.Errorf("expected 0 enqueued deliveries for non-matching event, got %d", len(repo.EnqueuedDeliveries))
	}
}

func TestMockWebhook_Dispatch_EnqueuesForMatchingWebhooks(t *testing.T) {
	repo := &MockWebhookRepository{
		ActiveWebhooks: []models.Webhook{
			{ID: 1, URL: "http://localhost:1/a", Events: models.StringSlice{"article.created"}},
			{ID: 2, URL: "http://localhost:1/b", Events: models.StringSlice{"article.created"}},
			{ID: 3, URL: "http://localhost:1/c", Events: models.StringSlice{"comment.created"}},
		},
	}
	svc := NewWebhookServiceWithRepo(repo)

	svc.Dispatch("article.created", map[string]string{"id": "1"}, 1)

	if len(repo.EnqueuedDeliveries) != 2 {
		t.Fatalf("expected 2 enqueued deliveries (matching webhooks), got %d", len(repo.EnqueuedDeliveries))
	}
	for _, d := range repo.EnqueuedDeliveries {
		if d.Status != models.WebhookDeliveryPending {
			t.Errorf("expected pending status, got %s", d.Status)
		}
		if d.Event != "article.created" {
			t.Errorf("expected event article.created, got %s", d.Event)
		}
		if d.Payload == "" {
			t.Error("expected non-empty payload")
		}
	}
}

// ---------- hmacSign ----------

func TestHmacSign(t *testing.T) {
	// Verify hmacSign matches a manually computed HMAC-SHA256.
	secret := []byte("test-secret")
	data := []byte(`{"event":"article.created"}`)

	sig := hmacSign(secret, data)

	h := hmac.New(sha256.New, secret)
	h.Write(data)
	expected := hex.EncodeToString(h.Sum(nil))

	if sig != expected {
		t.Errorf("expected %s, got %s", expected, sig)
	}
	// Verify it's a 64-char hex string.
	if len(sig) != 64 {
		t.Errorf("expected 64-char hex, got %d chars", len(sig))
	}
}

// zeroDeleteWebhookRepo is a minimal WebhookRepository whose Delete returns 0 rows affected.
type zeroDeleteWebhookRepo struct{}

func (z *zeroDeleteWebhookRepo) Create(_ *models.Webhook) error        { return nil }
func (z *zeroDeleteWebhookRepo) List(_ uint) ([]models.Webhook, error) { return nil, nil }
func (z *zeroDeleteWebhookRepo) GetByID(_, _ uint) (*models.Webhook, error) {
	return nil, gorm.ErrRecordNotFound
}
func (z *zeroDeleteWebhookRepo) Delete(_, _ uint) (int64, error) { return 0, nil }
func (z *zeroDeleteWebhookRepo) ListLogs(_ uint, _ int, _ uint) ([]models.WebhookLog, error) {
	return nil, nil
}
func (z *zeroDeleteWebhookRepo) CreateLog(_ *models.WebhookLog) error        { return nil }
func (z *zeroDeleteWebhookRepo) ListActive(_ uint) ([]models.Webhook, error) { return nil, nil }

// Queue stubs (not exercised by this mock).
func (z *zeroDeleteWebhookRepo) EnqueueDelivery(_ *models.WebhookDelivery) error {
	return nil
}
func (z *zeroDeleteWebhookRepo) ClaimDueDeliveries(_ time.Time, _ int) ([]models.WebhookDelivery, error) {
	return nil, nil
}
func (z *zeroDeleteWebhookRepo) CompleteDelivery(_ uint, _ repository.DeliveryOutcome) error {
	return nil
}
func (z *zeroDeleteWebhookRepo) RequeueStaleDeliveries() (int64, error)       { return 0, nil }
func (z *zeroDeleteWebhookRepo) CountPendingDeliveries(_ uint) (int64, error) { return 0, nil }
