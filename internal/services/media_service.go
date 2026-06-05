package services

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vortexcms/go-cms/internal/config"
	"github.com/vortexcms/go-cms/internal/models"
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
	db  *gorm.DB
	cfg config.UploadConfig
}

// NewMediaService creates a new MediaService.
func NewMediaService(db *gorm.DB, cfg config.UploadConfig) *MediaService {
	return &MediaService{db: db, cfg: cfg}
}

// List returns media files with pagination and filters.
func (s *MediaService) List(params MediaListParams) ([]models.Media, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	query := s.db.Model(&models.Media{}).Preload("Uploader")

	if params.MimeType != "" {
		query = query.Where("mime_type LIKE ?", params.MimeType+"%")
	}
	if params.Folder != "" {
		query = query.Where("folder = ?", params.Folder)
	}
	if params.Search != "" {
		query = query.Where("filename LIKE ? OR original_name LIKE ? OR alt LIKE ?",
			"%"+params.Search+"%", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	switch params.Sort {
	case "oldest":
		query = query.Order("created_at ASC")
	case "name":
		query = query.Order("filename ASC")
	case "size":
		query = query.Order("file_size DESC")
	default:
		query = query.Order("created_at DESC")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var media []models.Media
	if err := query.Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).Find(&media).Error; err != nil {
		return nil, 0, err
	}

	return media, total, nil
}

// Get returns a single media item by ID.
func (s *MediaService) Get(id uint) (*models.Media, error) {
	var media models.Media
	if err := s.db.Preload("Uploader").First(&media, id).Error; err != nil {
		return nil, err
	}
	return &media, nil
}

// Upload handles file upload: validates, saves to disk, and creates the DB record.
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

	// Determine and sanitize folder path.
	if folder == "" {
		folder = "/" + time.Now().Format("2006/01")
	}
	folder = filepath.Clean("/" + folder)
	if strings.Contains(folder, "..") {
		return nil, fmt.Errorf("invalid folder path")
	}

	// Create directory.
	uploadDir := filepath.Join(s.cfg.StoragePath, folder)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate unique filename.
	ext := filepath.Ext(header.Filename)
	hash := sha256.New()
	fmt.Fprintf(hash, "%s-%d-%s", header.Filename, time.Now().UnixNano(), header.Filename)
	filename := fmt.Sprintf("%x%s", hash.Sum(nil), ext)
	filePath := filepath.Join(uploadDir, filename)

	// Save file to disk.
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Calculate checksum by re-reading the saved file.
	savedFile, err := os.Open(filePath)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to read saved file for checksum: %w", err)
	}
	checksum := sha256.New()
	io.Copy(checksum, savedFile)
	savedFile.Close()

	url := s.cfg.URLPrefix + "/" + folder + "/" + filename

	media := models.Media{
		Filename:     filename,
		OriginalName: header.Filename,
		FilePath:     filePath,
		URL:          url,
		MimeType:     mimeType,
		FileSize:     size,
		Folder:       folder,
		UploaderID:   uploaderID,
		Checksum:     fmt.Sprintf("%x", checksum.Sum(nil)),
		Alt:          alt,
		Title:        title,
		Caption:      caption,
		Description:  description,
	}

	if err := s.db.Create(&media).Error; err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save media record: %w", err)
	}

	return &media, nil
}

// Update updates media metadata.
func (s *MediaService) Update(id uint, req UpdateMediaRequest) error {
	var media models.Media
	if err := s.db.First(&media, id).Error; err != nil {
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

	return s.db.Model(&media).Updates(updates).Error
}

// Delete removes a media file from disk and the database.
func (s *MediaService) Delete(id uint) error {
	var media models.Media
	if err := s.db.First(&media, id).Error; err != nil {
		return err
	}

	// Remove file from disk.
	os.Remove(media.FilePath)
	if media.ThumbnailURL != "" {
		thumbPath := strings.Replace(media.ThumbnailURL, s.cfg.URLPrefix, s.cfg.StoragePath, 1)
		os.Remove(thumbPath)
	}

	return s.db.Delete(&media).Error
}

// BulkDelete removes multiple media files by ID. Returns the number of rows affected.
func (s *MediaService) BulkDelete(ids []uint) (int64, error) {
	var media []models.Media
	if err := s.db.Where("id IN ?", ids).Find(&media).Error; err != nil {
		return 0, err
	}

	// Remove files from disk.
	for _, m := range media {
		os.Remove(m.FilePath)
		if m.ThumbnailURL != "" {
			thumbPath := strings.Replace(m.ThumbnailURL, s.cfg.URLPrefix, s.cfg.StoragePath, 1)
			os.Remove(thumbPath)
		}
	}

	result := s.db.Where("id IN ?", ids).Delete(&models.Media{})
	return result.RowsAffected, result.Error
}

// Folders returns all unique media folders.
func (s *MediaService) Folders() ([]string, error) {
	var folders []string
	if err := s.db.Model(&models.Media{}).Distinct().Pluck("folder", &folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

// Stats returns media library statistics.
func (s *MediaService) Stats() (MediaStats, error) {
	var stats MediaStats

	if err := s.db.Model(&models.Media{}).Count(&stats.TotalFiles).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Media{}).Select("COALESCE(SUM(file_size), 0)").Scan(&stats.TotalSize).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Media{}).Where("mime_type LIKE ?", "image%").Count(&stats.Images).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Media{}).Where("mime_type LIKE ?", "video%").Count(&stats.Videos).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Media{}).Where("mime_type LIKE ?", "application%").Count(&stats.Documents).Error; err != nil {
		return stats, err
	}

	return stats, nil
}
