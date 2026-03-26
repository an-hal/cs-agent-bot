package tracer

import (
	"context"
	"fmt"
	"os"
	"strings"

	gcpexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"github.com/Sejutacita/cs-agent-bot/config"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer wraps OpenTelemetry tracer for distributed tracing
type Tracer interface {
	Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
	Shutdown(ctx context.Context) error
}

type otelTracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
}

// New creates a new tracer based on configuration
func New(cfg *config.AppConfig) (Tracer, error) {
	// Create resource with service metadata
	res, err := createResource(cfg.TracerServiceName, cfg.TracerServiceVersion, cfg.Env)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	exporter, sampler, err := createExporter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set global propagator for W3C Trace Context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := provider.Tracer(cfg.TracerServiceName)

	return &otelTracer{
		tracer:   tracer,
		provider: provider,
	}, nil
}

// Start creates a new span
func (t *otelTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// Shutdown gracefully shuts down the tracer
func (t *otelTracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// createResource creates an OpenTelemetry resource with service metadata
func createResource(serviceName, serviceVersion, environment string) (*resource.Resource, error) {
	if podNamespace := os.Getenv("POD_NAMESPACE"); podNamespace != "" {
		serviceName = fmt.Sprintf("%s-%s", podNamespace, serviceName)
	}

	res, err := resource.New(
		context.Background(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// noopTracer is a no-op tracer for testing
type noopTracer struct{}

// NewNoopTracer creates a no-op tracer for testing
func NewNoopTracer() Tracer {
	return &noopTracer{}
}

// Start creates a no-op span
func (t *noopTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

// Shutdown is a no-op
func (t *noopTracer) Shutdown(ctx context.Context) error {
	return nil
}

// createExporter creates the appropriate exporter and sampler based on configuration
func createExporter(cfg *config.AppConfig) (sdktrace.SpanExporter, sdktrace.Sampler, error) {
	tracerExporter := strings.ToLower(cfg.TracerExporter)

	// GCP Cloud Trace exporter
	if tracerExporter == "gcp" {
		gcpProject := cfg.GCPProject
		if gcpProject == "" {
			// Try to auto-detect from environment
			gcpProject = os.Getenv("GCP_PROJECT")
		}

		if gcpProject != "" {
			exporter, err := gcpexporter.New(gcpexporter.WithProjectID(gcpProject))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create GCP exporter: %w", err)
			}

			sampler := sdktrace.TraceIDRatioBased(0.01)
			return exporter, sampler, nil
		}

		tracerExporter = "stdout"
	}

	// Zipkin exporter
	if tracerExporter == "zipkin" && cfg.TracerZipkinCreateSpanURL != "" {
		exporter, err := zipkin.New(cfg.TracerZipkinCreateSpanURL)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create Zipkin exporter: %w", err)
		}

		sampler := sdktrace.AlwaysSample()
		return exporter, sampler, nil
	}

	// Default: Stdout exporter
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	return exporter, sampler, nil
}
