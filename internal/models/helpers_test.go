package models

import (
	"strings"
	"testing"
	"time"
)

// ---------- User helpers ----------

func TestUser_IsActive(t *testing.T) {
	u := &User{Status: UserStatusActive}
	if !u.IsActive() {
		t.Fatal("active user should be active")
	}
	u.Status = "banned"
	if u.IsActive() {
		t.Fatal("banned user should not be active")
	}
}

func TestUser_Roles(t *testing.T) {
	admin := &User{Role: Role{Slug: "admin"}}
	if !admin.IsAdmin() || !admin.IsEditor() {
		t.Fatal("admin should be admin and editor")
	}
	editor := &User{Role: Role{Slug: "editor"}}
	if editor.IsAdmin() || !editor.IsEditor() {
		t.Fatal("editor should not be admin but should be editor")
	}
	author := &User{Role: Role{Slug: "author"}}
	if author.IsAdmin() || author.IsEditor() {
		t.Fatal("author should be neither admin nor editor")
	}
}

func TestUser_AvatarURL(t *testing.T) {
	u := &User{Avatar: "https://cdn.example.com/a.png"}
	if u.AvatarURL() != "https://cdn.example.com/a.png" {
		t.Fatal("should return custom avatar")
	}
	u2 := &User{DisplayName: "John Doe"}
	got := u2.AvatarURL()
	if !strings.Contains(got, "ui-avatars.com") || !strings.Contains(got, "John+Doe") {
		t.Fatalf("unexpected default avatar URL: %q", got)
	}
}

func TestUser_RecordLogin(t *testing.T) {
	u := &User{}
	u.RecordLogin("1.2.3.4")
	if u.LastLoginAt == nil || u.LastLoginIP != "1.2.3.4" || u.LoginCount != 1 {
		t.Fatalf("login not recorded: %+v", u)
	}
	u.RecordLogin("5.6.7.8")
	if u.LoginCount != 2 || u.LastLoginIP != "5.6.7.8" {
		t.Fatal("second login not recorded correctly")
	}
}

// ---------- Article helpers ----------

func TestArticle_GenerateSlug(t *testing.T) {
	a := &Article{Title: "Hello World 2024"}
	a.GenerateSlug()
	if a.Slug == "" || strings.Contains(a.Slug, " ") {
		t.Fatalf("bad slug: %q", a.Slug)
	}
	// Existing slug is preserved.
	a2 := &Article{Title: "X", Slug: "custom-slug"}
	a2.GenerateSlug()
	if a2.Slug != "custom-slug" {
		t.Fatalf("existing slug overwritten: %q", a2.Slug)
	}
}

func TestArticle_CalcReadingTime(t *testing.T) {
	a := &Article{Content: "<p>" + strings.Repeat("word ", 400) + "</p>"}
	a.CalcReadingTime()
	if a.WordCount == 0 || a.ReadingTime < 1 {
		t.Fatalf("reading time not calculated: words=%d mins=%d", a.WordCount, a.ReadingTime)
	}
	if strings.Contains(stripHTMLRe.ReplaceAllString(a.Content, ""), "<p>") {
		t.Fatal("HTML should be stripped before counting")
	}
	// Short content → minimum 1 minute.
	short := &Article{Content: "hello"}
	short.CalcReadingTime()
	if short.ReadingTime != 1 {
		t.Fatalf("expected min 1 min, got %d", short.ReadingTime)
	}
}

