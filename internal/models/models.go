package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// BaseModel contains common fields for all models.
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// User represents a system user.
type User struct {
	BaseModel
	Username     string         `gorm:"uniqueIndex;size:64;not null" json:"username" validate:"required,min=3,max=64"`
	Email        string         `gorm:"uniqueIndex;size:255;not null" json:"email" validate:"required,email"`
	Password     string         `gorm:"size:255;not null" json:"-"`
	DisplayName  string         `gorm:"size:128" json:"display_name"`
	Avatar       string         `gorm:"size:512" json:"avatar"`
	Bio          string         `gorm:"size:1000" json:"bio"`
	Website      string         `gorm:"size:512" json:"website"`
	Role         Role           `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	RoleID       uint           `gorm:"index;not null;default:1" json:"role_id"`
	Status       UserStatus     `gorm:"size:20;not null;default:'active'" json:"status"`
	LastLoginAt  *time.Time     `json:"last_login_at"`
	LastLoginIP  string         `gorm:"size:45" json:"last_login_ip"`
	LoginCount   int            `gorm:"default:0" json:"login_count"`
	Preferences  UserPreferences `gorm:"type:json" json:"preferences"`
	Articles     []Article      `gorm:"foreignKey:AuthorID" json:"articles,omitempty"`
	Comments     []Comment      `gorm:"foreignKey:UserID" json:"comments,omitempty"`
}

// UserStatus represents the status of a user account.
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusBanned   UserStatus = "banned"
	UserStatusPending  UserStatus = "pending"
)

// UserPreferences stores user-specific settings.
type UserPreferences struct {
	Language         string `json:"language"`
	Theme            string `json:"theme"`
	EmailNotify      bool   `json:"email_notify"`
	MarkdownEditor   bool   `json:"markdown_editor"`
	ItemsPerPage     int    `json:"items_per_page"`
	DefaultPostStatus string `json:"default_post_status"`
}

// Value implements the driver.Valuer interface for database storage.
func (p UserPreferences) Value() (driver.Value, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements the sql.Scanner interface for database retrieval.
func (p *UserPreferences) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("failed to scan UserPreferences: %v", value)
	}
	return json.Unmarshal(bytes, p)
}

// Role represents a user role for RBAC.
type Role struct {
	BaseModel
	Name        string       `gorm:"uniqueIndex;size:64;not null" json:"name" validate:"required"`
	Slug        string       `gorm:"uniqueIndex;size:64;not null" json:"slug"`
	Description string       `gorm:"size:255" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	IsDefault   bool         `gorm:"default:false" json:"is_default"`
	IsSystem    bool         `gorm:"default:false" json:"is_system"`
	UserCount   int          `gorm:"-" json:"user_count,omitempty"`
}

// Permission represents a system permission.
type Permission struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:128;not null" json:"name"`
	Slug        string `gorm:"uniqueIndex;size:128;not null" json:"slug"`
	Module      string `gorm:"size:64;not null;index" json:"module"`
	Description string `gorm:"size:255" json:"description"`
}

