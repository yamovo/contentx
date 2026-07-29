package repository

import (
	"regexp"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ============================================================
// ContentTypeRepository
// ============================================================

// ContentTypeRepository defines data-access operations for content types
// and their field definitions.
type ContentTypeRepository interface {
	CountByUID(uid string) (int64, error)
	Create(ct *models.ContentType) error
	List() ([]models.ContentType, error)
	FindByUID(uid string) (*models.ContentType, error)
	FindByID(id uint) (*models.ContentType, error)
	Delete(id uint) error
	CountEntriesByTypeID(typeID uint) (int64, error)
}

// gormContentTypeRepository implements ContentTypeRepository with GORM.
type gormContentTypeRepository struct {
	db *gorm.DB
}

// NewContentTypeRepository builds a GORM-backed ContentTypeRepository.
func NewContentTypeRepository(db *gorm.DB) ContentTypeRepository {
	return &gormContentTypeRepository{db: db}
}

func (r *gormContentTypeRepository) CountByUID(uid string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.ContentType{}).Where("uid = ?", uid).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *gormContentTypeRepository) Create(ct *models.ContentType) error {
	return r.db.Create(ct).Error
}

func (r *gormContentTypeRepository) List() ([]models.ContentType, error) {
	var types []models.ContentType
	if err := r.db.Preload("Fields", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("created_at ASC").Find(&types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

func (r *gormContentTypeRepository) FindByUID(uid string) (*models.ContentType, error) {
	var ct models.ContentType
	if err := r.db.Preload("Fields", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Where("uid = ?", uid).First(&ct).Error; err != nil {
		return nil, err
	}
	return &ct, nil
}

func (r *gormContentTypeRepository) FindByID(id uint) (*models.ContentType, error) {
	var ct models.ContentType
	if err := r.db.First(&ct, id).Error; err != nil {
		return nil, err
	}
	return &ct, nil
}

func (r *gormContentTypeRepository) Delete(id uint) error {
	// Best-effort: delete entries, then fields, then the type itself
	// (mirrors prior service behaviour; no transaction).
	if err := r.db.Where("content_type_id = ?", id).Delete(&models.ContentEntry{}).Error; err != nil {
		return err
	}
	if err := r.db.Where("content_type_id = ?", id).Delete(&models.ContentField{}).Error; err != nil {
		return err
	}
	return r.db.Delete(&models.ContentType{}, id).Error
}

func (r *gormContentTypeRepository) CountEntriesByTypeID(typeID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.ContentEntry{}).Where("content_type_id = ?", typeID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ============================================================
// ContentEntryRepository
// ============================================================

// ContentEntryListFilter holds query parameters for listing content entries.
type ContentEntryListFilter struct {
	TypeID   uint
	Page     int
	PageSize int
	Status   string
	Search   string
	Sort     string
	Filters  map[string]string // field_name=value (applied via dialect-aware JSON extraction)
	Locale   string            // i18n: filter by locale (exact match)
}

// ContentEntryRepository defines data-access operations for content entries.
type ContentEntryRepository interface {
	FindByDocumentID(typeID uint, docID string) (*models.ContentEntry, error)
	Create(entry *models.ContentEntry) error
	Save(entry *models.ContentEntry) error
	DeleteByDocumentID(typeID uint, docID string) (rowsAffected int64, err error)
	List(filter ContentEntryListFilter) ([]models.ContentEntry, int64, error)
	FindByIDs(typeID uint, ids []uint) ([]models.ContentEntry, error)
	Search(typeID uint, query string, limit int) ([]models.ContentEntry, error)
	ExportAll(typeID uint) ([]models.ContentEntry, error)
	CreateMany(entries []models.ContentEntry) (int, error)
	// i18n: ListTranslations returns all entries sharing the same
	// translation group (excluding the entry itself).
	ListTranslations(typeID, groupID, excludeID uint) ([]models.ContentEntry, error)
	// FindTranslationInLocale returns the entry in the given translation
	// group for the requested locale, or gorm.ErrRecordNotFound.
	FindTranslationInLocale(typeID, groupID uint, locale string) (*models.ContentEntry, error)
}

// validJSONFieldName restricts JSON filter fields to safe identifiers,
// matching how ContentField names are defined.
var validJSONFieldName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsonFieldEqual builds a dialect-specific WHERE condition comparing a JSON
// field inside the text `data` column against a value. The path/key and the
// value stay fully parameterized on every dialect.
//
// json_extract is SQLite syntax; MySQL needs JSON_UNQUOTE to strip the JSON
// string quoting before comparison; PostgreSQL stores `data` as text and
// needs a jsonb cast plus the ->> text operator.
func jsonFieldEqual(dialect, field, value string) (string, []interface{}) {
	switch dialect {
	case "postgres":
		return "data::jsonb ->> ? = ?", []interface{}{field, value}
	case "mysql":
		return "JSON_UNQUOTE(JSON_EXTRACT(data, ?)) = ?", []interface{}{"$." + field, value}
	default: // sqlite
		return "json_extract(data, ?) = ?", []interface{}{"$." + field, value}
	}
}

// gormContentEntryRepository implements ContentEntryRepository with GORM.
type gormContentEntryRepository struct {
	db *gorm.DB
}

// NewContentEntryRepository builds a GORM-backed ContentEntryRepository.
func NewContentEntryRepository(db *gorm.DB) ContentEntryRepository {
	return &gormContentEntryRepository{db: db}
}

func (r *gormContentEntryRepository) FindByDocumentID(typeID uint, docID string) (*models.ContentEntry, error) {
	var entry models.ContentEntry
	if err := r.db.Where("content_type_id = ? AND document_id = ?", typeID, docID).First(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *gormContentEntryRepository) Create(entry *models.ContentEntry) error {
	return r.db.Create(entry).Error
}

func (r *gormContentEntryRepository) Save(entry *models.ContentEntry) error {
	return r.db.Save(entry).Error
}

func (r *gormContentEntryRepository) DeleteByDocumentID(typeID uint, docID string) (int64, error) {
	result := r.db.Where("content_type_id = ? AND document_id = ?", typeID, docID).Delete(&models.ContentEntry{})
	return result.RowsAffected, result.Error
}

func (r *gormContentEntryRepository) List(filter ContentEntryListFilter) ([]models.ContentEntry, int64, error) {
	query := r.db.Model(&models.ContentEntry{}).Where("content_type_id = ?", filter.TypeID)

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Locale != "" {
		query = query.Where("locale = ?", filter.Locale)
	}

	// JSON field filters (dialect-aware; see jsonFieldEqual). Field names
	// come from arbitrary query params, so invalid identifiers are skipped
	// instead of producing a malformed JSON path error.
	for field, value := range filter.Filters {
		if !validJSONFieldName.MatchString(field) {
			continue
		}
		clause, args := jsonFieldEqual(r.db.Name(), field, value)
		query = query.Where(clause, args...)
	}

	// Search in text fields.
	if filter.Search != "" {
		query = query.Where("data LIKE ?", "%"+filter.Search+"%")
	}

	// Sorting.
	switch filter.Sort {
	case "oldest":
		query = query.Order("created_at ASC")
	case "updated":
		query = query.Order("updated_at DESC")
	default:
		query = query.Order("created_at DESC")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var entries []models.ContentEntry
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

func (r *gormContentEntryRepository) FindByIDs(typeID uint, ids []uint) ([]models.ContentEntry, error) {
	var entries []models.ContentEntry
	if err := r.db.Where("content_type_id = ? AND id IN ?", typeID, ids).Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *gormContentEntryRepository) Search(typeID uint, query string, limit int) ([]models.ContentEntry, error) {
	var entries []models.ContentEntry
	if err := r.db.Where("content_type_id = ? AND data LIKE ?", typeID, "%"+query+"%").
		Order("created_at DESC").
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *gormContentEntryRepository) ExportAll(typeID uint) ([]models.ContentEntry, error) {
	var entries []models.ContentEntry
	if err := r.db.Where("content_type_id = ?", typeID).Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *gormContentEntryRepository) CreateMany(entries []models.ContentEntry) (int, error) {
	count := 0
	for _, entry := range entries {
		// Match the original ImportEntries behaviour: create one by one,
		// ignore per-row errors, count successes.
		if err := r.db.Create(&entry).Error; err == nil {
			count++
		}
	}
	return count, nil
}

// ─── i18n: translation queries ──────────────────────────────────────────────

func (r *gormContentEntryRepository) ListTranslations(typeID, groupID, excludeID uint) ([]models.ContentEntry, error) {
	var entries []models.ContentEntry
	// The group root has translation_group_id = NULL; its own id is the
	// group id. Match it via (id = groupID) so siblings see the root too.
	if err := r.db.Where(
		"content_type_id = ? AND (translation_group_id = ? OR (translation_group_id IS NULL AND id = ?)) AND id != ?",
		typeID, groupID, groupID, excludeID,
	).Order("locale ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *gormContentEntryRepository) FindTranslationInLocale(typeID, groupID uint, locale string) (*models.ContentEntry, error) {
	var entry models.ContentEntry
	if err := r.db.Where(
		"content_type_id = ? AND (translation_group_id = ? OR (translation_group_id IS NULL AND id = ?)) AND locale = ?",
		typeID, groupID, groupID, locale,
	).First(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}
