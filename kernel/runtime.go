package kernel

import (
	"context"
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"log"
)

type spanCtxPair struct {
	span trace.Span
	ctx  context.Context
}

type RequestRuntime struct {
	AppRuntime *AppRuntime
	DB         *gorm.DB

	Token *models.Token

	RequestContext *gin.Context
	Span           trace.Span
	SpanContext    context.Context

	Error error

	pairs   []*spanCtxPair
	current uint8
}

func InitRequest(art *AppRuntime, rctx *gin.Context) *RequestRuntime {
	ctx := rctx.Request.Context()
	span, ctx := art.Diagnostic.BeginTracing(ctx, rctx.FullPath())

	log.Printf("Initializing request %s", rctx.Request.RequestURI)

	rt := &RequestRuntime{
		AppRuntime: art,
		DB:         art.DatabaseClient,

		RequestContext: rctx,
		Span:           span,
		SpanContext:    ctx,

		pairs:   make([]*spanCtxPair, 0),
		current: 0,
	}

	rt.pairs = append(rt.pairs, &spanCtxPair{span: span, ctx: ctx})

	return rt
}

func (rt *RequestRuntime) NewChildTracer(spanName string) *RequestRuntime {
	ctx, span := rt.AppRuntime.Diagnostic.Tracer.Start(rt.SpanContext, spanName)
	log.Printf("  - Initializing child tracer %s (%s)", spanName, span.SpanContext().SpanID())
	rt.PushTrace(span, ctx)
	return rt
}

func (rt *RequestRuntime) PushTrace(span trace.Span, ctx context.Context) {
	log.Printf("    - Pushing trace pair %+v into stack (no. %d)", &span, len(rt.pairs))
	rt.pairs = append(rt.pairs, &spanCtxPair{span: span, ctx: ctx})
}

func (rt *RequestRuntime) Advance() {
	log.Printf("-> Advancing %d -> %d", rt.current, rt.current+1)
	if uint8(len(rt.pairs)-1) < rt.current+1 {
		log.Printf(" !!! trying to advance out of bounds, %d < %d", uint8(len(rt.pairs)-1), rt.current+1)
		return
	}
	rt.current = rt.current + 1

	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
}

func (rt *RequestRuntime) StepBack() {
	log.Printf("<- Stepping back %d -> %d", rt.current, rt.current-1)
	if rt.current-1 < 0 {
		log.Printf(" !!! trying to step back out of bounds")
		return
	}
	if rt.current-1 > rt.current {
		log.Printf(" !!! uint8 underflow, not continuing")
		return
	}

	rt.current = rt.current - 1

	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
}

func (rt *RequestRuntime) SkipOverTo(index uint8) {
	log.Printf("<> Skipping over %d -> %d", rt.current, index)
	if uint8(len(rt.pairs)) < index {
		log.Printf("trying to skip over out of bounds")
		return
	}

	rt.current = index
	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
}

func (rt *RequestRuntime) End() *RequestRuntime {
	log.Printf(" * Ending %d", rt.current)
	if rt.Span.IsRecording() {
		rt.Span.End()
	} else {
		log.Printf(" - Already ended!")
		return rt
	}
	rt.pairs = append(rt.pairs[:rt.current], rt.pairs[rt.current+1:]...)
	return rt
}

func (rt *RequestRuntime) EndBlock() {
	rt.End().StepBack()
}
