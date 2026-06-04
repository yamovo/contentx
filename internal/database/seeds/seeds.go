package database

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"

	"github.com/vortexcms/go-cms/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedAll populates the database with default data.
func SeedAll(db *gorm.DB) error {
	if err := seedPermissions(db); err != nil {
		return err
	}
	if err := seedRoles(db); err != nil {
		return err
	}
	if err := seedAdminUser(db); err != nil {
		return err
	}
	if err := seedDefaultSettings(db); err != nil {
		return err
	}
	if err := seedDefaultMenus(db); err != nil {
		return err
	}
	log.Println("[Seed] Database seeding completed")
	return nil
}

func seedPermissions(db *gorm.DB) error {
	permissions := []models.Permission{
		// Articles
		{Name: "View Articles", Slug: "articles.view", Module: "articles"},
		{Name: "Create Articles", Slug: "articles.create", Module: "articles"},
		{Name: "Edit Articles", Slug: "articles.edit", Module: "articles"},
		{Name: "Delete Articles", Slug: "articles.delete", Module: "articles"},
		{Name: "Publish Articles", Slug: "articles.publish", Module: "articles"},
		{Name: "Manage Own Articles", Slug: "articles.manage_own", Module: "articles"},
		// Categories
		{Name: "View Categories", Slug: "categories.view", Module: "categories"},
		{Name: "Manage Categories", Slug: "categories.manage", Module: "categories"},
		// Tags
		{Name: "View Tags", Slug: "tags.view", Module: "tags"},
		{Name: "Manage Tags", Slug: "tags.manage", Module: "tags"},
		// Comments
		{Name: "View Comments", Slug: "comments.view", Module: "comments"},
		{Name: "Moderate Comments", Slug: "comments.moderate", Module: "comments"},
		{Name: "Delete Comments", Slug: "comments.delete", Module: "comments"},
		// Media
		{Name: "Upload Media", Slug: "media.upload", Module: "media"},
		{Name: "View Media", Slug: "media.view", Module: "media"},
		{Name: "Delete Media", Slug: "media.delete", Module: "media"},
		{Name: "Manage Media", Slug: "media.manage", Module: "media"},
		// Users
		{Name: "View Users", Slug: "users.view", Module: "users"},
		{Name: "Create Users", Slug: "users.create", Module: "users"},
		{Name: "Edit Users", Slug: "users.edit", Module: "users"},
		{Name: "Delete Users", Slug: "users.delete", Module: "users"},
		// Roles
		{Name: "View Roles", Slug: "roles.view", Module: "roles"},
		{Name: "Manage Roles", Slug: "roles.manage", Module: "roles"},
		// Settings
		{Name: "View Settings", Slug: "settings.view", Module: "settings"},
		{Name: "Manage Settings", Slug: "settings.manage", Module: "settings"},
		// SEO
		{Name: "Manage SEO", Slug: "seo.manage", Module: "seo"},
		// Themes
		{Name: "Manage Themes", Slug: "themes.manage", Module: "themes"},
		// Plugins
		{Name: "Manage Plugins", Slug: "plugins.manage", Module: "plugins"},
		// Analytics
		{Name: "View Analytics", Slug: "analytics.view", Module: "analytics"},
		// Menus
		{Name: "Manage Menus", Slug: "menus.manage", Module: "menus"},
		// System
		{Name: "View Activity Log", Slug: "system.activity_log", Module: "system"},
		{Name: "Manage Backups", Slug: "system.backups", Module: "system"},
		{Name: "Manage Redirects", Slug: "system.redirects", Module: "system"},
	}

	for _, perm := range permissions {
		err := db.Where("slug = ?", perm.Slug).
			Assign(perm).
			FirstOrCreate(&models.Permission{}).Error
		if err != nil {
			return err
		}
	}
	log.Printf("[Seed] Seeded %d permissions", len(permissions))
	return nil
}

