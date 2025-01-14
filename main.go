package main

import (
	"context"
	"errors"
	"git.sr.ht/~aondrejcak/ts-api/endpoints/payments"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"git.sr.ht/~aondrejcak/ts-api/endpoints"
	"git.sr.ht/~aondrejcak/ts-api/middleware"
)

func main() {
	art := kernel.LoadConfig()
	art.Context = context.Background()

	if art.DeploymentEnvironment == "production" {
		log.Printf(" === RUNNING IN PRODUCTION MODE ===")
		gin.SetMode(gin.ReleaseMode)
	}

	cleanupFunc, err := art.SetupOtel()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanupFunc()

	span, _ := art.Diagnostic.BeginTracing(art.Context, "main")
	defer span.End()

	err = art.PrepareDatabase()
	if err != nil {
		span.RecordError(err)
	}

	r := gin.Default() // TODO: route & recovery middleware (prod)
	err = r.SetTrustedProxies([]string{})
	if err != nil {
		span.RecordError(err)
		log.Fatal(err)
	}

	if art.DeploymentEnvironment == "production" {
		r.Use(gin.Logger())
		r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "a panic occurred, request aborted",
			})
		}))
		r.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"https://portal.tadam.space"},
			AllowMethods:     []string{"POST", "OPTIONS"},
			AllowHeaders:     []string{"Content-Type"},
			ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin", "Access-Control-Allow-Headers", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           7 * time.Hour * 24,
			AllowAllOrigins:  false,
		}))
	}

	r.Use(otelgin.Middleware(art.ServiceName))
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

		payments.RegisterController(authorized)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	err = r.Run(art.Host)
	if err != nil {
		span.RecordError(err)
		log.Fatal(err)
	}
}
