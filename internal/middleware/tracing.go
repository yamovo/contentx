package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware creates a server span, extracts W3C trace headers, and
// exposes the resulting TraceID in Gin context, logs, and response headers.
func TracingMiddleware(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName + "/http")
	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		spanName := c.Request.Method + " " + path
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("url.path", c.Request.URL.Path),
				attribute.String("http.route", path),
			),
		)
		if requestID, ok := c.Get("request_id"); ok {
			span.SetAttributes(attribute.String("request.id", fmt.Sprint(requestID)))
		}
		c.Request = c.Request.WithContext(ctx)
		if span.SpanContext().IsValid() {
			traceID := span.SpanContext().TraceID().String()
			c.Set("trace_id", traceID)
			c.Header("X-Trace-ID", traceID)
		}

		c.Next()
		statusCode := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		}
		for _, ginErr := range c.Errors {
			span.RecordError(ginErr.Err)
		}
		span.End()
	}
}
