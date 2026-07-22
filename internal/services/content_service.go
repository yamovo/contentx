package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// ─── Content Type Service ───────────────────────────────────────────────────

// ContentTypeService manages content types and their dynamic entries.
type ContentTypeService struct {
	typeRepo  repository.ContentTypeRepository
	entryRepo repository.ContentEntryRepository
	cache     cache.Driver
	cacheTTL  time.Duration
}

// NewContentTypeService creates a new ContentTypeService backed by GORM repositories.
// Kept for backward compatibility with existing callers and tests.
func NewContentTypeService(db *gorm.DB) *ContentTypeService {
	return &ContentTypeService{
		typeRepo:  repository.NewContentTypeRepository(db),
		entryRepo: repository.NewContentEntryRepository(db),
	}
}

// NewContentTypeServiceWithRepo builds a ContentTypeService with explicit
// repositories, enabling unit tests to inject mocks.
func NewContentTypeServiceWithRepo(typeRepo repository.ContentTypeRepository, entryRepo repository.ContentEntryRepository) *ContentTypeService {
	return &ContentTypeService{
		typeRepo:  typeRepo,
		entryRepo: entryRepo,
	}
}

// WithCache attaches an optional cache used to memoize content type lookups.
// When ttl <= 0 a default of 10 minutes is used. Returns the service for chaining.
func (s *ContentTypeService) WithCache(c cache.Driver, ttl time.Duration) *ContentTypeService {
	s.cache = c
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	s.cacheTTL = ttl
	return s
}

// invalidateType removes a cached content type definition.
func (s *ContentTypeService) invalidateType(uid string) {
	if s.cache != nil {
		_ = s.cache.Delete(context.Background(), "contenttype:"+uid)
	}
}

// ─── Content Type CRUD ──────────────────────────────────────────────────────

// CreateContentTypeRequest is the payload for creating a content type.
type CreateContentTypeRequest struct {
	UID          string               `json:"uid" binding:"required,max=64"`
	Name         string               `json:"name" binding:"required,max=128"`
	Description  string               `json:"description"`
	IsSingle     bool                 `json:"is_single"`
	DraftPublish bool                 `json:"draft_publish"`
	Fields       []CreateFieldRequest `json:"fields" binding:"required,min=1"`
}

// CreateFieldRequest defines a field during content type creation.
type CreateFieldRequest struct {
	Name         string   `json:"name" binding:"required,max=64"`
	Label        string   `json:"label" binding:"required,max=128"`
	FieldType    string   `json:"field_type" binding:"required"`
	Required     bool     `json:"required"`
	Unique       bool     `json:"unique"`
	DefaultValue string   `json:"default_value"`
	Options      []string `json:"options"` // for enum
	RelationType string   `json:"relation_type"`
	RelationUID  string   `json:"relation_uid"`
	MinLength    *int     `json:"min_length"`
	MaxLength    *int     `json:"max_length"`
	MinValue     *float64 `json:"min_value"`
	MaxValue     *float64 `json:"max_value"`
}

// CreateContentType creates a new content type with fields.
func (s *ContentTypeService) CreateContentType(req CreateContentTypeRequest) (*models.ContentType, error) {
	// Validate UID format (lowercase, underscores only).
	if !isValidUID(req.UID) {
		return nil, errs.ErrValidation.WithMessage("uid must be lowercase letters, numbers, and underscores only")
	}

	// Check uniqueness.
	count, err := s.typeRepo.CountByUID(req.UID)
	if err != nil {
		return nil, errs.New("CREATE_TYPE_FAILED", "failed to create content type", http.StatusInternalServerError)
	}
	if count > 0 {
		return nil, errs.ErrConflict.WithMessage("content type uid already exists")
	}

	// Validate field types.
	for _, f := range req.Fields {
		if !models.ValidFieldTypes[f.FieldType] {
			return nil, fmt.Errorf("invalid field type: %s", f.FieldType)
		}
		if f.FieldType == models.FieldTypeEnum && len(f.Options) == 0 {
			return nil, fmt.Errorf("enum field %s must have options", f.Name)
		}
	}

	ct := models.ContentType{
		UID:          req.UID,
		Name:         req.Name,
		Description:  req.Description,
		IsSingle:     req.IsSingle,
		DraftPublish: req.DraftPublish,
	}

	for i, f := range req.Fields {
		ct.Fields = append(ct.Fields, models.ContentField{
			Name:         f.Name,
			Label:        f.Label,
			FieldType:    f.FieldType,
			Required:     f.Required,
			Unique:       f.Unique,
			DefaultValue: f.DefaultValue,
			Options:      f.Options,
			RelationType: f.RelationType,
			RelationUID:  f.RelationUID,
			MinLength:    f.MinLength,
			MaxLength:    f.MaxLength,
			MinValue:     f.MinValue,
			MaxValue:     f.MaxValue,
			SortOrder:    i,
		})
	}

	if err := s.typeRepo.Create(&ct); err != nil {
		return nil, errs.New("CREATE_TYPE_FAILED", "failed to create content type", http.StatusInternalServerError)
	}

	return &ct, nil
}

