package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
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
	})
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
	})
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

	result, err := svc.List()
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

	wh, err := svc.Get(1)
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

	_, err := svc.Get(99)
	if err == nil || err.Error() != "webhook not found" {
		t.Errorf("expected 'webhook not found', got %v", err)
	}
}

func TestMockWebhook_Delete_Success(t *testing.T) {
	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	if err := svc.Delete(1); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestMockWebhook_Delete_NotFound(t *testing.T) {
	// zeroDeleteWebhookRepo returns 0 rows affected → service returns "webhook not found".
	svc := NewWebhookServiceWithRepo(&zeroDeleteWebhookRepo{})

	err := svc.Delete(99)
	if err == nil || err.Error() != "webhook not found" {
		t.Errorf("expected 'webhook not found', got %v", err)
	}
}

func TestMockWebhook_Delete_Error(t *testing.T) {
	repo := &MockWebhookRepository{DeleteErr: gorm.ErrInvalidDB}
	svc := NewWebhookServiceWithRepo(repo)

	if err := svc.Delete(1); err == nil {
		t.Fatal("expected error")
	}
}

func TestMockWebhook_GetLogs_Success(t *testing.T) {
	repo := &MockWebhookRepository{
		Logs: []models.WebhookLog{{ID: 1, Event: "article.created"}},
	}
	svc := NewWebhookServiceWithRepo(repo)

	logs, err := svc.GetLogs(1, 50)
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
	svc.Dispatch("article.created", map[string]string{"id": "1"})

	if repo.ListActiveCalls != 1 {
		t.Errorf("expected 1 ListActive call, got %d", repo.ListActiveCalls)
	}
}

func TestMockWebhook_Dispatch_NoActiveWebhooks(t *testing.T) {
	repo := &MockWebhookRepository{ActiveWebhooks: nil}
	svc := NewWebhookServiceWithRepo(repo)

	svc.Dispatch("article.created", map[string]string{"id": "1"})

	if repo.ListActiveCalls != 1 {
		t.Errorf("expected 1 ListActive call, got %d", repo.ListActiveCalls)
	}
}

func TestMockWebhook_Dispatch_EventNotMatching(t *testing.T) {
	repo := &MockWebhookRepository{
		ActiveWebhooks: []models.Webhook{
			{ID: 1, URL: "http://localhost:1/hook", Events: models.StringSlice{"comment.created"}},
		},
	}
	svc := NewWebhookServiceWithRepo(repo)

	svc.Dispatch("article.created", map[string]string{"id": "1"})
	// Give goroutines a moment (none should fire since event doesn't match).
	time.Sleep(50 * time.Millisecond)
	// No logs should be created since no webhook matched.
	if len(repo.CreatedLogs) != 0 {
		t.Errorf("expected 0 logs for non-matching event, got %d", len(repo.CreatedLogs))
	}
}

// ---------- deliver (direct call) ----------

func TestMockWebhook_Deliver_Success(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody string
		gotHdr  http.Header
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotHdr = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	wh := models.Webhook{ID: 1, URL: srv.URL, Events: models.StringSlice{"article.created"}}
	payload := WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: map[string]string{"id": "1"}}

	svc.deliver(wh, payload)

	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log created, got %d", len(repo.CreatedLogs))
	}
	log := repo.CreatedLogs[0]
	if !log.Success {
		t.Errorf("expected success=true, got false (error: %s)", log.Error)
	}
	if log.Response != http.StatusOK {
		t.Errorf("expected response 200, got %d", log.Response)
	}
	if log.Event != "article.created" {
		t.Errorf("expected event article.created, got %s", log.Event)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(gotBody, `"event":"article.created"`) {
		t.Errorf("expected body to contain event, got: %s", gotBody)
	}
	if gotHdr.Get("X-ContentX-Event") != "article.created" {
		t.Errorf("expected X-ContentX-Event header, got %s", gotHdr.Get("X-ContentX-Event"))
	}
	if gotHdr.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", gotHdr.Get("Content-Type"))
	}
}

func TestMockWebhook_Deliver_Non2xxResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	wh := models.Webhook{ID: 1, URL: srv.URL}
	payload := WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"}

	svc.deliver(wh, payload)

	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	if repo.CreatedLogs[0].Success {
		t.Error("expected success=false for 500 response")
	}
	if repo.CreatedLogs[0].Response != http.StatusInternalServerError {
		t.Errorf("expected response 500, got %d", repo.CreatedLogs[0].Response)
	}
}

func TestMockWebhook_Deliver_RequestError(t *testing.T) {
	// Invalid URL → http.NewRequest succeeds but client.Do fails.
	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	wh := models.Webhook{ID: 1, URL: "http://127.0.0.1:0/nonexistent"} // port 0 won't connect
	payload := WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"}

	svc.deliver(wh, payload)

	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	if repo.CreatedLogs[0].Success {
		t.Error("expected success=false on request error")
	}
	if repo.CreatedLogs[0].Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestMockWebhook_Deliver_HMACSignature(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-ContentX-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	wh := models.Webhook{
		ID:     1,
		URL:    srv.URL,
		Secret: "my-secret",
	}
	payload := WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"}

	svc.deliver(wh, payload)

	// Verify the signature header was set and matches expected HMAC.
	if gotSig == "" {
		t.Fatal("expected non-empty X-ContentX-Signature header")
	}
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Fatalf("expected sig to start with sha256=, got %s", gotSig)
	}
	// Recompute expected sig from the log payload (which holds the marshaled body).
	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	h := hmac.New(sha256.New, []byte("my-secret"))
	h.Write([]byte(repo.CreatedLogs[0].Payload))
	expected := "sha256=" + hex.EncodeToString(h.Sum(nil))
	if gotSig != expected {
		t.Errorf("expected sig %s, got %s", expected, gotSig)
	}
}

func TestMockWebhook_Deliver_BadURL(t *testing.T) {
	// Malformed URL → http.NewRequest returns error.
	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	wh := models.Webhook{ID: 1, URL: "://bad-url"}
	payload := WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"}

	svc.deliver(wh, payload)
	// No log should be created since request creation failed.
	if len(repo.CreatedLogs) != 0 {
		t.Errorf("expected 0 logs on bad URL, got %d", len(repo.CreatedLogs))
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

func (z *zeroDeleteWebhookRepo) Create(_ *models.Webhook) error { return nil }
func (z *zeroDeleteWebhookRepo) List() ([]models.Webhook, error) { return nil, nil }
func (z *zeroDeleteWebhookRepo) GetByID(_ uint) (*models.Webhook, error) {
	return nil, gorm.ErrRecordNotFound
}
func (z *zeroDeleteWebhookRepo) Delete(_ uint) (int64, error) { return 0, nil }
func (z *zeroDeleteWebhookRepo) ListLogs(_ uint, _ int) ([]models.WebhookLog, error) {
	return nil, nil
}
func (z *zeroDeleteWebhookRepo) CreateLog(_ *models.WebhookLog) error { return nil }
func (z *zeroDeleteWebhookRepo) ListActive() ([]models.Webhook, error) { return nil, nil }
