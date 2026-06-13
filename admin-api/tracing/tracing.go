// tracing/tracing.go
// OpenTelemetry tracing setup for Cont Admin API.
// Supports OTLP exporter (OTEL_EXPORTER_OTLP_ENDPOINT) and W3C TraceContext propagation.

package tracing

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

var tp *sdktrace.TracerProvider

// Init initializes the OpenTelemetry tracer.
// If OTEL_EXPORTER_OTLP_ENDPOINT is not set, tracing is a no-op (silent fallback).
func Init() {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()

	var exporter *otlptrace.Exporter
	var err error

	if endpoint != "" {
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			log.Printf("cont/tracing: failed to create OTLP exporter: %v (tracing disabled)", err)
		}
	}

	res, resErr := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", "cont-admin-api"),
			attribute.String("service.version", "1.0.0"),
		),
	)
	if resErr != nil {
		log.Printf("cont/tracing: failed to create resource: %v", resErr)
		res = resource.Default()
	}

	if exporter != nil {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		log.Printf("cont/tracing: OTLP exporter configured, endpoint=%s", endpoint)
	} else {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
		log.Printf("cont/tracing: no OTLP endpoint set, tracing disabled (spans in-memory)")
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

// Tracer returns a tracer for creating spans.
func Tracer(name string) trace.Tracer {
	return tp.Tracer(name)
}
