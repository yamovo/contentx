package models

import "time"

// Webhook represents a webhook endpoint configuration.
type Webhook struct {
	ID        uint        `gorm:"primarykey" json:"id"`
	TenantID  uint        `gorm:"not null;default:1;index" json:"-"` // RFC-001 §4.3
	Name      string      `gorm:"size:128;not null" json:"name"`
	URL       string      `gorm:"size:512;not null" json:"url"`
	Events    StringSlice `gorm:"type:text" json:"events"`
	Headers   StringSlice `gorm:"type:text" json:"headers"`
	Secret    string      `gorm:"size:128" json:"-"`
	IsActive  bool        `gorm:"default:true;index" json:"is_active"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// WebhookLog records a webhook delivery attempt.
type WebhookLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TenantID  uint      `gorm:"not null;default:1;index" json:"-"` // RFC-001 §4.3
	WebhookID uint      `gorm:"index;not null" json:"webhook_id"`
	Webhook   *Webhook  `gorm:"foreignKey:WebhookID" json:"webhook,omitempty"`
	Event     string    `gorm:"size:64;not null" json:"event"`
	Payload   string    `gorm:"type:text" json:"payload"`
	Response  int       `json:"response"`
	Duration  int       `json:"duration"` // milliseconds
	Success   bool      `json:"success"`
	Error     string    `gorm:"type:text" json:"error,omitempty"`
	Retries   int       `gorm:"default:0" json:"retries"`
	CreatedAt time.Time `json:"created_at"`
}

// Webhook delivery queue statuses. A delivery is created as pending, claimed
// into delivering by the worker, and ends in exactly one terminal state.
const (
	WebhookDeliveryPending    = "pending"    // awaiting an attempt (initial or rescheduled retry)
	WebhookDeliveryDelivering = "delivering" // claimed by a worker, attempt in flight
	WebhookDeliverySuccess    = "success"    // terminal: endpoint returned 2xx
	WebhookDeliveryFailed     = "failed"     // terminal: permanent failure (4xx, webhook deleted)
	WebhookDeliveryExhausted  = "exhausted"  // terminal: retry budget used up (5xx/network)
)

// WebhookDelivery is one persistent queue entry for delivering an event to a
// webhook endpoint. WebhookService.Dispatch enqueues rows; WebhookWorker
// claims due rows, performs a single HTTP attempt per claim, and either
// resolves the row to a terminal state or reschedules it with backoff.
// Persistence makes deliveries survive process restarts (at-least-once).
type WebhookDelivery struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	TenantID     uint       `gorm:"not null;default:1;index" json:"-"` // RFC-001 §4.3
	WebhookID    uint       `gorm:"index;not null" json:"webhook_id"`
	Event        string     `gorm:"size:64;not null" json:"event"`
	Payload      string     `gorm:"type:text;not null" json:"payload"`
	Status       string     `gorm:"size:16;not null;default:pending;index" json:"status"`
	Attempts     int        `gorm:"not null;default:0" json:"attempts"`
	NextRetryAt  time.Time  `gorm:"not null;index" json:"next_retry_at"`
	ResponseCode int        `json:"response_code"`
	LastError    string     `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Webhook event constants.
const (
	WebhookEventEntryCreate    = "entry.create"
	WebhookEventEntryUpdate    = "entry.update"
	WebhookEventEntryDelete    = "entry.delete"
	WebhookEventEntryPublish   = "entry.publish"
	WebhookEventEntryUnpublish = "entry.unpublish"
	WebhookEventEntrySchedule  = "entry.schedule"
	WebhookEventMediaCreate    = "media.create"
	WebhookEventMediaDelete    = "media.delete"
	WebhookEventCommentCreate  = "comment.create"
	WebhookEventUserCreate     = "user.create"
)

// AllWebhookEvents is the list of all supported webhook events.
var AllWebhookEvents = []string{
	WebhookEventEntryCreate,
	WebhookEventEntryUpdate,
	WebhookEventEntryDelete,
	WebhookEventEntryPublish,
	WebhookEventEntryUnpublish,
	WebhookEventEntrySchedule,
	WebhookEventMediaCreate,
	WebhookEventMediaDelete,
	WebhookEventCommentCreate,
	WebhookEventUserCreate,
}