func seedRoles(db *gorm.DB) error {
	// Fetch all permissions.
	var allPerms []models.Permission
	db.Find(&allPerms)

	permMap := make(map[string]models.Permission)
	for _, p := range allPerms {
		permMap[p.Slug] = p
	}

	roles := []struct {
		role        models.Role
		permissions []string
	}{
		{
			role: models.Role{
				Name:      "Administrator",
				Slug:      "admin",
				Description: "Full access to all features",
				IsDefault: false,
				IsSystem:  true,
			},
			permissions: func() []string {
				slugs := make([]string, len(allPerms))
				for i, p := range allPerms {
					slugs[i] = p.Slug
				}
				return slugs
			}(),
		},
		{
			role: models.Role{
				Name:      "Editor",
				Slug:      "editor",
				Description: "Can publish and manage articles and comments",
				IsDefault: false,
				IsSystem:  true,
			},
			permissions: []string{
				"articles.view", "articles.create", "articles.edit",
				"articles.delete", "articles.publish",
				"categories.view", "categories.manage",
				"tags.view", "tags.manage",
				"comments.view", "comments.moderate", "comments.delete",
				"media.upload", "media.view", "media.delete",
				"menus.manage",
			},
		},
		{
			role: models.Role{
				Name:      "Author",
				Slug:      "author",
				Description: "Can write and manage own articles",
				IsDefault: false,
				IsSystem:  true,
			},
			permissions: []string{
				"articles.view", "articles.create", "articles.manage_own",
				"categories.view", "tags.view",
				"media.upload", "media.view",
			},
		},
		{
			role: models.Role{
				Name:      "Subscriber",
				Slug:      "subscriber",
				Description: "Can read and comment",
				IsDefault: true,
				IsSystem:  true,
			},
			permissions: []string{
				"articles.view",
			},
		},
	}

	for _, r := range roles {
		var role models.Role
		err := db.Where("slug = ?", r.role.Slug).FirstOrCreate(&role, r.role).Error
		if err != nil {
			return err
		}
		// Assign permissions.
		var perms []models.Permission
		for _, slug := range r.permissions {
			if p, ok := permMap[slug]; ok {
				perms = append(perms, p)
			}
		}
		if err := db.Model(&role).Association("Permissions").Replace(perms); err != nil {
			return err
		}
	}
	log.Println("[Seed] Seeded roles: admin, editor, author, subscriber")
	return nil
}

func seedAdminUser(db *gorm.DB) error {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		log.Println("[Seed] Admin user already exists, skipping")
		return nil
	}

	var adminRole models.Role
	if err := db.Where("slug = ?", "admin").First(&adminRole).Error; err != nil {
		return err
	}

	// Get admin password from environment or generate random one.
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = generateRandomPassword(16)
		log.Printf("[Seed] =======================================================")
		log.Printf("[Seed] Generated admin password: %s", adminPassword)
		log.Printf("[Seed] IMPORTANT: Save this password! Set ADMIN_PASSWORD env var for custom.")
		log.Printf("[Seed] =======================================================")
	}

	hashedPw, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := models.User{
		Username:    "admin",
		Email:       "admin@vortexcms.local",
		Password:    string(hashedPw),
		DisplayName: "Administrator",
		RoleID:      adminRole.ID,
		Status:      models.UserStatusActive,
		Preferences: models.UserPreferences{
			Language:       "zh",
			Theme:          "auto",
			EmailNotify:    true,
			MarkdownEditor: true,
			ItemsPerPage:   20,
		},
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	log.Println("[Seed] Created admin user: admin")
	return nil
}

// generateRandomPassword creates a cryptographically secure random password.
func generateRandomPassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("[Seed] Warning: Failed to generate random password: %v", err)
		return "ChangeMeNow123!"
	}
	return hex.EncodeToString(bytes)[:length]
}

