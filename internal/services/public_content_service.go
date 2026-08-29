package services

import (
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
)

// PublicContentPageSizeMax is the hard upper bound for the public list page
// size (RFC-002 §2: only capped page/page_size/locale are accepted).
const PublicContentPageSizeMax = 50

// PublicContentService is the dedicated read-only service behind the RFC-002
// public content delivery routes. It validates public pagination bounds and
// delegates to the published-only repository; it intentionally has no write
// paths and no administrative status filters to reuse.
type PublicContentService struct {
	repo repository.PublicContentRepository
}

// NewPublicContentService builds a PublicContentService.
func NewPublicContentService(repo repository.PublicContentRepository) *PublicContentService {
	return &PublicContentService{repo: repo}
}

// PublicContentListParams holds the caller-supplied list filters.
type PublicContentListParams struct {
	Page     int
	PageSize int
	Locale   string
}

// normalize clamps defaults and rejects out-of-range pagination so illegal
// requests fail with 400 instead of silently degrading.
func (p *PublicContentListParams) normalize() error {
	if p.Page <= 0 {
		if p.Page == 0 {
			p.Page = 1
		} else {
			return errs.ErrBadRequest.WithMessage("page must be a positive integer")
		}
	}
	if p.PageSize <= 0 {
		if p.PageSize == 0 {
			p.PageSize = 20
		} else {
			return errs.ErrBadRequest.WithMessage("page_size must be a positive integer")
		}
	}
	if p.PageSize > PublicContentPageSizeMax {
		return errs.ErrBadRequest.WithMessage("page_size must not exceed 50")
	}
	return nil
}

// List returns a page of published entries for the given content type UID
// within the default tenant.
func (s *PublicContentService) List(uid string, params PublicContentListParams) ([]models.ContentEntry, int64, error) {
	if err := params.normalize(); err != nil {
		return nil, 0, err
	}
	return s.repo.ListPublishedByTypeUID(uid, models.DefaultTenantID, params.Page, params.PageSize, params.Locale)
}

// Get returns one published entry by public document ID within the default
// tenant. Missing types, missing entries, and unpublished entries are
// indistinguishable to the caller (the handler maps them all to 404).
func (s *PublicContentService) Get(uid, documentID string) (*models.ContentEntry, error) {
	return s.repo.GetPublishedByDocumentID(uid, documentID, models.DefaultTenantID)
}
