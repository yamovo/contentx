package services

// SEC-1 Webhook SSRF 防护测试：
//   - IP 黑名单判定（loopback / RFC1918 / link-local 云元数据 / ULA / unspecified）
//   - 加固客户端在 dial 层拦截内网/元数据目标（请求不落到目标服务器）
//   - redirect 不被跟随
//   - Create 时的 URL 早期校验（scheme / 字面内网 IP / release 强制 https）

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yamovo/contentx/internal/errs"
)

func TestWebhookSSRF_DisallowedIPTable(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"10.0.0.1",        // RFC1918
		"172.16.0.1",      // RFC1918
		"192.168.1.1",     // RFC1918
		"169.254.169.254", // link-local (cloud metadata)
		"0.0.0.0",         // unspecified
		"::1",             // IPv6 loopback
		"fe80::1",         // IPv6 link-local
		"fc00::1",         // IPv6 ULA
	}
	for _, s := range blocked {
		if !isDisallowedWebhookIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "93.184.216.34", "2606:4700::1111"}
	for _, s := range allowed {
		if isDisallowedWebhookIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
}

// TestWebhookSSRF_LoopbackBlockedAtDial: the hardened delivery client blocks
// loopback targets at the dial layer — the target server must receive nothing.
func TestWebhookSSRF_LoopbackBlockedAtDial(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()

	client := newWebhookHTTPClient(false) // hardened (private targets blocked)
	resp, err := client.Post(srv.URL, "application/json", strings.NewReader("{}"))
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected dial-layer block error for loopback target")
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected 0 requests to reach loopback target, got %d", got)
	}
	if !strings.Contains(err.Error(), "SSRF") {
		t.Errorf("expected SSRF block error, got %q", err.Error())
	}
}

// TestWebhookSSRF_MetadataEndpointBlocked: the cloud metadata address is
// rejected at dial time (no request leaves the process).
func TestWebhookSSRF_MetadataEndpointBlocked(t *testing.T) {
	client := newWebhookHTTPClient(false)
	resp, err := client.Post("http://169.254.169.254/latest/meta-data/", "application/json", strings.NewReader("{}"))
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error for metadata endpoint")
	}
	if !strings.Contains(err.Error(), "SSRF") {
		t.Errorf("expected SSRF block error, got %q", err.Error())
	}
}

// TestWebhookSSRF_RedirectNotFollowed: 3xx is not followed, so a redirect to
// an internal target never receives a request.
func TestWebhookSSRF_RedirectNotFollowed(t *testing.T) {
	var redirectTargetCalls int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&redirectTargetCalls, 1)
	}))
	defer target.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	client := newWebhookHTTPClient(true) // allow loopback to exercise redirect behaviour
	resp, err := client.Post(srv.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&redirectTargetCalls); got != 0 {
		t.Fatalf("expected redirect target not to be called, got %d calls", got)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 (redirect not followed), got %d", resp.StatusCode)
	}
}

// TestWebhookSSRF_AllowPrivateOptIn: with the escape hatch enabled, loopback
// delivery succeeds.
func TestWebhookSSRF_AllowPrivateOptIn(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newWebhookHTTPClient(true) // opt-in → permissive
	resp, err := client.Post(srv.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("unexpected error with opt-in: %v", err)
	}
	_ = resp.Body.Close()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 request with opt-in, got %d", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestValidateWebhookURL(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_PRIVATE_TARGETS", "")
	t.Setenv("SERVER_MODE", "debug")

	valid := []string{
		"https://example.com/hook",
		"http://example.com/hook", // http allowed in debug mode
	}
	for _, u := range valid {
		if err := validateWebhookURL(u); err != nil {
			t.Errorf("expected %s to be valid, got %v", u, err)
		}
	}

	invalid := []string{
		"ftp://example.com/hook",           // scheme not allowed
		"file:///etc/passwd",               // scheme not allowed
		"http://169.254.169.254/meta-data", // literal metadata IP
		"http://127.0.0.1:8080/hook",       // literal loopback
		"https://10.0.0.5/hook",            // literal RFC1918
		"http://[::1]/hook",                // literal IPv6 loopback
	}
	for _, u := range invalid {
		if err := validateWebhookURL(u); err == nil {
			t.Errorf("expected %s to be rejected", u)
		}
	}
}

func TestValidateWebhookURL_ReleaseRequiresHTTPS(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_PRIVATE_TARGETS", "")
	t.Setenv("SERVER_MODE", "release")

	if err := validateWebhookURL("http://example.com/hook"); err == nil {
		t.Error("expected http URL to be rejected in release mode")
	}
	if err := validateWebhookURL("https://example.com/hook"); err != nil {
		t.Errorf("expected https URL to be accepted in release mode, got %v", err)
	}
}

func TestValidateWebhookURL_AllowPrivateOptIn(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_PRIVATE_TARGETS", "true")

	if err := validateWebhookURL("http://127.0.0.1:8080/hook"); err != nil {
		t.Errorf("expected loopback URL to be accepted with opt-in, got %v", err)
	}
}

// TestWebhookCreate_RejectsUnsafeURL: Create 层早期拒绝并返回 400 语义错误。
func TestWebhookCreate_RejectsUnsafeURL(t *testing.T) {
	t.Setenv("WEBHOOK_ALLOW_PRIVATE_TARGETS", "")

	repo := &MockWebhookRepository{}
	svc := NewWebhookServiceWithRepo(repo)

	_, err := svc.Create(CreateWebhookRequest{
		Name: "evil", URL: "http://169.254.169.254/latest/meta-data/", Events: []string{"article.created"},
	}, 1)
	var appErr *errs.AppError
	if !errs.Is(err, &appErr) || appErr.Code != errs.ErrBadRequest.Code {
		t.Fatalf("expected bad-request error, got %v", err)
	}
	if len(repo.CreatedWebhooks) != 0 {
		t.Errorf("expected no webhook persisted, got %d", len(repo.CreatedWebhooks))
	}
}