// Article represents a blog post or page.
type Article struct {
	BaseModel
	Title         string        `gorm:"size:512;not null;index" json:"title" validate:"required,max=512"`
	Slug          string        `gorm:"uniqueIndex;size:512;not null" json:"slug"`
	Content       string        `gorm:"type:longtext" json:"content"`
	Excerpt       string        `gorm:"type:text" json:"excerpt"`
	Author        User          `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	AuthorID      uint          `gorm:"index;not null" json:"author_id"`
	Category      *Category     `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	CategoryID    *uint         `gorm:"index" json:"category_id"`
	Tags          []Tag         `gorm:"many2many:article_tags;" json:"tags,omitempty"`
	FeaturedImage string        `gorm:"size:512" json:"featured_image"`
	Status        ArticleStatus `gorm:"size:20;not null;default:'draft';index" json:"status"`
	PostType      PostType      `gorm:"size:20;not null;default:'post';index" json:"post_type"`
	Format        string        `gorm:"size:20;default:'standard'" json:"format"`
	Visibility    Visibility    `gorm:"size:20;not null;default:'public'" json:"visibility"`
	Password      string        `gorm:"size:255" json:"-"`
	IsPinned      bool          `gorm:"default:false;index" json:"is_pinned"`
	IsFeatured    bool          `gorm:"default:false;index" json:"is_featured"`
	AllowComment  bool          `gorm:"default:true" json:"allow_comment"`
	ViewCount     int64         `gorm:"default:0;index" json:"view_count"`
	LikeCount     int64         `gorm:"default:0" json:"like_count"`
	WordCount     int           `gorm:"default:0" json:"word_count"`
	ReadingTime   int           `gorm:"default:0" json:"reading_time"` // minutes
	PublishedAt   *time.Time    `gorm:"index" json:"published_at"`
	ScheduledAt   *time.Time    `gorm:"index" json:"scheduled_at"`
	MetaTitle     string        `gorm:"size:255" json:"meta_title"`
	MetaDesc      string        `gorm:"size:512" json:"meta_desc"`
	MetaKeywords  string        `gorm:"size:512" json:"meta_keywords"`
	CanonicalURL  string        `gorm:"size:512" json:"canonical_url"`
	RobotsIndex   bool          `gorm:"default:true" json:"robots_index"`
	RobotsFollow  bool          `gorm:"default:true" json:"robots_follow"`
	OGImage       string        `gorm:"size:512" json:"og_image"`
	Template      string        `gorm:"size:128" json:"template"`
	SortOrder     int           `gorm:"default:0" json:"sort_order"`
	Version       int           `gorm:"default:1" json:"version"`
	CommentCount  int           `gorm:"default:0" json:"comment_count"`
	Comments      []Comment     `gorm:"foreignKey:ArticleID" json:"comments,omitempty"`
	Revisions     []Revision    `gorm:"foreignKey:ArticleID" json:"revisions,omitempty"`
	CustomFields  []CustomField `gorm:"foreignKey:ArticleID" json:"custom_fields,omitempty"`
}

// ArticleStatus represents the publication status.
type ArticleStatus string

const (
	StatusDraft     ArticleStatus = "draft"
	StatusPublished ArticleStatus = "published"
	StatusPending   ArticleStatus = "pending"
	StatusScheduled ArticleStatus = "scheduled"
	StatusTrash     ArticleStatus = "trash"
	StatusArchived  ArticleStatus = "archived"
)

// PostType distinguishes posts from pages.
type PostType string

const (
	PostTypePost PostType = "post"
	PostTypePage PostType = "page"
)

// Visibility controls who can see an article.
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityPrivate  Visibility = "private"
	VisibilityPassword Visibility = "password"
)

// Category represents an article category.
type Category struct {
	BaseModel
	Name        string     `gorm:"size:128;not null;uniqueIndex" json:"name" validate:"required,max=128"`
	Slug        string     `gorm:"uniqueIndex;size:128;not null" json:"slug"`
	Description string     `gorm:"type:text" json:"description"`
	Parent      *Category  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	ParentID    *uint      `gorm:"index" json:"parent_id"`
	Children    []Category `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Image       string     `gorm:"size:512" json:"image"`
	Color       string     `gorm:"size:7" json:"color"`
	SortOrder   int        `gorm:"default:0;index" json:"sort_order"`
	PostCount   int        `gorm:"default:0" json:"post_count"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	MetaTitle   string     `gorm:"size:255" json:"meta_title"`
	MetaDesc    string     `gorm:"size:512" json:"meta_desc"`
	Articles    []Article  `gorm:"foreignKey:CategoryID" json:"articles,omitempty"`
}

// Tag represents an article tag.
type Tag struct {
	BaseModel
	Name      string    `gorm:"size:64;not null;uniqueIndex" json:"name" validate:"required,max=64"`
	Slug      string    `gorm:"uniqueIndex;size:64;not null" json:"slug"`
	Count     int       `gorm:"default:0" json:"count"`
	Color     string    `gorm:"size:7" json:"color"`
	Articles  []Article `gorm:"many2many:article_tags;" json:"articles,omitempty"`
}

