package utils

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
)

func SpanErr(span trace.Span, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.End()
	return err
}

func SpanErrf(span trace.Span, format string, args ...interface{}) error {
	return SpanErr(span, fmt.Errorf(format, args...))
}

func SpanHttpErrf(span trace.Span, rsp *http.Response, format string, args ...interface{}) error {
	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return SpanErrf(span, "failed to read response body: %v", err)
	}

	span.RecordError(fmt.Errorf("http request returned %d: %s", rsp.StatusCode, string(body)))
	return fmt.Errorf(format, args...)
}

func SpanGinErr(span trace.Span, c *gin.Context, status int, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	c.AbortWithStatusJSON(status, &gin.H{
		"error":   err.Error(),
		"traceId": span.SpanContext().TraceID().String(),
		"spanId":  span.SpanContext().SpanID().String(),
	})
	span.End()
}

func SpanGinErrf(span trace.Span, c *gin.Context, status int, format string, args ...interface{}) {
	SpanGinErr(span, c, status, fmt.Errorf(format, args...))
}
