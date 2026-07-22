package models

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gosimple/slug"
)

// ---------- User helpers ----------

// IsActive checks if the user account is active.
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

// IsAdmin checks if the user has the admin role.
func (u *User) IsAdmin() bool {
	return u.Role.Slug == "admin"
}

// IsEditor checks if the user has editor-or-above privileges.
func (u *User) IsEditor() bool {
	return u.Role.Slug == "admin" || u.Role.Slug == "editor"
}

// AvatarURL returns the avatar URL or a default.
func (u *User) AvatarURL() string {
	if u.Avatar != "" {
		return u.Avatar
	}
	return fmt.Sprintf("https://ui-avatars.com/api/?name=%s&size=200&background=random",
		strings.ReplaceAll(u.DisplayName, " ", "+"))
}

// RecordLogin updates login metadata.
func (u *User) RecordLogin(ip string) {
	now := time.Now()
	u.LastLoginAt = &now
	u.LastLoginIP = ip
	u.LoginCount++
}

// ---------- Article helpers ----------

var (
	stripHTMLRe  = regexp.MustCompile(`<[^>]*>`)
	multiSpaceRe = regexp.MustCompile(`\s+`)
)

// GenerateSlug creates a URL-safe slug from the title.
func (a *Article) GenerateSlug() {
	if a.Slug == "" {
		a.Slug = slug.MakeLang(a.Title, "zh")
		if a.Slug == "" {
			a.Slug = slug.Make(a.Title)
		}
	}
	// Ensure uniqueness is handled at the repository layer.
}

// CalcReadingTime estimates reading time in minutes (≈300 CJK / ≈200 EN words per minute).
func (a *Article) CalcReadingTime() {
	text := stripHTMLRe.ReplaceAllString(a.Content, "")
	a.WordCount = utf8.RuneCountInString(text)
	// CJK chars count as 1 word each; average 200 wpm for mixed content.
	if a.WordCount > 0 {
		mins := a.WordCount / 200
		if mins < 1 {
			mins = 1
		}
		a.ReadingTime = mins
	}
}

// MakeExcerpt generates an excerpt from content if not set.
func (a *Article) MakeExcerpt(maxLen int) {
	if a.Excerpt != "" {
		return
	}
	text := stripHTMLRe.ReplaceAllString(a.Content, "")
	text = multiSpaceRe.ReplaceAllString(strings.TrimSpace(text), " ")
	text = html.UnescapeString(text)
	if maxLen <= 0 {
		maxLen = 200
	}
	runes := []rune(text)
	if len(runes) > maxLen {
		a.Excerpt = string(runes[:maxLen]) + "…"
	} else {
		a.Excerpt = text
	}
}

// IsPublished checks if the article is published.
func (a *Article) IsPublished() bool {
	return a.Status == StatusPublished
}

// IsVisible checks visibility considering password protection.
func (a *Article) IsVisible() bool {
	return a.Visibility == VisibilityPublic && a.IsPublished()
}

// Publish sets the article to published status and records publish time.
func (a *Article) Publish() {
	a.Status = StatusPublished
	if a.PublishedAt == nil {
		now := time.Now()
		a.PublishedAt = &now
	}
}

// Unpublish reverts a published/scheduled article back to draft, keeping the
// historical PublishedAt value intact.
func (a *Article) Unpublish() {
	a.Status = StatusDraft
}

// SubmitForReview moves a draft into the pending (review) queue.
func (a *Article) SubmitForReview() {
	a.Status = StatusPending
}

// Approve marks a pending article as published.
func (a *Article) Approve() {
	a.Status = StatusPublished
	if a.PublishedAt == nil {
		now := time.Now()
		a.PublishedAt = &now
	}
}

// Schedule marks a draft/pending article for automatic publication at the
// given time. The article stays non-public until the scheduler flips it.
func (a *Article) Schedule(at time.Time) {
	a.Status = StatusScheduled
	a.ScheduledAt = &at
}

// Archive moves an article out of the active lifecycle.
func (a *Article) Archive() {
	a.Status = StatusArchived
}

// Trash moves the article to trash.
func (a *Article) Trash() {
	a.Status = StatusTrash
}

// CanTransitionTo reports whether transitioning from the article's current
// status to target is allowed by the publication state machine.
//
// Allowed transitions:
//   draft      → pending, published, scheduled, archived, trash
//   pending    → draft, published, archived, trash
//   published  → draft, archived, trash
//   scheduled  → draft, published, archived, trash
//   archived   → draft, trash
//   trash      → draft, published (restore)
func (a *Article) CanTransitionTo(target ArticleStatus) bool {
	return AllowedTransition(a.Status, target)
}