// Comment represents a user comment on an article.
type Comment struct {
	BaseModel
	Article    Article    `gorm:"foreignKey:ArticleID" json:"article,omitempty"`
	ArticleID  uint       `gorm:"index;not null" json:"article_id"`
	User       *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	UserID     *uint      `gorm:"index" json:"user_id"`
	Parent     *Comment   `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	ParentID   *uint      `gorm:"index" json:"parent_id"`
	Children   []Comment  `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	AuthorName string     `gorm:"size:100" json:"author_name"`
	AuthorEmail string    `gorm:"size:255" json:"author_email"`
	AuthorURL  string     `gorm:"size:512" json:"author_url"`
	AuthorIP   string     `gorm:"size:45" json:"author_ip"`
	Content    string     `gorm:"type:text;not null" json:"content" validate:"required"`
	Status     string     `gorm:"size:20;not null;default:'pending';index" json:"status"`
	Agent      string     `gorm:"size:512" json:"agent"`
	Depth      int        `gorm:"default:0" json:"depth"`
	LikeCount  int        `gorm:"default:0" json:"like_count"`
	IsSticky   bool       `gorm:"default:false" json:"is_sticky"`
}

// Media represents an uploaded file.
type Media struct {
	BaseModel
	Filename     string `gorm:"size:255;not null" json:"filename"`
	OriginalName string `gorm:"size:255;not null" json:"original_name"`
	FilePath     string `gorm:"size:512;not null" json:"file_path"`
	URL          string `gorm:"size:512;not null" json:"url"`
	ThumbnailURL string `gorm:"size:512" json:"thumbnail_url"`
	MimeType     string `gorm:"size:128;not null;index" json:"mime_type"`
	FileSize     int64  `gorm:"not null" json:"file_size"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Duration     int    `json:"duration,omitempty"` // seconds for video/audio
	Alt          string `gorm:"size:512" json:"alt"`
	Title        string `gorm:"size:255" json:"title"`
	Caption      string `gorm:"type:text" json:"caption"`
	Description  string `gorm:"type:text" json:"description"`
	Folder       string `gorm:"size:255;index;default:'/'" json:"folder"`
	Uploader     User   `gorm:"foreignKey:UploaderID" json:"uploader,omitempty"`
	UploaderID   uint   `gorm:"index;not null" json:"uploader_id"`
	Checksum     string `gorm:"size:64;index" json:"checksum"`
	IsPublic     bool   `gorm:"default:true" json:"is_public"`
	Downloads    int64  `gorm:"default:0" json:"downloads"`
	Meta         map[string]interface{} `gorm:"type:json" json:"meta,omitempty"`
}

// Revision stores article version history.
type Revision struct {
	BaseModel
	ArticleID uint   `gorm:"index;not null" json:"article_id"`
	Title     string `gorm:"size:512" json:"title"`
	Content   string `gorm:"type:longtext" json:"content"`
	Excerpt   string `gorm:"type:text" json:"excerpt"`
	Editor    User   `gorm:"foreignKey:EditorID" json:"editor,omitempty"`
	EditorID  uint   `gorm:"index;not null" json:"editor_id"`
	Version   int    `gorm:"not null" json:"version"`
	Note      string `gorm:"size:512" json:"note"`
}

// CustomField stores key-value metadata for articles.
type CustomField struct {
	BaseModel
	ArticleID uint   `gorm:"index;not null" json:"article_id"`
	Key       string `gorm:"size:255;not null;index" json:"key"`
	Value     string `gorm:"type:longtext" json:"value"`
}

// Menu represents a navigation menu.
type Menu struct {
	BaseModel
	Name      string  `gorm:"size:128;not null" json:"name"`
	Slug      string  `gorm:"uniqueIndex;size:128;not null" json:"slug"`
	Locations string  `gorm:"size:255" json:"locations"` // comma-separated: header,footer,sidebar
	Items     []MenuItem `gorm:"foreignKey:MenuID" json:"items,omitempty"`
}

// MenuItem represents a single item in a menu.
type MenuItem struct {
	BaseModel
	MenuID    uint       `gorm:"index;not null" json:"menu_id"`
	Parent    *MenuItem  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	ParentID  *uint      `gorm:"index" json:"parent_id"`
	Children  []MenuItem `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Title     string     `gorm:"size:255;not null" json:"title"`
	URL       string     `gorm:"size:512" json:"url"`
	Target    string     `gorm:"size:20;default:'_self'" json:"target"`
	CSSClass  string     `gorm:"size:255" json:"css_class"`
	Icon      string     `gorm:"size:128" json:"icon"`
	SortOrder int        `gorm:"default:0;index" json:"sort_order"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	// Optional link types
	ArticleID *uint `gorm:"index" json:"article_id"`
	CategoryID *uint `gorm:"index" json:"category_id"`
}

// SiteSetting stores key-value site configuration.
type SiteSetting struct {
	ID        uint   `gorm:"primarykey" json:"id"`
	Key       string `gorm:"uniqueIndex;size:128;not null" json:"key"`
	Value     string `gorm:"type:longtext" json:"value"`
	Type      string `gorm:"size:20;default:'string'" json:"type"` // string, int, bool, json, text
	Group     string `gorm:"size:64;not null;index" json:"group"`
	Label     string `gorm:"size:255" json:"label"`
	HelpText  string `gorm:"size:512" json:"help_text"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`
	IsPublic  bool   `gorm:"default:false" json:"is_public"`
}

// SEOSetting stores per-page SEO overrides.
type SEOSetting struct {
	BaseModel
	EntityType string `gorm:"size:50;not null;index;uniqueIndex:idx_seo_entity" json:"entity_type"` // article, category, tag, page
	EntityID   uint   `gorm:"not null;index;uniqueIndex:idx_seo_entity" json:"entity_id"`
	Title      string `gorm:"size:255" json:"title"`
	Desc       string `gorm:"size:512" json:"desc"`
	Keywords   string `gorm:"size:512" json:"keywords"`
	Canonical  string `gorm:"size:512" json:"canonical"`
	OGImage    string `gorm:"size:512" json:"og_image"`
	OGType     string `gorm:"size:50" json:"og_type"`
	Robots     string `gorm:"size:64" json:"robots"`
	Extra      map[string]string `gorm:"type:json" json:"extra"`
}

// RedirectRule manages URL redirects (301/302).
type RedirectRule struct {
	BaseModel
	FromPath   string `gorm:"size:512;not null;uniqueIndex" json:"from_path"`
	ToPath     string `gorm:"size:512;not null" json:"to_path"`
	StatusCode int    `gorm:"default:301" json:"status_code"`
	IsActive   bool   `gorm:"default:true;index" json:"is_active"`
	HitCount   int64  `gorm:"default:0" json:"hit_count"`
	Note       string `gorm:"size:255" json:"note"`
}

// Plugin represents an installed plugin.
type Plugin struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:128;not null" json:"name"`
	Slug        string `gorm:"uniqueIndex;size:128;not null" json:"slug"`
	Description string `gorm:"type:text" json:"description"`
	Author      string `gorm:"size:128" json:"author"`
	Version     string `gorm:"size:32" json:"version"`
	Website     string `gorm:"size:512" json:"website"`
	Config      map[string]interface{} `gorm:"type:json" json:"config"`
	IsEnabled   bool   `gorm:"default:false;index" json:"is_enabled"`
	EntryPoint  string `gorm:"size:255" json:"entry_point"`
	Dependencies string `gorm:"size:512" json:"dependencies"` // comma-separated
	MinVersion  string `gorm:"size:32" json:"min_version"`   // minimum CMS version
}

