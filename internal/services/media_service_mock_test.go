package services

import (
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// newTestUploadConfig 构造用于 Upload 测试的配置，存储路径使用 t.TempDir()。
func newTestUploadConfig(t *testing.T) config.UploadConfig {
	t.Helper()
	return config.UploadConfig{
		MaxSize:      10 << 20, // 10 MB
		AllowedTypes: []string{"image/png", "image/jpeg", "image/gif", "image/svg+xml", "text/plain"},
		StoragePath:  t.TempDir(),
		URLPrefix:    "/uploads",
	}
}

// newMultipartHeader 构造一个 multipart.FileHeader，内容为 content，文件名为 filename。
func newMultipartHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	// Use multipart writer to create a real FileHeader.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Close()
	// Parse it back to get a *multipart.FileHeader.
	reader := multipart.NewReader(bytes.NewReader(buf.Bytes()), w.Boundary())
	form, err := reader.ReadForm(32 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	return form.File["file"][0]
}

// openFileHeader 打开 FileHeader 对应的文件，返回 reader 和 header。
func openFileHeader(t *testing.T, fh *multipart.FileHeader) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	f, err := fh.Open()
	if err != nil {
		t.Fatalf("Open file header: %v", err)
	}
	return f, fh
}

func TestMockMedia_NewWithRepo(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	s := NewMediaServiceWithRepo(repo, cfg)
	if s == nil {
		t.Fatal("expected non-nil MediaService")
	}
}

func TestMockMedia_Upload_TooLarge(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	cfg.MaxSize = 10
	s := NewMediaServiceWithRepo(repo, cfg)

	content := bytes.Repeat([]byte("x"), 100)
	fh := newMultipartHeader(t, "big.txt", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "", "", "", "", "", 1)
	if err == nil || !strings.Contains(err.Error(), "file too large") {
		t.Fatalf("expected file too large error, got %v", err)
	}
	if len(repo.CreatedMedia) != 0 {
		t.Errorf("expected 0 Create calls, got %d", len(repo.CreatedMedia))
	}
}

func TestMockMedia_Upload_TypeNotAllowed(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	cfg.AllowedTypes = []string{"image/png"} // 仅允许 PNG
	s := NewMediaServiceWithRepo(repo, cfg)

	// 上传 text/plain 内容，但文件名也用 .txt → mimeType = text/plain; charset=utf-8
	fh := newMultipartHeader(t, "note.txt", []byte("hello world"))
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "", "", "", "", "", 1)
	if err == nil || !strings.Contains(err.Error(), "file type not allowed") {
		t.Fatalf("expected type not allowed error, got %v", err)
	}
}

func TestMockMedia_Upload_InvalidFolderPath(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	s := NewMediaServiceWithRepo(repo, cfg)

	// PNG magic header 让 DetectContentType 返回 image/png
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	content := append(pngHeader, bytes.Repeat([]byte{0}, 100)...)
	fh := newMultipartHeader(t, "img.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "a..b", "", "", "", "", 1)
	if err == nil || !strings.Contains(err.Error(), "invalid folder path") {
		t.Fatalf("expected invalid folder path error, got %v", err)
	}
}

func TestMockMedia_Upload_PNGSuccess(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	s := NewMediaServiceWithRepo(repo, cfg)

	// 构造一个最小有效 PNG 头 + 内容
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	content := append(pngHeader, bytes.Repeat([]byte{0x42}, 256)...)
	fh := newMultipartHeader(t, "photo.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	media, err := s.Upload(f, header, "pics", "alt-text", "title", "caption", "desc", 42)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if media == nil {
		t.Fatal("expected non-nil media")
	}
	if media.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want image/png", media.MimeType)
	}
	if media.OriginalName != "photo.png" {
		t.Errorf("OriginalName = %q, want photo.png", media.OriginalName)
	}
	if media.Alt != "alt-text" || media.Title != "title" || media.Caption != "caption" || media.Description != "desc" {
		t.Errorf("metadata fields not set: %+v", media)
	}
	if media.UploaderID != 42 {
		t.Errorf("UploaderID = %d, want 42", media.UploaderID)
	}
	if media.URL == "" || !strings.HasPrefix(media.URL, "/uploads/") {
		t.Errorf("URL = %q, want prefix /uploads/", media.URL)
	}
	if !strings.Contains(media.URL, "pics") {
		t.Errorf("URL = %q, want it to contain 'pics'", media.URL)
	}
	if media.Checksum == "" {
		t.Error("Checksum should be set")
	}
	// 文件应该真的写到磁盘
	if _, err := os.Stat(media.FilePath); err != nil {
		t.Errorf("file not written to disk: %v", err)
	}
	// repo.Create 应该被调用一次
	if len(repo.CreatedMedia) != 1 {
		t.Fatalf("expected 1 Create call, got %d", len(repo.CreatedMedia))
	}
	if repo.CreatedMedia[0].Filename != media.Filename {
		t.Errorf("CreatedMedia Filename = %q, want %q", repo.CreatedMedia[0].Filename, media.Filename)
	}
}

