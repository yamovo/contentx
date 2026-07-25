package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// TestMCPResources verifies that content types appear as concrete resources
// (resources/list), articles as a template (resources/templates/list), and that
// resources/read resolves correctly for both — respecting published-only.
func TestMCPResources(t *testing.T) {
	// Setup in-memory DB.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Seed(db)

	// Author.
	var role models.Role
	db.Where("slug = ?", "admin").First(&role)
	author := models.User{
		Username: "res-author", Email: "res@test.com", Password: "x",
		DisplayName: "Res Author", RoleID: role.ID, Status: models.UserStatusActive,
	}
	db.Create(&author)

	// Published + draft articles.
	now := time.Now()
	pub := models.Article{
		Title: "Res Published", Slug: "res-pub", Content: "resource body alpha",
		AuthorID: author.ID, Status: models.StatusPublished, PublishedAt: &now,
	}
	draft := models.Article{
		Title: "Res Draft", Slug: "res-draft", Content: "secret draft",
		AuthorID: author.ID, Status: models.StatusDraft,
	}
	db.Create(&pub)
	db.Create(&draft)

	// Content type with 2 fields (must exist before NewServer → registerResources).
	ct := models.ContentType{UID: "event", Name: "Event", Description: "Events collection"}
	db.Create(&ct)
	db.Create(&models.ContentField{ContentTypeID: ct.ID, Name: "title", Label: "Title", FieldType: "text", Required: true, SortOrder: 0})
	db.Create(&models.ContentField{ContentTypeID: ct.ID, Name: "date", Label: "Date", FieldType: "date", Required: false, SortOrder: 1})

	// Services.
	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	if _, err := articleSvc.ReindexAll(context.Background()); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	ctSvc := services.NewContentTypeService(db)

	// Build MCP server (registerResources queries content types → registers "event").
	deps := Deps{
		Article: articleSvc, ContentType: ctSvc,
		BaseURL: "http://localhost:8080", IncludeDrafts: false,
	}
	srv := NewServer(deps, "test")

	// In-memory client ↔ server.
	clientT, serverT := mcpsdk.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = ss.Close() }()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// 1) resources/list contains the content type resource.
	listRes, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}
	found := false
	for _, r := range listRes.Resources {
		if r.URI == "contentx://content-types/event" {
			found = true
		}
	}
	if !found {
		t.Error("expected content type 'event' in resources/list")
	}

	// 2) resources/templates/list contains the article template.
	listTmpl, err := cs.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	foundTmpl := false
	for _, tmpl := range listTmpl.ResourceTemplates {
		if tmpl.URITemplate == "contentx://articles/{id}" {
			foundTmpl = true
		}
	}
	if !foundTmpl {
		t.Error("expected article template in resources/templates/list")
	}

	// 3) Read content type resource.
	ctRes, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: "contentx://content-types/event"})
	if err != nil {
		t.Fatalf("read content type: %v", err)
	}
	if len(ctRes.Contents) == 0 {
		t.Fatal("empty content type response")
	}
	var ctInfo contentTypeInfo
	if err := json.Unmarshal([]byte(ctRes.Contents[0].Text), &ctInfo); err != nil {
		t.Fatalf("unmarshal content type: %v", err)
	}
	if ctInfo.UID != "event" {
		t.Errorf("uid = %q, want event", ctInfo.UID)
	}
	if len(ctInfo.Fields) != 2 {
		t.Errorf("fields count = %d, want 2", len(ctInfo.Fields))
	}

	// 4) Read published article resource.
	artRes, err := cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: fmt.Sprintf("contentx://articles/%d", pub.ID)})
	if err != nil {
		t.Fatalf("read article: %v", err)
	}
	if len(artRes.Contents) == 0 {
		t.Fatal("empty article response")
	}
	var art articleDetail
	if err := json.Unmarshal([]byte(artRes.Contents[0].Text), &art); err != nil {
		t.Fatalf("unmarshal article: %v", err)
	}
	if art.Content != "resource body alpha" {
		t.Errorf("content = %q, want the published body", art.Content)
	}

	// 5) Reading a draft article should fail (published-only).
	_, err = cs.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: fmt.Sprintf("contentx://articles/%d", draft.ID)})
	if err == nil {
		t.Error("expected error reading a draft article resource")
	}
}