// ListContentTypes returns all content types with entry counts.
func (s *ContentTypeService) ListContentTypes() ([]models.ContentType, error) {
	types, err := s.typeRepo.List()
	if err != nil {
		return nil, err
	}

	// Fill entry counts.
	for i := range types {
		count, err := s.typeRepo.CountEntriesByTypeID(types[i].ID)
		if err != nil {
			return nil, err
		}
		types[i].EntryCount = count
	}

	return types, nil
}

// GetContentType returns a single content type by UID.
func (s *ContentTypeService) GetContentType(uid string) (*models.ContentType, error) {
	cacheKey := "contenttype:" + uid
	if s.cache != nil {
		if data, err := s.cache.Get(context.Background(), cacheKey); err == nil {
			var cached models.ContentType
			if json.Unmarshal(data, &cached) == nil {
				return &cached, nil
			}
		}
	}

	ct, err := s.typeRepo.FindByUID(uid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("content type not found")
		}
		return nil, err
	}

	if s.cache != nil {
		if data, err := json.Marshal(ct); err == nil {
			_ = s.cache.Set(context.Background(), cacheKey, data, s.cacheTTL)
		}
	}
	return ct, nil
}

// DeleteContentType deletes a content type and all its entries.
func (s *ContentTypeService) DeleteContentType(uid string) error {
	ct, err := s.typeRepo.FindByUID(uid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errs.ErrNotFound.WithMessage("content type not found")
		}
		return err
	}

	if err := s.typeRepo.Delete(ct.ID); err != nil {
		return err
	}
	s.invalidateType(uid)
	return nil
}

// ─── Content Entry CRUD ─────────────────────────────────────────────────────

// CreateEntryRequest is the payload for creating an entry.
type CreateEntryRequest struct {
	Data   map[string]interface{} `json:"data" binding:"required"`
	Status string                 `json:"status"` // draft (default) or published
	Locale string                 `json:"locale"` // i18n: BCP-47 tag, defaults to "en"
}

// UpdateEntryRequest is the payload for updating an entry.
type UpdateEntryRequest struct {
	Data   map[string]interface{} `json:"data"`
	Status *string                `json:"status"`
}

// ListEntriesParams holds query parameters for listing entries.
type ListEntriesParams struct {
	Page     int
	PageSize int
	Status   string
	Search   string
	Sort     string
	Filters  map[string]string // field_name=value
	Locale   string            // i18n: filter by locale (exact match)
}

// ListEntries returns entries of a content type.
func (s *ContentTypeService) ListEntries(uid string, params ListEntriesParams) (interface{}, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}

	// Pagination defaults.
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	filter := repository.ContentEntryListFilter{
		TypeID:   ct.ID,
		Page:     params.Page,
		PageSize: params.PageSize,
		Status:   params.Status,
		Search:   params.Search,
		Sort:     params.Sort,
		Filters:  params.Filters,
		Locale:   params.Locale,
	}

	entries, total, err := s.entryRepo.List(filter)
	if err != nil {
		return nil, err
	}

	return models.NewListResponse(entries, models.Paginate{
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    total,
	}), nil
}

// GetEntry returns a single entry by document_id.
func (s *ContentTypeService) GetEntry(uid string, documentID string) (*models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}

	entry, err := s.entryRepo.FindByDocumentID(ct.ID, documentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("entry not found")
		}
		return nil, err
	}

	return entry, nil
}

// CreateEntry creates a new entry for a content type.
func (s *ContentTypeService) CreateEntry(uid string, req CreateEntryRequest, userID uint) (*models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}

	// Validate required fields.
	if err := s.validateEntryData(ct, req.Data); err != nil {
		return nil, err
	}

	status := req.Status
	if status == "" {
		if ct.DraftPublish {
			status = models.EntryStatusDraft
		} else {
			status = models.EntryStatusPublished
		}
	}

	entry := models.ContentEntry{
		ContentTypeID: ct.ID,
		DocumentID:    uuid.New().String(),
		Status:        status,
		Data:          req.Data,
		CreatedByID:   userID,
		UpdatedByID:   userID,
	}
	if req.Locale != "" {
		entry.Locale = req.Locale
	} else {
		entry.Locale = "en"
	}

	if status == models.EntryStatusPublished {
		now := time.Now()
		entry.PublishedAt = &now
	}

	if err := s.entryRepo.Create(&entry); err != nil {
		return nil, errs.New("CREATE_ENTRY_FAILED", "failed to create entry", http.StatusInternalServerError)
	}

	return &entry, nil
}