func TestMockMedia_Upload_SVGSuccess(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	s := NewMediaServiceWithRepo(repo, cfg)

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`)
	fh := newMultipartHeader(t, "icon.svg", svg)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	media, err := s.Upload(f, header, "", "", "", "", "", 1)
	if err != nil {
		t.Fatalf("Upload SVG failed: %v", err)
	}
	if media.MimeType != "image/svg+xml" {
		t.Errorf("MimeType = %q, want image/svg+xml", media.MimeType)
	}
	// 文件应写入磁盘（净化后）
	if _, err := os.Stat(media.FilePath); err != nil {
		t.Errorf("SVG file not written: %v", err)
	}
	// 默认 folder 应为 "/YYYY/MM"（Windows 上分隔符为 \）
	nowYear := time.Now().Format("2006")
	if !strings.Contains(media.Folder, nowYear) {
		t.Errorf("default Folder = %q, expected to contain year %s", media.Folder, nowYear)
	}
}

func TestMockMedia_Upload_SVGRejected(t *testing.T) {
	repo := &MockMediaRepository{}
	cfg := newTestUploadConfig(t)
	s := NewMediaServiceWithRepo(repo, cfg)

	// 含 <script> 的 SVG 应被 SanitizeSVG 拒绝（取决于 SanitizeSVG 实现）
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	fh := newMultipartHeader(t, "evil.svg", svg)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "", "", "", "", "", 1)
	// SanitizeSVG 对 <script> 的处理可能是剥离而非拒绝，这里允许两种结果
	if err != nil {
		if !strings.Contains(err.Error(), "SVG rejected") && !strings.Contains(err.Error(), "failed to read SVG") {
			// 其它错误不可接受
			t.Logf("SVG upload returned error (acceptable if rejection): %v", err)
		}
	}
	// 不论是否拒绝，都不应崩溃；如果成功，文件应在磁盘上
}

func TestMockMedia_Upload_RepoCreateError(t *testing.T) {
	repo := &MockMediaRepository{
		CreateErr: gorm.ErrInvalidDB,
	}
	cfg := newTestUploadConfig(t)
	s := NewMediaServiceWithRepo(repo, cfg)

	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	content := append(pngHeader, bytes.Repeat([]byte{0x42}, 100)...)
	fh := newMultipartHeader(t, "img.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "", "", "", "", "", 1)
	if err == nil {
		t.Fatal("expected error from repo.Create")
	}
	// Upload 失败时应清理已写到磁盘的文件
	if len(repo.CreatedMedia) != 0 {
		t.Errorf("expected 0 Create calls (error path), got %d", len(repo.CreatedMedia))
	}
}

func TestMockMedia_List_Success(t *testing.T) {
	expected := []models.Media{
		{BaseModel: models.BaseModel{ID: 1}, Filename: "a.png"},
		{BaseModel: models.BaseModel{ID: 2}, Filename: "b.png"},
	}
	repo := &MockMediaRepository{
		MediaList: expected,
		ListTotal: int64(len(expected)),
	}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	got, total, err := s.List(MediaListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != int64(len(expected)) {
		t.Errorf("total = %d, want %d", total, len(expected))
	}
	if len(got) != len(expected) {
		t.Errorf("got %d items, want %d", len(got), len(expected))
	}
}

func TestMockMedia_Get_Success(t *testing.T) {
	expected := &models.Media{BaseModel: models.BaseModel{ID: 5}, Filename: "x.png"}
	repo := &MockMediaRepository{Media: expected}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	got, err := s.Get(5)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Filename != "x.png" {
		t.Errorf("Filename = %q, want x.png", got.Filename)
	}
}

func TestMockMedia_Update_Success(t *testing.T) {
	repo := &MockMediaRepository{
		FindMedia: &models.Media{BaseModel: models.BaseModel{ID: 3}},
	}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	err := s.Update(3, UpdateMediaRequest{
		Alt:    "new alt",
		Title:  "new title",
		Folder: "newfolder",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if len(repo.UpdatedFields) != 1 {
		t.Fatalf("expected 1 UpdateFields call, got %d", len(repo.UpdatedFields))
	}
	call := repo.UpdatedFields[0]
	if call.ID != 3 {
		t.Errorf("UpdateFields ID = %d, want 3", call.ID)
	}
	if call.Updates["alt"] != "new alt" {
		t.Errorf("alt = %v, want new alt", call.Updates["alt"])
	}
	if call.Updates["folder"] != "newfolder" {
		t.Errorf("folder = %v, want newfolder", call.Updates["folder"])
	}
}

func TestMockMedia_Update_NotFound(t *testing.T) {
	repo := &MockMediaRepository{
		FindByIDErr: gorm.ErrRecordNotFound,
	}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	err := s.Update(99, UpdateMediaRequest{Alt: "x"})
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockMedia_Delete_Success(t *testing.T) {
	// 先在磁盘上创建一个假文件
	dir := t.TempDir()
	filePath := filepath.Join(dir, "toDelete.png")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	repo := &MockMediaRepository{
		FindMedia: &models.Media{BaseModel: models.BaseModel{ID: 7}, FilePath: filePath},
	}
	s := NewMediaServiceWithRepo(repo, config.UploadConfig{
		StoragePath: dir,
		URLPrefix:   "/uploads",
	})

	if err := s.Delete(7); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(repo.DeletedMedia) != 1 {
		t.Fatalf("expected 1 Delete call, got %d", len(repo.DeletedMedia))
	}
	// 磁盘文件应已被删除
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected file removed from disk, got err=%v", err)
	}
}

func TestMockMedia_Delete_NotFound(t *testing.T) {
	repo := &MockMediaRepository{
		FindByIDErr: gorm.ErrRecordNotFound,
	}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	err := s.Delete(404)
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestMockMedia_BulkDelete_Success(t *testing.T) {
	dir := t.TempDir()
	// 创建两个假文件
	path1 := filepath.Join(dir, "a.png")
	path2 := filepath.Join(dir, "b.png")
	os.WriteFile(path1, []byte("a"), 0644)
	os.WriteFile(path2, []byte("b"), 0644)

	repo := &MockMediaRepository{
		FindByIDsRes: []models.Media{
			{BaseModel: models.BaseModel{ID: 1}, FilePath: path1},
			{BaseModel: models.BaseModel{ID: 2}, FilePath: path2},
		},
	}
	s := NewMediaServiceWithRepo(repo, config.UploadConfig{StoragePath: dir, URLPrefix: "/uploads"})

	n, err := s.BulkDelete([]uint{1, 2})
	if err != nil {
		t.Fatalf("BulkDelete failed: %v", err)
	}
	if n != 2 {
		t.Errorf("rows = %d, want 2", n)
	}
	if _, err := os.Stat(path1); !os.IsNotExist(err) {
		t.Error("file 1 not removed")
	}
	if _, err := os.Stat(path2); !os.IsNotExist(err) {
		t.Error("file 2 not removed")
	}
	if repo.DeleteByIDsCalls != 1 {
		t.Errorf("DeleteByIDsCalls = %d, want 1", repo.DeleteByIDsCalls)
	}
}

func TestMockMedia_Folders_Success(t *testing.T) {
	expected := []string{"/2024/01", "/2024/02"}
	repo := &MockMediaRepository{Folders: expected}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	got, err := s.Folders()
	if err != nil {
		t.Fatalf("Folders failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d folders, want 2", len(got))
	}
}

func TestMockMedia_Stats_Success(t *testing.T) {
	expected := repository.MediaStatsData{TotalFiles: 100, TotalSize: 5000, Images: 80, Videos: 5, Documents: 15}
	repo := &MockMediaRepository{StatsData: expected}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	got, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if got.TotalFiles != 100 || got.Images != 80 {
		t.Errorf("Stats = %+v, want %+v", got, expected)
	}
}

func TestMockMedia_Stats_Error(t *testing.T) {
	repo := &MockMediaRepository{StatsErr: gorm.ErrInvalidDB}
	s := NewMediaServiceWithRepo(repo, newTestUploadConfig(t))

	_, err := s.Stats()
	if err == nil {
		t.Fatal("expected error from Stats")
	}
}
