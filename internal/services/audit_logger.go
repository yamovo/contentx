package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
)

// Audit event provenance values. Source names the channel that produced the
// event; ActorType classifies who acted; Outcome records what happened. They
// are first-class ActivityLog columns (migration 013) so REST, MCP, and
// background events share one verifiable envelope (RESEARCH-001 §4).
const (
	SourceREST       = "rest"
	SourceMCP        = "mcp"
	SourceBackground = "background"
	SourceSystem     = "system"

	ActorUser      = "user"
	ActorToken     = "token"
	ActorAnonymous = "anonymous"
	ActorSystem    = "system"

	OutcomeSuccess = "success"
	OutcomeFailed  = "failed"
	OutcomeDenied  = "denied"
)

// AuditEvent is a business-level audit record. IP and UserAgent are optional —
// services that already receive them (AuthService, CommentService) populate
// them; other services leave them empty and the HTTP-layer ActivityLogger
// middleware still records the same request with IP/UA for correlation.
type AuditEvent struct {
	UserID    *uint  // nil for anonymous events (e.g. failed login for unknown user)
	TenantID  *uint  // nil for platform-level events (e.g. login, user management)
	Action    string // e.g. "article.publish", "user.disable", "login.failed"
	Entity    string // e.g. "article", "user", "role"
	EntityID  uint   // 0 when not applicable
	Details   any    // marshalled to JSON; sensitive fields must be redacted by caller
	IP        string // optional
	UserAgent string // optional

	// Envelope provenance. EventID is generated when empty. Request/trace IDs
	// are filled by callers that have request context (HTTP middleware, MCP
	// transport); background writers leave them empty.
	EventID   string
	RequestID string
	TraceID   string
	SpanID    string
	Source    string // SourceREST | SourceMCP | SourceBackground | SourceSystem
	ActorType string // ActorUser | ActorToken | ActorAnonymous | ActorSystem
	Outcome   string // OutcomeSuccess | OutcomeFailed | OutcomeDenied
}

// AuditLogger records business-level audit events with EntityID and Details,
// complementing the HTTP-level ActivityLogger middleware which only captures
// method/route/IP/UA. Best-effort: write failures are logged but never
// propagated to the caller, so an audit write failure cannot break a business
// operation.
type AuditLogger interface {
	Log(event AuditEvent)
	// LogReliable writes the event and returns an error when it could not be
	// persisted after bounded retries. High-risk operations (publish/unpublish,
	// role, tenant, and credential changes) use it so a lost audit record
	// surfaces as an explicit operation failure instead of being silently
	// dropped (RESEARCH-001 §4: outbox or explicit fail-closed strategy).
	LogReliable(event AuditEvent) error
}

// AuditLogger implementation notes: Log is best-effort for ordinary events.
// High-risk operations use LogReliable, which surfaces persistence failures to
// the caller so they can fail visibly instead of silently losing the audit
// trail (RESEARCH-001 §4).

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

func (NoopAuditLogger) Log(AuditEvent)               {}
func (NoopAuditLogger) LogReliable(AuditEvent) error { return nil }

type auditLogger struct {
	repo repository.ActivityLogRepository
}

// NewAuditLogger returns an AuditLogger backed by the given repository.
func NewAuditLogger(repo repository.ActivityLogRepository) AuditLogger {
	return &auditLogger{repo: repo}
}

// buildLog marshals and redacts the event details and materialises the
// envelope onto the storage model. Shared by Log and LogReliable so both
// write identical envelopes for the same event.
func buildLog(event AuditEvent) *models.ActivityLog {
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
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	return &models.ActivityLog{
		UserID:    event.UserID,
		TenantID:  event.TenantID,
		Action:    event.Action,
		Entity:    event.Entity,
		EntityID:  event.EntityID,
		Details:   detailStr,
		IP:        event.IP,
		UserAgent: event.UserAgent,
		EventID:   event.EventID,
		RequestID: event.RequestID,
		TraceID:   event.TraceID,
		SpanID:    event.SpanID,
		Source:    event.Source,
		ActorType: event.ActorType,
		Outcome:   event.Outcome,
	}
}

func (a *auditLogger) Log(event AuditEvent) {
	if err := a.repo.Create(buildLog(event)); err != nil {
		// Audit writes must never break the business operation; log and move on.
		slog.Warn("audit log write failed",
			"error", err,
			"action", event.Action,
			"entity", event.Entity,
			"entity_id", event.EntityID,
		)
	}
}

func (a *auditLogger) LogReliable(event AuditEvent) error {
	log := buildLog(event)
	// Bounded retry so a transient database hiccup does not fail a high-risk
	// business operation; persistent failure returns an error the caller must
	// surface as a fail-closed rejection.
	const attempts = 3
	var err error
	for i := 0; i < attempts; i++ {
		if err = a.repo.Create(log); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
		}
	}
	return fmt.Errorf("audit event %s (%s/%d) not persisted after %d attempts: %w",
		event.Action, event.Entity, event.EntityID, attempts, err)
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
