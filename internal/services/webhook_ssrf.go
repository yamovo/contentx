package services

// Webhook SSRF 防护（SEC-1）：
//   - 投递客户端使用自定义 Dialer Control，在 DNS 解析后的连接阶段拦截
//     loopback / RFC1918 私网 / link-local（含云元数据 169.254.169.254）/
//     ULA / 未指定地址，同时天然防御 DNS rebinding（校验的是实际拨号 IP）。
//   - 不跟随 redirect（3xx 原样返回，避免跳转到内网目标）。
//   - release 模式下 Create 仅接受 https URL。
//   - 自部署内网投递场景可显式设置 WEBHOOK_ALLOW_PRIVATE_TARGETS=true 放行。

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// allowPrivateWebhookTargets reports whether private/internal delivery targets
// are explicitly allowed by the operator (opt-in escape hatch).
func allowPrivateWebhookTargets() bool {
	return os.Getenv("WEBHOOK_ALLOW_PRIVATE_TARGETS") == "true"
}

// isDisallowedWebhookIP reports whether ip must never be a webhook target:
// loopback, private (RFC1918 + ULA), link-local (incl. cloud metadata
// 169.254.169.254), unspecified and multicast addresses.
func isDisallowedWebhookIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// newWebhookHTTPClient builds the hardened HTTP client used for webhook
// delivery. allowPrivate disables the IP blocklist (tests / trusted intranet).
func newWebhookHTTPClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("webhook: invalid dial address %q: %w", address, err)
			}
			ip := net.ParseIP(host)
			if ip == nil || isDisallowedWebhookIP(ip) {
				return fmt.Errorf("webhook: delivery to %s blocked (SSRF protection)", host)
			}
			return nil
		},
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(transport),
		// 不跟随 redirect：3xx 原样返回并按非 2xx 记录，防止跳转绕过 IP 校验。
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// validateWebhookURL rejects obviously unsafe webhook URLs at creation time.
// The dial-time Control check remains the authoritative enforcement; this is
// an early, user-friendly rejection (scheme check + literal-IP blocklist).
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if os.Getenv("SERVER_MODE") == "release" && !allowPrivateWebhookTargets() {
			return fmt.Errorf("webhook URL must use https in release mode")
		}
	default:
		return fmt.Errorf("webhook URL scheme %q is not allowed", u.Scheme)
	}
	if allowPrivateWebhookTargets() {
		return nil
	}
	// 字面 IP 直接拒绝；域名留给 dial 层在解析后校验（防 DNS rebinding）。
	if ip := net.ParseIP(u.Hostname()); ip != nil && isDisallowedWebhookIP(ip) {
		return fmt.Errorf("webhook URL targets a disallowed address (SSRF protection)")
	}
	return nil
}
