package auth_test

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

func TestArticleCalcReadingTime(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantMin int
	}{
		{"short article", "<p>Hello world</p>", 1},
		{"medium article", generateContent(500), 2},
		{"long article", generateContent(3000), 10},
		{"empty content", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article := models.Article{Content: tt.content}
			article.CalcReadingTime()
			if tt.wantMin > 0 && article.ReadingTime < tt.wantMin {
				t.Errorf("ReadingTime = %d, want >= %d", article.ReadingTime, tt.wantMin)
			}
		})
	}
}

func TestArticleMakeExcerpt(t *testing.T) {
	article := models.Article{
		Content: "<p>This is a test article with some content that should be truncated at a reasonable length for the excerpt.</p>",
	}
	article.MakeExcerpt(50)
	if article.Excerpt == "" {
		t.Error("Excerpt should not be empty")
	}
	if len([]rune(article.Excerpt)) > 52 { // 50 + "..."
		t.Errorf("Excerpt too long: %d chars", len([]rune(article.Excerpt)))
	}
}

func TestArticleIsPublished(t *testing.T) {
	tests := []struct {
		status models.ArticleStatus
		want   bool
	}{
		{models.StatusDraft, false},
		{models.StatusPublished, true},
		{models.StatusPending, false},
		{models.StatusTrash, false},
	}

	for _, tt := range tests {
		article := models.Article{Status: tt.status}
		if got := article.IsPublished(); got != tt.want {
			t.Errorf("IsPublished(%s) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestCommentIsApproved(t *testing.T) {
	comment := models.Comment{Status: "pending"}
	if comment.IsApproved() {
		t.Error("Pending comment should not be approved")
	}
	comment.Approve()
	if !comment.IsApproved() {
		t.Error("Comment should be approved after Approve()")
	}
}

func TestMediaIsImage(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"video/mp4", false},
		{"application/pdf", false},
	}
	for _, tt := range tests {
		m := models.Media{MimeType: tt.mime}
		if got := m.IsImage(); got != tt.want {
			t.Errorf("IsImage(%s) = %v, want %v", tt.mime, got, tt.want)
		}
	}
}

func TestMediaFileSizeFormatted(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500.0 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		m := models.Media{FileSize: tt.bytes}
		if got := m.FileSizeFormatted(); got != tt.want {
			t.Errorf("FileSizeFormatted(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestPagination(t *testing.T) {
	p := models.Paginate{Page: 2, PageSize: 10, Total: 55}
	if p.Offset() != 10 {
		t.Errorf("Offset() = %d, want 10", p.Offset())
	}
	if p.TotalPages() != 6 {
		t.Errorf("TotalPages() = %d, want 6", p.TotalPages())
	}
	if !p.HasNext() {
		t.Error("HasNext() should be true for page 2 of 6")
	}
	if !p.HasPrev() {
		t.Error("HasPrev() should be true for page 2")
	}

	p.Page = 6
	if p.HasNext() {
		t.Error("HasNext() should be false on last page")
	}

	p.Page = 1
	if p.HasPrev() {
		t.Error("HasPrev() should be false on first page")
	}
}

func generateContent(wordCount int) string {
	s := ""
	for i := 0; i < wordCount; i++ {
		s += "测试 "
	}
	return "<p>" + s + "</p>"
}
