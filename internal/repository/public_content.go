package repository

import (
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// PublicContentRepository is the dedicated published-only read surface for the
// RFC-002 public content delivery. It deliberately shares nothing with the
// administrative ContentRepository: every query hard-codes status=published,
// a non-null published_at that is not in the future, and the caller-provided
// tenant, so a public route can never accidentally expose draft or internal
// state through a reused management query.
type PublicContentRepository interface {
	// ListPublishedByTypeUID returns a page of published entries for the
	// content type identified by uid, newest first, plus the total count.
	ListPublishedByTypeUID(uid string, tenantID uint, page, pageSize int, locale string) ([]models.ContentEntry, int64, error)
	// GetPublishedByDocumentID returns one published entry by its public
	// document ID (UUID). Returns gorm.ErrRecordNotFound when the entry, the
	// type, or the published state is missing.
	GetPublishedByDocumentID(uid, documentID string, tenantID uint) (*models.ContentEntry, error)
}

type gormPublicContentRepository struct {
	db *gorm.DB
}

// NewPublicContentRepository builds a GORM-backed PublicContentRepository.
func NewPublicContentRepository(db *gorm.DB) PublicContentRepository {
	return &gormPublicContentRepository{db: db}
}

// publishedScope applies the published-only contract shared by every query:
// type UID (joined on content_types), tenant scope, published status, and a
// published_at that exists and is not in the future.
func publishedScope(db *gorm.DB, uid string, tenantID uint) *gorm.DB {
	return db.Model(&models.ContentEntry{}).
		Joins("JOIN content_types ON content_types.id = content_entries.content_type_id").
		Where("content_types.uid = ?", uid).
		Where("content_entries.tenant_id = ?", tenantID).
		Where("content_entries.status = ?", models.EntryStatusPublished).
		Where("content_entries.published_at IS NOT NULL AND content_entries.published_at <= ?", time.Now())
}

func (r *gormPublicContentRepository) ListPublishedByTypeUID(uid string, tenantID uint, page, pageSize int, locale string) ([]models.ContentEntry, int64, error) {
	base := publishedScope(r.db, uid, tenantID)
	if locale != "" {
		base = base.Where("content_entries.locale = ?", locale)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var entries []models.ContentEntry
	if err := base.
		Select("content_entries.*").
		Order("content_entries.published_at DESC, content_entries.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *gormPublicContentRepository) GetPublishedByDocumentID(uid, documentID string, tenantID uint) (*models.ContentEntry, error) {
	var entry models.ContentEntry
	if err := publishedScope(r.db, uid, tenantID).
		Select("content_entries.*").
		Where("content_entries.document_id = ?", documentID).
		First(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}