func TestArticle_MakeExcerpt(t *testing.T) {
	a := &Article{Content: "<p>Hello <b>world</b>, this is a test.</p>"}
	a.MakeExcerpt(10)
	if strings.Contains(a.Excerpt, "<") {
		t.Fatalf("excerpt contains HTML: %q", a.Excerpt)
	}
	if !strings.HasSuffix(a.Excerpt, "…") {
		t.Fatalf("long content excerpt should end with ellipsis: %q", a.Excerpt)
	}
	// Existing excerpt preserved.
	a2 := &Article{Excerpt: "preset", Content: "something"}
	a2.MakeExcerpt(5)
	if a2.Excerpt != "preset" {
		t.Fatal("existing excerpt overwritten")
	}
	// Short content: no ellipsis.
	a3 := &Article{Content: "short"}
	a3.MakeExcerpt(100)
	if a3.Excerpt != "short" {
		t.Fatalf("expected 'short', got %q", a3.Excerpt)
	}
}

func TestArticle_StatusHelpers(t *testing.T) {
	a := &Article{Visibility: VisibilityPublic}
	if a.IsPublished() || a.IsVisible() {
		t.Fatal("draft should not be published/visible")
	}
	a.Publish()
	if !a.IsPublished() || !a.IsVisible() || a.PublishedAt == nil {
		t.Fatal("publish failed")
	}
	firstPublish := a.PublishedAt
	a.Publish() // idempotent: keeps original PublishedAt
	if a.PublishedAt != firstPublish {
		t.Fatal("re-publish should keep original PublishedAt")
	}
	a.Trash()
	if a.IsPublished() {
		t.Fatal("trashed article should not be published")
	}
}

// ---------- Category helpers ----------

func TestCategory_FullPath(t *testing.T) {
	root := &Category{Name: "Tech"}
	child := &Category{Name: "Go", Parent: root}
	grand := &Category{Name: "Gin", Parent: child}
	if grand.FullPath() != "Tech / Go / Gin" {
		t.Fatalf("unexpected path: %q", grand.FullPath())
	}
	if root.FullPath() != "Tech" {
		t.Fatalf("root path wrong: %q", root.FullPath())
	}
}

// ---------- Comment helpers ----------

func TestComment_StatusHelpers(t *testing.T) {
	c := &Comment{Status: "pending"}
	if !c.IsPending() || c.IsApproved() {
		t.Fatal("pending state wrong")
	}
	c.Approve()
	if !c.IsApproved() || c.IsPending() {
		t.Fatal("approve failed")
	}
	c.Spam()
	if c.Status != "spam" {
		t.Fatal("spam failed")
	}
}

func TestComment_AuthorDisplayName(t *testing.T) {
	c := &Comment{User: &User{DisplayName: "Alice", Username: "alice"}}
	if c.AuthorDisplayName() != "Alice" {
		t.Fatal("should prefer display name")
	}
	c = &Comment{User: &User{Username: "bob"}}
	if c.AuthorDisplayName() != "bob" {
		t.Fatal("should fall back to username")
	}
	c = &Comment{AuthorName: "Guest"}
	if c.AuthorDisplayName() != "Guest" {
		t.Fatal("should fall back to author name")
	}
	c = &Comment{}
	if c.AuthorDisplayName() != "Anonymous" {
		t.Fatal("should default to Anonymous")
	}
}

// ---------- Media helpers ----------

func TestMedia_TypeChecks(t *testing.T) {
	img := &Media{MimeType: "image/png"}
	if !img.IsImage() || img.IsVideo() || img.IsAudio() {
		t.Fatal("image detection wrong")
	}
	vid := &Media{MimeType: "video/mp4"}
	if !vid.IsVideo() || vid.IsImage() {
		t.Fatal("video detection wrong")
	}
	aud := &Media{MimeType: "audio/mpeg"}
	if !aud.IsAudio() {
		t.Fatal("audio detection wrong")
	}
}