func seedDefaultSettings(db *gorm.DB) error {
	settings := []models.SiteSetting{
		// General
		{Key: "site_name", Value: "VortexCMS", Type: "string", Group: "general", Label: "站点名称", SortOrder: 1},
		{Key: "site_description", Value: "A modern content management system", Type: "string", Group: "general", Label: "站点描述", SortOrder: 2},
		{Key: "site_url", Value: "http://localhost:8080", Type: "string", Group: "general", Label: "站点 URL", SortOrder: 3},
		{Key: "site_language", Value: "zh", Type: "string", Group: "general", Label: "站点语言", SortOrder: 4},
		{Key: "site_timezone", Value: "Asia/Shanghai", Type: "string", Group: "general", Label: "时区", SortOrder: 5},
		{Key: "site_logo", Value: "", Type: "string", Group: "general", Label: "站点 Logo", SortOrder: 6},
		{Key: "site_favicon", Value: "", Type: "string", Group: "general", Label: "Favicon", SortOrder: 7},
		{Key: "site_footer", Value: "© 2024 VortexCMS. All rights reserved.", Type: "text", Group: "general", Label: "页脚文本", SortOrder: 8},

		// Content
		{Key: "posts_per_page", Value: "10", Type: "int", Group: "content", Label: "每页文章数", SortOrder: 1},
		{Key: "default_category", Value: "uncategorized", Type: "string", Group: "content", Label: "默认分类", SortOrder: 2},
		{Key: "default_post_status", Value: "draft", Type: "string", Group: "content", Label: "默认文章状态", SortOrder: 3},
		{Key: "excerpt_length", Value: "200", Type: "int", Group: "content", Label: "摘要长度", SortOrder: 4},
		{Key: "enable_comments", Value: "true", Type: "bool", Group: "content", Label: "启用评论", SortOrder: 5},
		{Key: "comment_moderation", Value: "true", Type: "bool", Group: "content", Label: "评论审核", SortOrder: 6},
		{Key: "auto_save_interval", Value: "60", Type: "int", Group: "content", Label: "自动保存间隔(秒)", SortOrder: 7},
		{Key: "revision_limit", Value: "20", Type: "int", Group: "content", Label: "版本历史限制", SortOrder: 8},
		{Key: "enable_markdown", Value: "true", Type: "bool", Group: "content", Label: "启用 Markdown", SortOrder: 9},
		{Key: "enable_rich_text", Value: "true", Type: "bool", Group: "content", Label: "启用富文本", SortOrder: 10},

		// Reading
		{Key: "show_author_bio", Value: "true", Type: "bool", Group: "reading", Label: "显示作者简介", SortOrder: 1},
		{Key: "show_post_meta", Value: "true", Type: "bool", Group: "reading", Label: "显示文章 Meta", SortOrder: 2},
		{Key: "show_related_posts", Value: "true", Type: "bool", Group: "reading", Label: "显示相关文章", SortOrder: 3},
		{Key: "related_posts_count", Value: "3", Type: "int", Group: "reading", Label: "相关文章数", SortOrder: 4},

		// SEO
		{Key: "seo_title_separator", Value: " - ", Type: "string", Group: "seo", Label: "标题分隔符", SortOrder: 1},
		{Key: "seo_home_title", Value: "", Type: "string", Group: "seo", Label: "首页标题", SortOrder: 2},
		{Key: "seo_home_description", Value: "", Type: "string", Group: "seo", Label: "首页描述", SortOrder: 3},
		{Key: "seo_home_keywords", Value: "", Type: "string", Group: "seo", Label: "首页关键词", SortOrder: 4},
		{Key: "seo_enable_sitemap", Value: "true", Type: "bool", Group: "seo", Label: "启用 Sitemap", SortOrder: 5},
		{Key: "seo_enable_robots", Value: "true", Type: "bool", Group: "seo", Label: "启用 Robots.txt", SortOrder: 6},
		{Key: "seo_google_analytics", Value: "", Type: "string", Group: "seo", Label: "Google Analytics ID", SortOrder: 7},
		{Key: "seo_baidu_analytics", Value: "", Type: "string", Group: "seo", Label: "百度统计 ID", SortOrder: 8},
		{Key: "seo_enable_canonical", Value: "true", Type: "bool", Group: "seo", Label: "启用 Canonical URL", SortOrder: 9},
		{Key: "seo_enable_og", Value: "true", Type: "bool", Group: "seo", Label: "启用 Open Graph", SortOrder: 10},

		// Social
		{Key: "social_twitter", Value: "", Type: "string", Group: "social", Label: "Twitter", SortOrder: 1},
		{Key: "social_github", Value: "", Type: "string", Group: "social", Label: "GitHub", SortOrder: 2},
		{Key: "social_weibo", Value: "", Type: "string", Group: "social", Label: "微博", SortOrder: 3},
		{Key: "social_wechat", Value: "", Type: "string", Group: "social", Label: "微信公众号", SortOrder: 4},

		// Email
		{Key: "email_from_name", Value: "VortexCMS", Type: "string", Group: "email", Label: "发件人名称", SortOrder: 1},
		{Key: "email_from_address", Value: "noreply@vortexcms.local", Type: "string", Group: "email", Label: "发件人地址", SortOrder: 2},

		// Media
		{Key: "media_max_upload_size", Value: "20", Type: "int", Group: "media", Label: "最大上传大小(MB)", SortOrder: 1},
		{Key: "media_allowed_types", Value: "jpg,jpeg,png,gif,webp,pdf,mp4", Type: "string", Group: "media", Label: "允许的文件类型", SortOrder: 2},
		{Key: "media_image_quality", Value: "85", Type: "int", Group: "media", Label: "图片质量", SortOrder: 3},
		{Key: "media_thumbnail_size", Value: "400", Type: "int", Group: "media", Label: "缩略图尺寸", SortOrder: 4},
		{Key: "media_watermark_enabled", Value: "false", Type: "bool", Group: "media", Label: "启用水印", SortOrder: 5},

		// Cache
		{Key: "cache_enabled", Value: "true", Type: "bool", Group: "cache", Label: "启用缓存", SortOrder: 1},
		{Key: "cache_ttl", Value: "600", Type: "int", Group: "cache", Label: "缓存 TTL(秒)", SortOrder: 2},
	}

	for _, s := range settings {
		err := db.Where("key = ?", s.Key).
			Assign(s).
			FirstOrCreate(&models.SiteSetting{}).Error
		if err != nil {
			return err
		}
	}
	log.Printf("[Seed] Seeded %d site settings", len(settings))
	return nil
}