// ThemeConfig stores theme settings.
type ThemeConfig struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:128;not null" json:"name"`
	Slug        string `gorm:"uniqueIndex;size:128;not null" json:"slug"`
	Description string `gorm:"type:text" json:"description"`
	Author      string `gorm:"size:128" json:"author"`
	Version     string `gorm:"size:32" json:"version"`
	Screenshot  string `gorm:"size:512" json:"screenshot"`
	IsActive    bool   `gorm:"default:false;index" json:"is_active"`
	Config      map[string]interface{} `gorm:"type:json" json:"config"`
	TemplateDir string `gorm:"size:255" json:"template_dir"`
}

// PageView stores analytics data.
type PageView struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	ArticleID *uint     `gorm:"index" json:"article_id"`
	Path      string    `gorm:"size:512;not null;index" json:"path"`
	Referrer  string    `gorm:"size:1024" json:"referrer"`
	UserAgent string    `gorm:"size:512" json:"user_agent"`
	IP        string    `gorm:"size:45;index" json:"ip"`
	Country   string    `gorm:"size:4" json:"country"`
	City      string    `gorm:"size:128" json:"city"`
	Device    string    `gorm:"size:32" json:"device"` // desktop, mobile, tablet
	Browser   string    `gorm:"size:64" json:"browser"`
	OS        string    `gorm:"size:64" json:"os"`
	SessionID string    `gorm:"size:64;index" json:"session_id"`
	Duration  int       `gorm:"default:0" json:"duration"` // seconds on page
}