func TestMedia_FileSizeFormatted(t *testing.T) {
	cases := []struct {
		size int64
		want string
	}{
		{500, "500.0 B"},
		{2048, "2.0 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, tc := range cases {
		m := &Media{FileSize: tc.size}
		if got := m.FileSizeFormatted(); got != tc.want {
			t.Fatalf("size %d: want %q got %q", tc.size, tc.want, got)
		}
	}
}

func TestMedia_Extension(t *testing.T) {
	if (&Media{Filename: "Photo.PNG"}).Extension() != "png" {
		t.Fatal("extension should be lowercased")
	}
	if (&Media{Filename: "archive.tar.gz"}).Extension() != "gz" {
		t.Fatal("should return last extension")
	}
	if (&Media{Filename: "noext"}).Extension() != "" {
		t.Fatal("no extension should be empty")
	}
}

// ---------- Pagination ----------

func TestPaginate_Offset(t *testing.T) {
	p := &Paginate{Page: 3, PageSize: 10}
	if p.Offset() != 20 {
		t.Fatalf("expected offset 20, got %d", p.Offset())
	}
	p2 := &Paginate{Page: 0, PageSize: 10}
	if p2.Offset() != 0 || p2.Page != 1 {
		t.Fatal("page 0 should be normalized to 1")
	}
}

func TestPaginate_TotalPages(t *testing.T) {
	p := &Paginate{PageSize: 10, Total: 25}
	if p.TotalPages() != 3 {
		t.Fatalf("expected 3 pages, got %d", p.TotalPages())
	}
	zero := &Paginate{PageSize: 0, Total: 10}
	if zero.TotalPages() != 0 {
		t.Fatal("zero page size should yield 0 pages")
	}
}

func TestPaginate_Navigation(t *testing.T) {
	p := &Paginate{Page: 2, PageSize: 10, Total: 30}
	if !p.HasPrev() || !p.HasNext() {
		t.Fatal("middle page should have both prev and next")
	}
	first := &Paginate{Page: 1, PageSize: 10, Total: 30}
	if first.HasPrev() || !first.HasNext() {
		t.Fatal("first page should only have next")
	}
	last := &Paginate{Page: 3, PageSize: 10, Total: 30}
	if last.HasNext() || !last.HasPrev() {
		t.Fatal("last page should only have prev")
	}
}

func TestNewListResponse(t *testing.T) {
	p := Paginate{Page: 1, PageSize: 2, Total: 5}
	resp := NewListResponse([]string{"a", "b"}, p)
	if resp.TotalPages != 3 || !resp.HasNext || resp.HasPrev || resp.Total != 5 {
		t.Fatalf("bad list response: %+v", resp)
	}
}

// ---------- StringSlice ----------

func TestStringSlice_Has(t *testing.T) {
	s := StringSlice{"a", "b"}
	if !s.Has("a") || s.Has("z") {
		t.Fatal("Has() wrong")
	}
}

func TestStringSlice_Value(t *testing.T) {
	var nilSlice StringSlice
	v, err := nilSlice.Value()
	if err != nil || v != "[]" {
		t.Fatalf("nil slice should serialize to [], got %v", v)
	}
	s := StringSlice{"x", "y"}
	v2, err := s.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	str, ok := v2.(string)
	if !ok || !strings.Contains(str, "x") || !strings.Contains(str, "y") {
		t.Fatalf("unexpected value: %v", v2)
	}
}

func TestStringSlice_Scan(t *testing.T) {
	var s StringSlice
	if err := s.Scan(`["a","b"]`); err != nil {
		t.Fatalf("Scan string: %v", err)
	}
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Fatalf("scan result wrong: %v", s)
	}
	var s2 StringSlice
	if err := s2.Scan([]byte(`["c"]`)); err != nil {
		t.Fatalf("Scan bytes: %v", err)
	}
	if len(s2) != 1 || s2[0] != "c" {
		t.Fatalf("scan bytes wrong: %v", s2)
	}
	var s3 StringSlice
	if err := s3.Scan(nil); err != nil {
		t.Fatalf("Scan nil should not error: %v", err)
	}
	if err := s3.Scan(123); err == nil {
		t.Fatal("Scan with unsupported type should error")
	}
}

// Ensure time import is used (Publish uses time internally).
var _ = time.Now
