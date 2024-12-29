package kernel

import (
	"context"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type AppDiagnostic struct {
	Tracer trace.Tracer
	Meter  metric.Meter

	RequestCounter metric.Int64Counter
	ErrorCounter   metric.Int64Counter
}

func (diag *AppDiagnostic) BeginTracing(ctx context.Context, spanName string) (trace.Span, context.Context) {
	ctx, span := diag.Tracer.Start(ctx, spanName)
	return span, ctx
}
