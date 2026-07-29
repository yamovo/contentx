package services

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
)

// AuditEvent is a business-level audit record. IP and UserAgent are optional —
// services that already receive them (AuthService, CommentService) populate
// them; other services leave them empty and the HTTP-layer ActivityLogger
// middleware still records the same request with IP/UA for correlation.
type AuditEvent struct {
	UserID    *uint  // nil for anonymous events (e.g. failed login for unknown user)
	Action    string // e.g. "article.publish", "user.disable", "login.failed"
	Entity    string // e.g. "article", "user", "role"
	EntityID  uint   // 0 when not applicable
	Details   any    // marshalled to JSON; sensitive fields must be redacted by caller
	IP        string // optional
	UserAgent string // optional
}

// AuditLogger records business-level audit events with EntityID and Details,
// complementing the HTTP-level ActivityLogger middleware which only captures
// method/route/IP/UA. Best-effort: write failures are logged but never
// propagated to the caller, so an audit write failure cannot break a business
// operation.
type AuditLogger interface {
	Log(event AuditEvent)
}

// auditActor returns a stable pointer for an optional actor ID. Service
// methods accept the actor as a variadic argument to preserve compatibility
// with non-HTTP callers while authenticated handlers always supply it.
func auditActor(actorIDs []uint) *uint {
	if len(actorIDs) == 0 || actorIDs[0] == 0 {
		return nil
	}
	id := actorIDs[0]
	return &id
}

// NoopAuditLogger is a no-op implementation used by tests and services that
// don't need auditing. It satisfies the interface without a database.
type NoopAuditLogger struct{}

func (NoopAuditLogger) Log(AuditEvent) {}

type auditLogger struct {
	repo repository.ActivityLogRepository
}

// NewAuditLogger returns an AuditLogger backed by the given repository.
func NewAuditLogger(repo repository.ActivityLogRepository) AuditLogger {
	return &auditLogger{repo: repo}
}

func (a *auditLogger) Log(event AuditEvent) {
	detailStr := ""
	if event.Details != nil {
		if b, err := json.Marshal(event.Details); err == nil {
			var normalized any
			if json.Unmarshal(b, &normalized) == nil {
				b, _ = json.Marshal(redactAuditValue(normalized))
			}
			detailStr = string(b)
		}
	}
	log := &models.ActivityLog{
		UserID:    event.UserID,
		Action:    event.Action,
		Entity:    event.Entity,
		EntityID:  event.EntityID,
		Details:   detailStr,
		IP:        event.IP,
		UserAgent: event.UserAgent,
	}
	if err := a.repo.Create(log); err != nil {
		// Audit writes must never break the business operation; log and move on.
		slog.Warn("audit log write failed",
			"error", err,
			"action", event.Action,
			"entity", event.Entity,
			"entity_id", event.EntityID,
		)
	}
}

func redactAuditValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveAuditKey(key) {
				out[key] = "***"
			} else {
				out[key] = redactAuditValue(child)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactAuditValue(child)
		}
		return out
	default:
		return value
	}
}

func isSensitiveAuditKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, marker := range []string{
		"password", "passwd", "secret", "token", "authorization",
		"cookie", "totp", "api_key", "apikey", "private_key",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
