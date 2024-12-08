package utils

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.26.0"
	"os"
	"time"
)

func SetupOtel(c *AppConfig) (func(), error) {
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(c.ServiceName),
			semconv.ServiceVersion(c.ServiceVersion),
			semconv.DeploymentEnvironment(c.DeploymentEnvironment),
		))
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracehttp.New(c.Context,
		otlptracehttp.WithEndpoint(c.JaegerEndpoint),
		otlptracehttp.WithInsecure(), // TODO: Remove for production, use TLS
	)
	if err != nil {
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}

	//metricExporter, err := otlpmetrichttp.New(context.Background(),
	//	otlpmetrichttp.WithEndpoint(c.PrometheusEndpoint),
	//	otlpmetrichttp.WithInsecure(), // TODO: Remove for production, use TLS
	//)
	metricExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("creating metric exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(tracerProvider)

	// Metric Provider
	//metricProvider := sdkmetric.NewMeterProvider(
	//	sdkmetric.WithResource(res),
	//	sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	//)
	metricProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricExporter),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(metricProvider)

	// Propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Runtime metrics
	if err := runtime.Start(); err != nil {
		return nil, fmt.Errorf("starting runtime metrics: %w", err)
	}

	// Request counter metric
	counter, err := c.Meter.Int64Counter("http_requests_total",
		metric.WithDescription("Total number of HTTP requests"))
	if err != nil {
		return nil, fmt.Errorf("creating request counter: %w", err)
	}
	c.RequestCounter = counter

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}).With().Caller().Logger()

	// Set global logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Cleanup function
	return func() {
		_ = tracerProvider.Shutdown(c.Context)
		_ = metricProvider.Shutdown(c.Context)
	}, nil
}
