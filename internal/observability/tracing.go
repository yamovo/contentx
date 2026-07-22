package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TraceOptions configures the process-wide OpenTelemetry tracer provider.
type TraceOptions struct {
	Enabled     bool
	Endpoint    string
	Insecure    bool
	SampleRatio float64
	ServiceName string
	Version     string
	Environment string
}

// InitTracing installs W3C propagation and, when enabled, an OTLP/HTTP exporter.
// The returned shutdown function must be called during graceful shutdown.
func InitTracing(ctx context.Context, opts TraceOptions) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if !opts.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exporterOptions := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(opts.Endpoint)}
	if opts.Insecure {
		exporterOptions = append(exporterOptions, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, exporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	ratio := opts.SampleRatio
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", opts.ServiceName),
		attribute.String("service.version", opts.Version),
		attribute.String("deployment.environment.name", opts.Environment),
	))
	if err != nil {
		return nil, fmt.Errorf("create trace resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}
