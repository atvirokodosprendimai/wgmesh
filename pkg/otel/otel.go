// Package otel provides OpenTelemetry initialization for wgmesh.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is set, the package configures
// TracerProvider, MeterProvider, and LoggerProvider with gRPC OTLP exporters.
// When the env var is unset, noop providers are used with zero overhead.
package otel

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Init initializes OpenTelemetry providers based on environment variables.
//
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it configures gRPC OTLP exporters
// for traces, metrics, and logs. Otherwise, global providers remain as noops.
//
// The returned function must be called on shutdown to flush pending telemetry.
// It is safe to call even when no exporter was configured.
func Init(ctx context.Context, serviceName, serviceVersion string) (func(context.Context), error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return func(context.Context) {}, nil
	}

	res, err := buildResource(ctx, serviceName, serviceVersion)
	if err != nil {
		return func(context.Context) {}, fmt.Errorf("otel resource: %w", err)
	}

	// Trace provider
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return func(context.Context) {}, fmt.Errorf("otel trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Metric provider
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return shutdownFunc(tp, nil, nil), fmt.Errorf("otel metric exporter: %w", err)
	}
	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(30*time.Second))),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Log provider
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return shutdownFunc(tp, mp, nil), fmt.Errorf("otel log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	otellog.SetLoggerProvider(lp)

	// Install log bridge so existing log.Printf calls emit OTel log records
	InstallLogBridge(lp)

	log.Printf("[OTel] initialized: endpoint=%s service=%s", endpoint, serviceName)

	return shutdownFunc(tp, mp, lp), nil
}

// buildResource creates the OTel resource with service and host attributes.
func buildResource(ctx context.Context, serviceName, serviceVersion string) (*resource.Resource, error) {
	hostname, _ := os.Hostname()

	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.HostName(hostname),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
}

type shutdownable interface {
	Shutdown(context.Context) error
}

// shutdownFunc returns a function that shuts down all non-nil providers with a timeout.
func shutdownFunc(providers ...shutdownable) func(context.Context) {
	return func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		for _, p := range providers {
			if p != nil {
				if err := p.Shutdown(ctx); err != nil {
					log.Printf("[OTel] shutdown error: %v", err)
				}
			}
		}
	}
}