// AllowedTransition reports whether transitioning from one status to another
// is permitted by the publication state machine. Extracted as a free function
// so callers can validate a target before applying it.
func AllowedTransition(from, to ArticleStatus) bool {
	if from == to {
		return true // no-op transitions are always allowed
	}
	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

// transitions defines the legal status transitions for the publication
// workflow. Keys are source statuses; values are sets of allowed targets.
var transitions = map[ArticleStatus]map[ArticleStatus]struct{}{
	StatusDraft: {
		StatusPending:   {},
		StatusPublished: {},
		StatusScheduled: {},
		StatusArchived:  {},
		StatusTrash:     {},
	},
	StatusPending: {
		StatusDraft:     {},
		StatusPublished: {},
		StatusArchived:  {},
		StatusTrash:     {},
	},
	StatusPublished: {
		StatusDraft:    {},
		StatusArchived: {},
		StatusTrash:    {},
	},
	StatusScheduled: {
		StatusDraft:     {},
		StatusPublished: {},
		StatusArchived:  {},
		StatusTrash:     {},
	},
	StatusArchived: {
		StatusDraft: {},
		StatusTrash: {},
	},
	StatusTrash: {
		StatusDraft:     {},
		StatusPublished: {},
	},
}

// ---------- Category helpers ----------

// FullPath returns "Parent / Child" string.
func (c *Category) FullPath() string {
	if c.Parent != nil {
		return c.Parent.FullPath() + " / " + c.Name
	}
	return c.Name
}

// ---------- Comment helpers ----------

// IsApproved checks if the comment is approved.
func (c *Comment) IsApproved() bool {
	return c.Status == "approved"
}

// IsPending checks if the comment is pending moderation.
func (c *Comment) IsPending() bool {
	return c.Status == "pending"
}

// Approve sets comment status to approved.
func (c *Comment) Approve() {
	c.Status = "approved"
}

// Spam marks the comment as spam.
func (c *Comment) Spam() {
	c.Status = "spam"
}

// AuthorDisplayName returns the best available display name.
func (c *Comment) AuthorDisplayName() string {
	if c.User != nil && c.User.DisplayName != "" {
		return c.User.DisplayName
	}
	if c.User != nil {
		return c.User.Username
	}
	if c.AuthorName != "" {
		return c.AuthorName
	}
	return "Anonymous"
}

// ---------- Media helpers ----------

// IsImage checks if the media file is an image.
func (m *Media) IsImage() bool {
	return strings.HasPrefix(m.MimeType, "image/")
}

// IsVideo checks if the media file is a video.
func (m *Media) IsVideo() bool {
	return strings.HasPrefix(m.MimeType, "video/")
}

// IsAudio checks if the media file is an audio.
func (m *Media) IsAudio() bool {
	return strings.HasPrefix(m.MimeType, "audio/")
}

// FileSizeFormatted returns human-readable file size.
func (m *Media) FileSizeFormatted() string {
	size := float64(m.FileSize)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for _, u := range units {
		if size < 1024 {
			return fmt.Sprintf("%.1f %s", size, u)
		}
		size /= 1024
	}
	return fmt.Sprintf("%.1f PB", size)
}

// Extension returns the file extension.
func (m *Media) Extension() string {
	parts := strings.Split(m.Filename, ".")
	if len(parts) > 1 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

// ---------- Pagination ----------

// Paginate is a helper for pagination params.
type Paginate struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int64 `json:"total"`
}

// Offset calculates the SQL OFFSET value.
func (p *Paginate) Offset() int {
	if p.Page <= 0 {
		p.Page = 1
	}
	return (p.Page - 1) * p.PageSize
}

// TotalPages returns the total number of pages.
func (p *Paginate) TotalPages() int {
	if p.PageSize <= 0 {
		return 0
	}
	pages := int(p.Total) / p.PageSize
	if int(p.Total)%p.PageSize > 0 {
		pages++
	}
	return pages
}

// HasNext checks if there is a next page.
func (p *Paginate) HasNext() bool {
	return p.Page < p.TotalPages()
}

// HasPrev checks if there is a previous page.
func (p *Paginate) HasPrev() bool {
	return p.Page > 1
}

// ListResponse is a generic paginated list response.
type ListResponse struct {
	Items      interface{} `json:"items"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
	HasNext    bool        `json:"has_next"`
	HasPrev    bool        `json:"has_prev"`
}

// NewListResponse creates a paginated list response.
func NewListResponse(items interface{}, p Paginate) ListResponse {
	return ListResponse{
		Items:      items,
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      p.Total,
		TotalPages: p.TotalPages(),
		HasNext:    p.HasNext(),
		HasPrev:    p.HasPrev(),
	}
}
