package models

import "time"

// Webhook represents a webhook endpoint configuration.
type Webhook struct {
	ID        uint        `gorm:"primarykey" json:"id"`
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
	WebhookID uint      `gorm:"index;not null" json:"webhook_id"`
	Webhook   *Webhook  `gorm:"foreignKey:WebhookID" json:"webhook,omitempty"`
	Event     string    `gorm:"size:64;not null" json:"event"`
	Payload   string    `gorm:"type:text" json:"payload"`
	Response  int       `json:"response"`
	Duration  int       `json:"duration"` // milliseconds
	Success   bool      `json:"success"`
	Error     string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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
