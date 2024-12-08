package main

import (
	"context"
	"errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"git.sr.ht/~aondrejcak/ts-api/endpoints"
	"git.sr.ht/~aondrejcak/ts-api/middleware"
	"git.sr.ht/~aondrejcak/ts-api/utils"
)

func main() {
	config := utils.LoadConfig()
	config.Context = context.Background()

	cleanupFunc, err := utils.SetupOtel(config)
	if err != nil {
		log.Fatal().Err(err).Msg("error setting up otel")
	}
	defer cleanupFunc()

	_, span := config.Tracer.Start(config.Context, "main")
	defer span.End()

	err = utils.PrepareDatabase(config)
	if err != nil {
		span.RecordError(err)
		log.Fatal().
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String()).
			Err(err).Msg("failed to prepare database")
	}

	r := gin.Default() // TODO: route & recovery middleware (prod)
	err = r.SetTrustedProxies([]string{})
	if err != nil {
		span.RecordError(err)
		log.Fatal().Err(err).Msg("failed to set trusted proxies: %s")
	}

	r.Use(otelgin.Middleware(config.ServiceName))
	r.Use(middleware.TracerMiddleware())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, &gin.Error{
			Err: errors.New("route not found"),
		})
	})

	r.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Next()

			if len(c.Errors) > 0 {
				c.JSON(500, &gin.Error{
					Err: errors.New(c.Errors.Last().Error()),
				})
				return
			}
		}
	}())

	r.POST("/authorize", endpoints.Authorize)
	r.POST("/callback", endpoints.Callback_)

	authorized := r.Group("/")
	authorized.Use(middleware.TokenMiddleware())
	{
		authorized.GET("/accounts", endpoints.Accounts)
		authorized.GET("/transactions", endpoints.Transactions)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	endpoints.RegisterControllers(r)

	err = r.Run(config.Host)
	if err != nil {
		span.RecordError(err)
		log.Fatal().Err(err).Msg("failed to start app: %s")
	}
}
