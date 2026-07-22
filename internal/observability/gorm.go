package observability

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const gormSpanKey = "contentx:otel-span"

// InstrumentGORM adds client spans around GORM create/query/update/delete/raw operations.
func InstrumentGORM(db *gorm.DB, driver string) error {
	if err := db.Callback().Create().Before("gorm:create").Register("contentx:otel:before_create", beforeGORM("create", driver)); err != nil {
		return fmt.Errorf("register GORM create callback: %w", err)
	}
	if err := db.Callback().Create().After("gorm:create").Register("contentx:otel:after_create", afterGORM); err != nil {
		return fmt.Errorf("register GORM create callback: %w", err)
	}
	if err := db.Callback().Query().Before("gorm:query").Register("contentx:otel:before_query", beforeGORM("query", driver)); err != nil {
		return fmt.Errorf("register GORM query callback: %w", err)
	}
	if err := db.Callback().Query().After("gorm:query").Register("contentx:otel:after_query", afterGORM); err != nil {
		return fmt.Errorf("register GORM query callback: %w", err)
	}
	if err := db.Callback().Update().Before("gorm:update").Register("contentx:otel:before_update", beforeGORM("update", driver)); err != nil {
		return fmt.Errorf("register GORM update callback: %w", err)
	}
	if err := db.Callback().Update().After("gorm:update").Register("contentx:otel:after_update", afterGORM); err != nil {
		return fmt.Errorf("register GORM update callback: %w", err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("contentx:otel:before_delete", beforeGORM("delete", driver)); err != nil {
		return fmt.Errorf("register GORM delete callback: %w", err)
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("contentx:otel:after_delete", afterGORM); err != nil {
		return fmt.Errorf("register GORM delete callback: %w", err)
	}
	if err := db.Callback().Row().Before("gorm:row").Register("contentx:otel:before_row", beforeGORM("row", driver)); err != nil {
		return fmt.Errorf("register GORM row callback: %w", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register("contentx:otel:after_row", afterGORM); err != nil {
		return fmt.Errorf("register GORM row callback: %w", err)
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register("contentx:otel:before_raw", beforeGORM("raw", driver)); err != nil {
		return fmt.Errorf("register GORM raw callback: %w", err)
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("contentx:otel:after_raw", afterGORM); err != nil {
		return fmt.Errorf("register GORM raw callback: %w", err)
	}
	return nil
}

func beforeGORM(operation, driver string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db.Statement == nil || db.Statement.Context == nil {
			return
		}
		ctx, span := otel.Tracer("contentx/gorm").Start(
			db.Statement.Context,
			"gorm."+operation,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system.name", driver),
				attribute.String("db.operation.name", operation),
			),
		)
		db.Statement.Context = ctx
		db.InstanceSet(gormSpanKey, span)
	}
}

func afterGORM(db *gorm.DB) {
	value, ok := db.InstanceGet(gormSpanKey)
	if !ok {
		return
	}
	span, ok := value.(trace.Span)
	if !ok {
		return
	}
	if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
		span.RecordError(db.Error)
		span.SetStatus(codes.Error, db.Error.Error())
	}
	if db.Statement != nil && db.Statement.Table != "" {
		span.SetAttributes(attribute.String("db.collection.name", db.Statement.Table))
	}
	span.End()
}
