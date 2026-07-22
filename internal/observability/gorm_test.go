package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInstrumentGORM_RecordsDatabaseSpans(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := InstrumentGORM(db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	type record struct {
		ID   uint `gorm:"primaryKey"`
		Name string
	}
	if err := db.AutoMigrate(&record{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&record{Name: "test"}).Error; err != nil {
		t.Fatal(err)
	}
	var got record
	if err := db.First(&got).Error; err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]bool)
	for _, span := range recorder.Ended() {
		seen[span.Name()] = true
	}
	if !seen["gorm.create"] || !seen["gorm.query"] {
		t.Fatalf("expected create and query spans, got %v", seen)
	}
}
