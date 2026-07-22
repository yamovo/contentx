package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"github.com/yamovo/contentx/internal/storage"
	"gorm.io/gorm"
)

// MediaListParams holds query parameters for listing media.
type MediaListParams struct {
	Page     int
	PageSize int
	MimeType string
	Folder   string
	Search   string
	Sort     string
}

// UpdateMediaRequest holds fields for updating media metadata.
type UpdateMediaRequest struct {
	Alt         string `json:"alt"`
	Title       string `json:"title"`
	Caption     string `json:"caption"`
	Description string `json:"description"`
	Folder      string `json:"folder"`
}

// MediaStats holds media library statistics.
type MediaStats struct {
	TotalFiles int64 `json:"total_files"`
	TotalSize  int64 `json:"total_size"`
	Images     int64 `json:"images"`
	Videos     int64 `json:"videos"`
	Documents  int64 `json:"documents"`
}

// MediaService handles media business logic.
type MediaService struct {
	repo    repository.MediaRepository
	cfg     config.UploadConfig
	store   storage.Driver // nil → fallback to legacy local-disk logic
	webhook WebhookDispatcher
}

// NewMediaService creates a new MediaService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewMediaService(db *gorm.DB, cfg config.UploadConfig) *MediaService {
	return &MediaService{repo: repository.NewMediaRepository(db), cfg: cfg}
}

// NewMediaServiceWithRepo builds a MediaService with an explicit repository,
// enabling unit tests to inject mocks.
func NewMediaServiceWithRepo(repo repository.MediaRepository, cfg config.UploadConfig) *MediaService {
	return &MediaService{repo: repo, cfg: cfg}
}

// SetWebhookDispatcher attaches a webhook dispatcher for event triggering.
func (s *MediaService) SetWebhookDispatcher(d WebhookDispatcher) { s.webhook = d }

// SetStorageDriver attaches a storage driver. When set, Upload/Delete/BulkDelete
// delegate file I/O to this driver; when nil, the legacy local-disk path is used.
func (s *MediaService) SetStorageDriver(d storage.Driver) { s.store = d }

// List returns media files with pagination and filters.
func (s *MediaService) List(params MediaListParams) ([]models.Media, int64, error) {
	return s.repo.List(repository.MediaListFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		MimeType: params.MimeType,
		Folder:   params.Folder,
		Search:   params.Search,
		Sort:     params.Sort,
	})
}

// Get returns a single media item by ID.
func (s *MediaService) Get(id uint) (*models.Media, error) {
	return s.repo.GetByID(id)
}

// Upload handles file upload: validates, saves via storage driver (or local disk),
// and creates the DB record.
func (s *MediaService) Upload(file io.Reader, header *multipart.FileHeader, folder, alt, title, caption, description string, uploaderID uint) (*models.Media, error) {
	// Validate file size.
	if header.Size > s.cfg.MaxSize {
		return nil, fmt.Errorf("file too large: max size %d bytes", s.cfg.MaxSize)
	}

	// Detect MIME type.
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	mimeType := http.DetectContentType(buf[:n])

	// http.DetectContentType 无法可靠识别 SVG，按扩展名修正。
	if strings.EqualFold(filepath.Ext(header.Filename), ".svg") {
		mimeType = "image/svg+xml"
	}

	// Validate file type.
	allowed := false
	for _, t := range s.cfg.AllowedTypes {
		if strings.HasPrefix(mimeType, t) || t == mimeType {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("file type not allowed: %s", mimeType)
	}

	// SVG 上传：读取完整内容并净化，剥离脚本、事件处理器与外部引用。
	var sanitizedSVG []byte
	if mimeType == "image/svg+xml" {
		rest, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read SVG content: %w", err)
		}
		full := make([]byte, 0, n+len(rest))
		full = append(full, buf[:n]...)
		full = append(full, rest...)
		sanitizedSVG, err = SanitizeSVG(full)
		if err != nil {
			return nil, fmt.Errorf("SVG rejected: %w", err)
		}
	}

	// Determine and sanitize folder path.
	if folder == "" {
		folder = "/" + time.Now().Format("2006/01")
	}
	folder = filepath.Clean("/" + folder)
	if strings.Contains(folder, "..") {
		return nil, fmt.Errorf("invalid folder path")
	}

	// Generate unique filename.
	ext := filepath.Ext(header.Filename)
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s-%d-%s", header.Filename, time.Now().UnixNano(), header.Filename)
	filename := fmt.Sprintf("%x%s", hash.Sum(nil), ext)

	// Prepare the file content as a single reader. We've already consumed `buf[:n]`
	// from `file`; we need to prepend it for non-SVG uploads.
	var fileContent io.Reader
	var size int64
	var checksumHex string

	if sanitizedSVG != nil {
		fileContent = bytes.NewReader(sanitizedSVG)
		size = int64(len(sanitizedSVG))
		sum := sha256.Sum256(sanitizedSVG)
		checksumHex = fmt.Sprintf("%x", sum)
	} else {
		// Reconstruct full content: buf[:n] + rest of file.
		rest, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file content: %w", err)
		}
		full := make([]byte, 0, n+len(rest))
		full = append(full, buf[:n]...)
		full = append(full, rest...)
		fileContent = bytes.NewReader(full)
		size = int64(len(full))
		sum := sha256.Sum256(full)
		checksumHex = fmt.Sprintf("%x", sum)
	}

	// Build the storage key (folder/filename) and save via the appropriate backend.
	// Normalize the folder to forward slashes so S3 object keys are well-formed
	// on every platform (filepath.Clean uses backslashes on Windows).
	key := strings.TrimPrefix(filepath.ToSlash(folder), "/") + "/" + filename

	var filePath, url string
	if s.store != nil {
		// Storage driver path (S3 / MinIO / etc.)
		url, err = s.store.Upload(context.Background(), key, fileContent, mimeType)
		if err != nil {
			return nil, fmt.Errorf("failed to upload to storage: %w", err)
		}
		filePath = key // for S3, FilePath stores the object key
	} else {
		// Legacy local-disk path.
		uploadDir := filepath.Join(s.cfg.StoragePath, folder)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create upload directory: %w", err)
		}
		filePath = filepath.Join(uploadDir, filename)
		dst, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}
		if _, err := io.Copy(dst, fileContent); err != nil {
			_ = dst.Close()
			_ = os.Remove(filePath)
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		_ = dst.Close()
		url = s.cfg.URLPrefix + "/" + folder + "/" + filename
	}

	media := models.Media{
		Filename:     filename,
		OriginalName: header.Filename,
		FilePath:     filePath,
		URL:          url,
		MimeType:     mimeType,
		FileSize:     size,
		Folder:       folder,
		UploaderID:   uploaderID,
		Checksum:     checksumHex,
		Alt:          alt,
		Title:        title,
		Caption:      caption,
		Description:  description,
	}

	if err := s.repo.Create(&media); err != nil {
		// Best-effort cleanup of the stored file.
		if s.store != nil {
			_ = s.store.Delete(context.Background(), key)
		} else {
			_ = os.Remove(filePath)
		}
		return nil, fmt.Errorf("failed to save media record: %w", err)
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventMediaCreate, &media)
	}

	return &media, nil
}

