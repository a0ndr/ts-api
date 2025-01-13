package middleware

import (
	"bytes"
	"context"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"io"
)

type responseWriter struct {
	gin.ResponseWriter
	ctx  context.Context
	span trace.Span
	body []byte
}

func TracerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		art := kernel.LoadConfig()
		rt := kernel.InitRequest(art, c)

		rt.Span.SetAttributes(
			attribute.KeyValue("http.method", c.Request.Method),
			attribute.KeyValue("http.url", c.Request.URL.String()),
			attribute.KeyValue("http.host", c.Request.Host),
		)

		bodyBytes, _ := c.GetRawData()
		rt.Span.SetAttributes(attribute.KeyValue("http.request_body", string(bodyBytes)))
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		art.Diagnostic.RequestCounter.Add(rt.SpanContext, 1,
			metric.WithAttributes(attribute.KeyValue("http.method", c.Request.Method)),
		)

		c.Writer = &responseWriter{
			ResponseWriter: c.Writer,
			ctx:            rt.SpanContext,
			span:           rt.Span,
		}

		c.Set("rt", rt)

		rt.End()
		rt.SetIndex(0)
		c.Next()
	}
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body = b

	w.span.SetAttributes(attribute.KeyValue("http.response_body", string(b)))

	return w.ResponseWriter.Write(b)
}
