package kernel

import (
	"context"
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"log"
	"strings"
)

type StepData struct {
	span trace.Span
	ctx  context.Context
	name string
}

type RequestRuntime struct {
	AppRuntime *AppRuntime
	DB         *gorm.DB

	Token *models.Token

	RequestContext *gin.Context
	Span           trace.Span
	SpanContext    context.Context

	Error error

	pairs   []*StepData
	current uint8
}

func (rt *RequestRuntime) logf(format string, args ...interface{}) {
	if rt.AppRuntime.Debug {
		log.Printf(format, args...)
	}
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

		pairs:   make([]*StepData, 0),
		current: 0,
	}

	rt.pairs = append(rt.pairs, &StepData{span: span, ctx: ctx, name: rctx.FullPath()})

	return rt
}

func (rt *RequestRuntime) StepInto(spanName string) {
	ctx, span := rt.AppRuntime.Diagnostic.Tracer.Start(rt.SpanContext, spanName)
	rt.pairs = append(rt.pairs, &StepData{span: span, ctx: ctx, name: spanName})
	rt.logf("%s-> Stepping into %d [%s] -> %d [%s]", strings.Repeat("| ", int(rt.current)), rt.current, rt.pairs[rt.current].name, rt.current+1, spanName)
	rt.current = rt.current + 1
	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
}

func (rt *RequestRuntime) StepBack() {
	rt.logf("%s<- Stepping back %d [%s] -> %d [%s]", strings.Repeat("| ", int(rt.current)), rt.current, rt.pairs[rt.current].name, rt.current-1, rt.pairs[rt.current-1].name)
	if rt.current-1 < 0 {
		rt.logf("  ! Can't step back from %d.", rt.current)
		return
	}
	rt.End()
	rt.current = rt.current - 1
	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
	rt.pairs = rt.pairs[:len(rt.pairs)-1]
}

func (rt *RequestRuntime) StepBackWithMessage(msg string) {
	rt.logf("%s<- Stepping back %d [%s] -> %d [%s] - %s", strings.Repeat("| ", int(rt.current-1)), rt.current, rt.pairs[rt.current].name, rt.current-1, rt.pairs[rt.current-1].name, msg)
	if rt.current-1 < 0 {
		rt.logf("%s ! Can't step back from %d", strings.Repeat("| ", int(rt.current)), rt.current)
		return
	}
	rt.EndMessage(msg)
	rt.current = rt.current - 1
	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
	rt.pairs = append(rt.pairs[:rt.current], rt.pairs[rt.current+1:]...)
}

func (rt *RequestRuntime) SkipBackTo(index uint8) {
	rt.logf("%s<> Skipping back to %d", strings.Repeat("| ", int(rt.current)), index)
	for ; rt.current > index; rt.current-- {
		rt.StepBack()
	}
}

func (rt *RequestRuntime) End() {
	rt.logf("%s * Ending %d [%s]", strings.Repeat("| ", int(rt.current)), rt.current, rt.pairs[rt.current].name)
	if rt.Span.IsRecording() {
		rt.Span.End()
	}
}

func (rt *RequestRuntime) EndMessage(msg string) {
	rt.logf("%s * Ending %d [%s] - %s", strings.Repeat("| ", int(rt.current)), rt.current, rt.pairs[rt.current].name, msg)
	if rt.Span.IsRecording() {
		rt.Span.End()
	}
}

func (rt *RequestRuntime) SetIndex(index uint8) {
	rt.logf("%s * Setting index %d", strings.Repeat("| ", int(index)), rt.current)
	rt.current = index
	pair := rt.pairs[rt.current]
	rt.Span = pair.span
	rt.SpanContext = pair.ctx
}