// Update updates media metadata.
func (s *MediaService) Update(id uint, req UpdateMediaRequest) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return err
	}

	updates := map[string]interface{}{
		"alt":         req.Alt,
		"title":       req.Title,
		"caption":     req.Caption,
		"description": req.Description,
	}
	if req.Folder != "" {
		updates["folder"] = req.Folder
	}

	return s.repo.UpdateFields(id, updates)
}

// Delete removes a media file from storage (or local disk) and the database.
func (s *MediaService) Delete(id uint) error {
	media, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	// Remove the primary file via the configured backend.
	s.removeStoredFile(media.FilePath, media.ThumbnailURL)

	if err := s.repo.Delete(media); err != nil {
		return err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventMediaDelete, media)
	}
	return nil
}

// BulkDelete removes multiple media files by ID. Returns the number of rows affected.
func (s *MediaService) BulkDelete(ids []uint) (int64, error) {
	media, err := s.repo.FindByIDs(ids)
	if err != nil {
		return 0, err
	}

	// Remove files via the configured backend.
	for _, m := range media {
		s.removeStoredFile(m.FilePath, m.ThumbnailURL)
	}

	return s.repo.DeleteByIDs(ids)
}

// removeStoredFile deletes the primary file (and thumbnail, if any) using the
// storage driver when configured, otherwise falls back to local-disk removal.
// For the storage-driver path, FilePath holds the object key. For the legacy
// local path, FilePath holds the absolute filesystem path and ThumbnailURL is
// resolved by replacing the URL prefix with the storage path.
func (s *MediaService) removeStoredFile(filePath, thumbnailURL string) {
	if s.store != nil {
		// Storage driver path: filePath is the object key. Ignore ThumbnailURL
		// because the driver does not manage local thumbnails.
		_ = s.store.Delete(context.Background(), filePath)
		return
	}
	// Legacy local-disk path.
	_ = os.Remove(filePath)
	if thumbnailURL != "" {
		thumbPath := strings.Replace(thumbnailURL, s.cfg.URLPrefix, s.cfg.StoragePath, 1)
		_ = os.Remove(thumbPath)
	}
}

// Folders returns all unique media folders.
func (s *MediaService) Folders() ([]string, error) {
	return s.repo.ListFolders()
}

// Stats returns media library statistics.
func (s *MediaService) Stats() (MediaStats, error) {
	data, err := s.repo.Stats()
	if err != nil {
		return MediaStats{}, err
	}
	return MediaStats{
		TotalFiles: data.TotalFiles,
		TotalSize:  data.TotalSize,
		Images:     data.Images,
		Videos:     data.Videos,
		Documents:  data.Documents,
	}, nil
}