// SitemapEntry represents a sitemap URL entry.
type SitemapEntry struct {
	BaseModel
	EntityType  string     `gorm:"size:50;not null;index" json:"entity_type"`
	EntityID    uint       `gorm:"not null;index" json:"entity_id"`
	Loc         string     `gorm:"size:512;not null" json:"loc"`
	LastMod     *time.Time `json:"last_mod"`
	ChangeFreq  string     `gorm:"size:16;default:'weekly'" json:"change_freq"`
	Priority    float64    `gorm:"default:0.5" json:"priority"`
	IsExcluded  bool       `gorm:"default:false" json:"is_excluded"`
}

// Notification represents a system notification.
type Notification struct {
	BaseModel
	UserID    uint             `gorm:"index;not null" json:"user_id"`
	Type      string           `gorm:"size:50;not null;index" json:"type"` // comment, mention, system
	Title     string           `gorm:"size:255;not null" json:"title"`
	Body      string           `gorm:"type:text" json:"body"`
	ActionURL string           `gorm:"size:512" json:"action_url"`
	IsRead    bool             `gorm:"default:false;index" json:"is_read"`
	ReadAt    *time.Time       `json:"read_at"`
	Extra     map[string]string `gorm:"type:json" json:"extra"`
}

// ActivityLog records audit trail.
type ActivityLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UserID    *uint     `gorm:"index" json:"user_id"`
	Action    string    `gorm:"size:50;not null;index" json:"action"` // create, update, delete, login, etc.
	Entity    string    `gorm:"size:50;not null;index" json:"entity"` // article, user, comment, etc.
	EntityID  uint      `gorm:"index" json:"entity_id"`
	Details   string    `gorm:"type:text" json:"details"`
	IP        string    `gorm:"size:45" json:"ip"`
	UserAgent string    `gorm:"size:512" json:"user_agent"`
}

// StringSlice is a []string that marshals to JSON for database storage.
type StringSlice []string

// Value implements driver.Valuer.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("failed to scan StringSlice: %v", value)
	}
	return json.Unmarshal(bytes, s)
}

// APIToken represents a long-lived API token for external access.
type APIToken struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	Name        string     `gorm:"size:128;not null" json:"name"`
	Token       string     `gorm:"size:255;uniqueIndex;not null" json:"-"`
	Permissions StringSlice `gorm:"type:text" json:"permissions"`
	IsActive    bool       `gorm:"default:true;index" json:"is_active"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	UseCount    int64      `gorm:"default:0" json:"use_count"`
	CreatedByID uint       `gorm:"index" json:"created_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
}
