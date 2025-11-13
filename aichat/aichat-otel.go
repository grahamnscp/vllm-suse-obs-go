package aichat

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	metric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Variables
var suseObsHTTPEndpoint = os.Getenv("SUSEOBS_EXPORTER_OTLP_HOSTNAME")
var suseObsAPIKey = os.Getenv("SUSEOBS_CLIENT_API_KEY")
var (
	// Global instruments for metrics
	apiLatency     metric.Float64Histogram
	successCounter metric.Int64Counter
)

var suseObsHeaders = map[string]string{
	"Content-Type":  "application/json",
	"Authorization": "SUSEObservability " + suseObsAPIKey,
}

// initOTelTraceProvider
func initOTelTraceProvider(ctx context.Context) (*trace.TracerProvider, error) {

	// Create a new OTLP trace exporter

	// http endpoint
	// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.38.0#Option
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(suseObsHTTPEndpoint),
		otlptracehttp.WithURLPath("/v1/traces"),
		otlptracehttp.WithHeaders(suseObsHeaders),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("initOTelTraceProvider: failed to create OTLP exporter: %w", err)
	}

	// Resource attributes
	resAttr, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.ServiceNamespace(serviceNamespace),
			semconv.DeploymentEnvironment(deploymentEnvironment),
			attribute.String("environment", deploymentEnvironment),
			semconv.TelemetrySDKName(telemetrySDKName),
			attribute.String("telemetry.sdk.name", "openlit"), // New attribute added here
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create a new tracer provider with the OTLP exporter.
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resAttr),
	)

	// Set global TraceProvider
	otel.SetTracerProvider(tp)

	return tp, nil
}

// Create new Metrics Provider
func initOTelMetricsProvider(ctx context.Context) (*sdkmetric.MeterProvider, error) {

	// Metrics Exporter
	me, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(suseObsHTTPEndpoint),
		otlpmetrichttp.WithURLPath("/v1/metrics"),
		otlpmetrichttp.WithHeaders(suseObsHeaders),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Resource attributes
	resAttr, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.ServiceNamespace(serviceNamespace),
			semconv.DeploymentEnvironment(deploymentEnvironment),
			attribute.String("environment", deploymentEnvironment),
			semconv.TelemetrySDKName(telemetrySDKName),
			attribute.String("telemetry.sdk.name", "openlit"), // New attribute added here
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create a periodic reader to push metrics to the exporter every 5 seconds
	metricReader := sdkmetric.NewPeriodicReader(me, sdkmetric.WithInterval(1*time.Second))

	// Create a new meter provider with the OTLP exporter.
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resAttr),
		sdkmetric.WithReader(metricReader),
	)

	// Set global MeterProvider
	otel.SetMeterProvider(mp)

	return mp, nil
}

// initMetrics initializes the metric instruments
func initOTelMetrics() error {

	// Global MeterProvider
	meter := otel.GetMeterProvider().Meter(meterName)

	// Create a Histogram for recording request latency
	var err error
	apiLatency, err = meter.Float64Histogram(
		"openai.api.latency",
		metric.WithDescription("Latency of OpenAI API calls in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1.0, 2.5, 5.0),
	)
	if err != nil {
		return fmt.Errorf("failed to create latency histogram: %w", err)
	}

	// Create a Counter for tracking successful requests
	successCounter, err = meter.Int64Counter(
		"openai.api.success_total",
		metric.WithDescription("Total number of successful OpenAI API calls"),
	)
	if err != nil {
		return fmt.Errorf("failed to create success counter: %w", err)
	}

	return nil
}
