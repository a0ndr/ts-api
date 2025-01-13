package kernel

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	attribute2 "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"io"
	"net/http"
)

func (rt *RequestRuntime) MakeError(err error) error {
	s := rt.Span
	s.RecordError(err)
	s.SetStatus(codes.Error, err.Error())
	s.End()
	rt.Error = err
	rt.StepBack()

	return err
}

func (rt *RequestRuntime) MakeErrorf(format string, args ...interface{}) error {
	return rt.MakeError(fmt.Errorf(format, args...))
}

func (rt *RequestRuntime) MakeErrorFromHttp(rsp *http.Response, err error) error {
	body, ioErr := io.ReadAll(rsp.Body)
	if ioErr != nil {
		return rt.MakeErrorf("failed to read response body: %v", ioErr)
	}
	rt.Span.RecordError(fmt.Errorf("http request returned %d: %s", rsp.StatusCode, string(body)))
	rt.Span.SetStatus(codes.Error, string(body))
	rt.Span.End()

	return err
}

func (rt *RequestRuntime) MakeErrorfFromHttp(rsp *http.Response, format string, args ...interface{}) error {
	return rt.MakeErrorFromHttp(rsp, fmt.Errorf(format, args...))
}

func (rt *RequestRuntime) RecordSpanStack() {
	if rt.Error == nil {
		oldIndex := rt.current
		rt.SkipBackTo(0)

		// this is running in the main span
		for index, pair := range rt.pairs {
			rt.Span.SetAttributes(attribute.KeyValue(attribute2.Key("span_stack."+string(rune(index))), gin.H{"span": pair.span, "context": pair.ctx}))
		}
		rt.Span.SetAttributes(attribute.KeyValue("span_stack.current", rt.current))

		rt.SkipBackTo(oldIndex)
	}
}

func (rt *RequestRuntime) E(code int, err error) *RequestRuntime {
	rt.RequestContext.AbortWithStatusJSON(code, &gin.H{
		"error":   rt.MakeError(err).Error(),
		"traceId": rt.Span.SpanContext().TraceID().String(),
	})
	return rt
}

func (rt *RequestRuntime) Ef(code int, format string, args ...interface{}) *RequestRuntime {
	return rt.E(code, fmt.Errorf(format, args...))
}