func seedDefaultMenus(db *gorm.DB) error {
	var count int64
	db.Model(&models.Menu{}).Count(&count)
	if count > 0 {
		return nil
	}

	mainMenu := models.Menu{
		Name:      "主导航",
		Slug:      "main-nav",
		Locations: "header",
		Items: []models.MenuItem{
			{Title: "首页", URL: "/", SortOrder: 1, Target: "_self"},
			{Title: "文章", URL: "/articles", SortOrder: 2, Target: "_self"},
			{Title: "关于", URL: "/about", SortOrder: 3, Target: "_self"},
			{Title: "联系", URL: "/contact", SortOrder: 4, Target: "_self"},
		},
	}

	footerMenu := models.Menu{
		Name:      "页脚导航",
		Slug:      "footer-nav",
		Locations: "footer",
		Items: []models.MenuItem{
			{Title: "隐私政策", URL: "/privacy", SortOrder: 1},
			{Title: "服务条款", URL: "/terms", SortOrder: 2},
			{Title: "站点地图", URL: "/sitemap.xml", SortOrder: 3},
			{Title: "RSS", URL: "/feed", SortOrder: 4},
		},
	}

	if err := db.Create(&mainMenu).Error; err != nil {
		return err
	}
	if err := db.Create(&footerMenu).Error; err != nil {
		return err
	}
	log.Println("[Seed] Seeded default menus")
	return nil
}
