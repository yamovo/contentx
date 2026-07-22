package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// MediaListFilter holds query parameters for listing media.
type MediaListFilter struct {
	Page     int
	PageSize int
	MimeType string
	Folder   string
	Search   string
	Sort     string
}

// MediaStatsData holds raw media statistics returned by the repository.
// The service layer is responsible for mapping these to its public MediaStats DTO.
type MediaStatsData struct {
	TotalFiles int64
	TotalSize  int64
	Images     int64
	Videos     int64
	Documents  int64
}

// MediaRepository defines data-access operations for media files.
type MediaRepository interface {
	List(filter MediaListFilter) ([]models.Media, int64, error)
	GetByID(id uint) (*models.Media, error) // preloads Uploader
	FindByID(id uint) (*models.Media, error)
	FindByIDs(ids []uint) ([]models.Media, error)
	Create(media *models.Media) error
	UpdateFields(id uint, updates map[string]interface{}) error
	Delete(media *models.Media) error
	DeleteByIDs(ids []uint) (rowsAffected int64, err error)
	ListFolders() ([]string, error)
	Stats() (MediaStatsData, error)
}

// gormMediaRepository implements MediaRepository with GORM.
type gormMediaRepository struct {
	db *gorm.DB
}

// NewMediaRepository builds a GORM-backed MediaRepository.
func NewMediaRepository(db *gorm.DB) MediaRepository {
	return &gormMediaRepository{db: db}
}

func (r *gormMediaRepository) List(filter MediaListFilter) ([]models.Media, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	query := r.db.Model(&models.Media{}).Preload("Uploader")

	if filter.MimeType != "" {
		query = query.Where("mime_type LIKE ?", filter.MimeType+"%")
	}
	if filter.Folder != "" {
		query = query.Where("folder = ?", filter.Folder)
	}
	if filter.Search != "" {
		query = query.Where("filename LIKE ? OR original_name LIKE ? OR alt LIKE ?",
			"%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	switch filter.Sort {
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
	if err := query.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&media).Error; err != nil {
		return nil, 0, err
	}

	return media, total, nil
}

func (r *gormMediaRepository) GetByID(id uint) (*models.Media, error) {
	var media models.Media
	if err := r.db.Preload("Uploader").First(&media, id).Error; err != nil {
		return nil, err
	}
	return &media, nil
}

func (r *gormMediaRepository) FindByID(id uint) (*models.Media, error) {
	var media models.Media
	if err := r.db.First(&media, id).Error; err != nil {
		return nil, err
	}
	return &media, nil
}

func (r *gormMediaRepository) FindByIDs(ids []uint) ([]models.Media, error) {
	var media []models.Media
	if err := r.db.Where("id IN ?", ids).Find(&media).Error; err != nil {
		return nil, err
	}
	return media, nil
}

func (r *gormMediaRepository) Create(media *models.Media) error {
	return r.db.Create(media).Error
}

func (r *gormMediaRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.Media{}).Where("id = ?", id).Updates(updates).Error
}

func (r *gormMediaRepository) Delete(media *models.Media) error {
	return r.db.Delete(media).Error
}

func (r *gormMediaRepository) DeleteByIDs(ids []uint) (int64, error) {
	result := r.db.Where("id IN ?", ids).Delete(&models.Media{})
	return result.RowsAffected, result.Error
}

func (r *gormMediaRepository) ListFolders() ([]string, error) {
	var folders []string
	if err := r.db.Model(&models.Media{}).Distinct().Pluck("folder", &folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

// Stats runs the same five COUNT/SUM queries as the original service implementation.
func (r *gormMediaRepository) Stats() (MediaStatsData, error) {
	var stats MediaStatsData

	if err := r.db.Model(&models.Media{}).Count(&stats.TotalFiles).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Media{}).Select("COALESCE(SUM(file_size), 0)").Scan(&stats.TotalSize).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Media{}).Where("mime_type LIKE ?", "image%").Count(&stats.Images).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Media{}).Where("mime_type LIKE ?", "video%").Count(&stats.Videos).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Media{}).Where("mime_type LIKE ?", "application%").Count(&stats.Documents).Error; err != nil {
		return stats, err
	}

	return stats, nil
}