// UpdateEntry updates an existing entry.
func (s *ContentTypeService) UpdateEntry(uid string, documentID string, req UpdateEntryRequest, userID uint) (*models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}

	entry, err := s.entryRepo.FindByDocumentID(ct.ID, documentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("entry not found")
		}
		return nil, err
	}

	// Merge data.
	if req.Data != nil {
		if err := s.validateEntryData(ct, req.Data); err != nil {
			return nil, err
		}
		// Merge with existing data.
		for k, v := range req.Data {
			entry.Data[k] = v
		}
	}

	// Update status.
	if req.Status != nil {
		entry.Status = *req.Status
		if *req.Status == models.EntryStatusPublished && entry.PublishedAt == nil {
			now := time.Now()
			entry.PublishedAt = &now
		}
	}

	entry.UpdatedByID = userID

	if err := s.entryRepo.Save(entry); err != nil {
		return nil, errs.New("UPDATE_ENTRY_FAILED", "failed to update entry", http.StatusInternalServerError)
	}

	return entry, nil
}

// DeleteEntry deletes an entry by document_id.
func (s *ContentTypeService) DeleteEntry(uid string, documentID string) error {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return err
	}

	rowsAffected, err := s.entryRepo.DeleteByDocumentID(ct.ID, documentID)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errs.ErrNotFound.WithMessage("entry not found")
	}
	return nil
}

// PublishEntry publishes a draft entry.
func (s *ContentTypeService) PublishEntry(uid string, documentID string, userID uint) (*models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}

	entry, err := s.entryRepo.FindByDocumentID(ct.ID, documentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("entry not found")
		}
		return nil, err
	}

	now := time.Now()
	entry.Status = models.EntryStatusPublished
	entry.PublishedAt = &now
	entry.UpdatedByID = userID

	if err := s.entryRepo.Save(entry); err != nil {
		return nil, errs.New("PUBLISH_ENTRY_FAILED", "failed to publish entry", http.StatusInternalServerError)
	}

	return entry, nil
}

// UnpublishEntry reverts a published entry to draft.
func (s *ContentTypeService) UnpublishEntry(uid string, documentID string, userID uint) (*models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}

	entry, err := s.entryRepo.FindByDocumentID(ct.ID, documentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("entry not found")
		}
		return nil, err
	}

	entry.Status = models.EntryStatusDraft
	entry.UpdatedByID = userID

	if err := s.entryRepo.Save(entry); err != nil {
		return nil, errs.New("UNPUBLISH_ENTRY_FAILED", "failed to unpublish entry", http.StatusInternalServerError)
	}

	return entry, nil
}

// ─── Validation ─────────────────────────────────────────────────────────────

func (s *ContentTypeService) validateEntryData(ct *models.ContentType, data map[string]interface{}) error {
	for _, field := range ct.Fields {
		value, exists := data[field.Name]

		if field.Required && (!exists || value == nil || value == "") {
			return fmt.Errorf("field %s is required", field.Name)
		}

		if !exists || value == nil {
			continue
		}

		// Type validation.
		switch field.FieldType {
		case models.FieldTypeInteger:
			switch v := value.(type) {
			case float64:
				if field.MinValue != nil && v < *field.MinValue {
					return fmt.Errorf("field %s: value must be >= %v", field.Name, *field.MinValue)
				}
				if field.MaxValue != nil && v > *field.MaxValue {
					return fmt.Errorf("field %s: value must be <= %v", field.Name, *field.MaxValue)
				}
			default:
				// Try to convert.
				if _, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64); err != nil {
					return fmt.Errorf("field %s: must be a number", field.Name)
				}
			}

		case models.FieldTypeFloat:
			if _, ok := value.(float64); !ok {
				if _, err := strconv.ParseFloat(fmt.Sprintf("%v", value), 64); err != nil {
					return fmt.Errorf("field %s: must be a number", field.Name)
				}
			}

		case models.FieldTypeBoolean:
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("field %s: must be a boolean", field.Name)
			}

		case models.FieldTypeEnum:
			strVal := fmt.Sprintf("%v", value)
			found := false
			for _, opt := range field.Options {
				if opt == strVal {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("field %s: invalid enum value %s, allowed: %v", field.Name, strVal, field.Options)
			}

		case models.FieldTypeText, models.FieldTypeRichText:
			strVal := fmt.Sprintf("%v", value)
			if field.MinLength != nil && len(strVal) < *field.MinLength {
				return fmt.Errorf("field %s: minimum length is %d", field.Name, *field.MinLength)
			}
			if field.MaxLength != nil && len(strVal) > *field.MaxLength {
				return fmt.Errorf("field %s: maximum length is %d", field.Name, *field.MaxLength)
			}

		case models.FieldTypeJSON:
			// Validate it's valid JSON by re-marshaling.
			b, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("field %s: invalid JSON", field.Name)
			}
			var check interface{}
			if err := json.Unmarshal(b, &check); err != nil {
				return fmt.Errorf("field %s: invalid JSON", field.Name)
			}
		}
	}
	return nil
}

