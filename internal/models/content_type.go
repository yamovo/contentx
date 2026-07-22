package models

import (
	"time"
)

// ContentType defines a user-created content structure (like Strapi Collection Type).
type ContentType struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	UID          string         `gorm:"uniqueIndex;size:64;not null" json:"uid"` // e.g. "product", "event"
	Name         string         `gorm:"size:128;not null" json:"name"`           // display name
	Description  string         `gorm:"size:512" json:"description"`
	IsSingle     bool           `gorm:"default:false" json:"is_single"`    // single vs collection
	DraftPublish bool           `gorm:"default:true" json:"draft_publish"` // supports draft/publish
	Fields       []ContentField `gorm:"foreignKey:ContentTypeID" json:"fields,omitempty"`
	EntryCount   int64          `gorm:"-" json:"entry_count,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// ContentField defines a field within a content type.
type ContentField struct {
	ID            uint        `gorm:"primarykey" json:"id"`
	ContentTypeID uint        `gorm:"index;not null" json:"content_type_id"`
	Name          string      `gorm:"size:64;not null" json:"name"`       // field name (snake_case)
	Label         string      `gorm:"size:128;not null" json:"label"`     // display label
	FieldType     string      `gorm:"size:32;not null" json:"field_type"` // text, rich_text, integer, float, boolean, date, media, relation, json, enum, email, url
	Required      bool        `gorm:"default:false" json:"required"`
	Unique        bool        `gorm:"default:false" json:"unique"`
	DefaultValue  string      `gorm:"type:text" json:"default_value,omitempty"`
	Options       StringSlice `gorm:"type:text" json:"options,omitempty"`     // for enum values
	RelationType  string      `gorm:"size:32" json:"relation_type,omitempty"` // one_to_one, one_to_many, many_to_one, many_to_many
	RelationUID   string      `gorm:"size:64" json:"relation_uid,omitempty"`  // related content type UID
	MinLength     *int        `json:"min_length,omitempty"`
	MaxLength     *int        `json:"max_length,omitempty"`
	MinValue      *float64    `json:"min_value,omitempty"`
	MaxValue      *float64    `json:"max_value,omitempty"`
	SortOrder     int         `gorm:"default:0" json:"sort_order"`
}

// FieldType constants.
const (
	FieldTypeText     = "text"
	FieldTypeRichText = "rich_text"
	FieldTypeInteger  = "integer"
	FieldTypeFloat    = "float"
	FieldTypeBoolean  = "boolean"
	FieldTypeDate     = "date"
	FieldTypeMedia    = "media"
	FieldTypeRelation = "relation"
	FieldTypeJSON     = "json"
	FieldTypeEnum     = "enum"
	FieldTypeEmail    = "email"
	FieldTypeURL      = "url"
	FieldTypeSlug     = "slug"
)

// ValidFieldTypes is the list of all supported field types.
var ValidFieldTypes = map[string]bool{
	FieldTypeText:     true,
	FieldTypeRichText: true,
	FieldTypeInteger:  true,
	FieldTypeFloat:    true,
	FieldTypeBoolean:  true,
	FieldTypeDate:     true,
	FieldTypeMedia:    true,
	FieldTypeRelation: true,
	FieldTypeJSON:     true,
	FieldTypeEnum:     true,
	FieldTypeEmail:    true,
	FieldTypeURL:      true,
	FieldTypeSlug:     true,
}