func isValidUID(uid string) bool {
	if len(uid) == 0 || len(uid) > 64 {
		return false
	}
	for _, c := range uid {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// GetEntriesByUID returns entries for a content type by UID (for relation loading).
func (s *ContentTypeService) GetEntriesByUID(uid string, ids []uint) ([]models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}
	return s.entryRepo.FindByIDs(ct.ID, ids)
}

// SearchEntries searches across all text fields of a content type.
func (s *ContentTypeService) SearchEntries(uid string, query string, limit int) ([]models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	return s.entryRepo.Search(ct.ID, query, limit)
}

// ExportEntries exports all entries of a content type as JSON.
func (s *ContentTypeService) ExportEntries(uid string) (string, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return "", err
	}

	entries, err := s.entryRepo.ExportAll(ct.ID)
	if err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// ImportEntries imports entries from JSON.
func (s *ContentTypeService) ImportEntries(uid string, data string, userID uint) (int, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return 0, err
	}

	var entries []models.ContentEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		return 0, errs.ErrBadRequest.WithMessage("invalid JSON data")
	}

	for i := range entries {
		entries[i].ID = 0 // reset ID
		entries[i].ContentTypeID = ct.ID
		entries[i].DocumentID = uuid.New().String()
		entries[i].CreatedByID = userID
		entries[i].UpdatedByID = userID
	}

	return s.entryRepo.CreateMany(entries)
}

// ─── i18n: entry translation helpers ────────────────────────────────────────

// effectiveEntryGroupID returns the translation group id for an entry. When
// the entry was created without an explicit group, its own ID is the root.
func effectiveEntryGroupID(e *models.ContentEntry) uint {
	if e.TranslationGroupID != nil {
		return *e.TranslationGroupID
	}
	return e.ID
}

// ListEntryTranslations returns sibling translations of the given entry
// (excluding the entry itself).
func (s *ContentTypeService) ListEntryTranslations(uid, documentID string) ([]models.ContentEntry, error) {
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}
	entry, err := s.entryRepo.FindByDocumentID(ct.ID, documentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("entry not found")
		}
		return nil, err
	}
	return s.entryRepo.ListTranslations(ct.ID, effectiveEntryGroupID(entry), entry.ID)
}

// CreateEntryTranslation creates a new entry as a translation of an existing
// one. The new entry inherits the source's data (which the caller may override
// via req.Data) and translation group, with the requested locale.
func (s *ContentTypeService) CreateEntryTranslation(uid, documentID, locale string, req CreateEntryRequest, userID uint) (*models.ContentEntry, error) {
	if locale == "" {
		return nil, errs.ErrBadRequest.WithMessage("locale is required for translation")
	}
	ct, err := s.GetContentType(uid)
	if err != nil {
		return nil, err
	}
	source, err := s.entryRepo.FindByDocumentID(ct.ID, documentID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrNotFound.WithMessage("entry not found")
		}
		return nil, err
	}

	// Refuse duplicate locale within the same group.
	if existing, err := s.entryRepo.FindTranslationInLocale(ct.ID, effectiveEntryGroupID(source), locale); err == nil && existing != nil {
		return nil, errs.ErrConflict.WithMessage("translation already exists for this locale")
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Merge source data with override.
	data := source.Data
	if req.Data != nil {
		for k, v := range req.Data {
			data[k] = v
		}
	}
	if err := s.validateEntryData(ct, data); err != nil {
		return nil, err
	}

	status := req.Status
	if status == "" {
		status = models.EntryStatusDraft
	}

	entry := models.ContentEntry{
		ContentTypeID:      ct.ID,
		DocumentID:         uuid.New().String(),
		Status:             status,
		Data:               data,
		CreatedByID:        userID,
		UpdatedByID:        userID,
		Locale:             locale,
		TranslationGroupID: new(uint),
	}
	*entry.TranslationGroupID = effectiveEntryGroupID(source)

	if status == models.EntryStatusPublished {
		now := time.Now()
		entry.PublishedAt = &now
	}

	if err := s.entryRepo.Create(&entry); err != nil {
		return nil, errs.New("CREATE_ENTRY_FAILED", "failed to create entry", http.StatusInternalServerError)
	}
	return &entry, nil
}
