package utils

import (
	"context"
	"fmt"
	"github.com/appleboy/gin-jwt/v2"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"time"
)

var (
	once      sync.Once
	appConfig *AppConfig
)

type AppConfig struct {
	Host                   string
	PrometheusExporterHost string

	ServiceName           string
	ServiceVersion        string
	DeploymentEnvironment string

	DatabaseDSN    string
	DatabaseClient *gorm.DB

	JaegerEndpoint     string
	PrometheusEndpoint string
	Insecure           bool

	ClientID              string
	ClientSecret          string
	TbEnv                 string
	TbUrl                 string
	RedirectUri           string
	CodeChallenge         string
	CodeChallengeVerifier string

	Tracer trace.Tracer
	Meter  metric.Meter

	RequestCounter metric.Int64Counter
	ErrorCounter   metric.Int64Counter

	Context context.Context

	// Admin JWT
	Realm       string
	IdentityKey string
	SecretKey   []byte
	JWT         *jwt.GinJWTMiddleware
}

func LoadConfig() *AppConfig {
	once.Do(func() {
		appEnv := os.Getenv("API_ENV")
		if appEnv == "" {
			appEnv = "development"
		}

		var env map[string]string
		env, err := godotenv.Read(".env." + appEnv)
		if err != nil {
			log.Fatal(err)
		}

		appConfig = &AppConfig{
			Host:                   env["HOST"],
			PrometheusExporterHost: env["PROM_EXPORTER_HOST"],
			DatabaseDSN:            env["DATABASE_DSN"],

			ServiceName:           env["SERVICE_NAME"],
			ServiceVersion:        env["SERVICE_VERSION"],
			DeploymentEnvironment: env["DEPLO_ENV"],

			JaegerEndpoint:     env["JAEGER_ENDPOINT"],
			PrometheusEndpoint: env["PROMETHEUS_ENDPOINT"],
			Insecure:           env["INSECURE"] == "true",

			ClientID:              env["TB_API_CLIENT_ID"],
			ClientSecret:          env["TB_API_CLIENT_SECRET"],
			TbEnv:                 env["TB_API_ENV"],
			TbUrl:                 fmt.Sprintf("https://api.tatrabanka.sk/premium/%s", env["TB_API_ENV"]),
			RedirectUri:           env["TB_REDIRECT_URI"],
			CodeChallenge:         env["TB_CODE_CHALLENGE"],
			CodeChallengeVerifier: env["TB_CODE_CHALLENGE_VERIFIER"],

			Tracer: otel.Tracer(env["SERVICE_NAME"] + "-tracer"),
			Meter:  otel.Meter(env["SERVICE_NAME"] + "-meter"),

			Realm:       env["SEC_JWT_REALM"],
			IdentityKey: env["SEC_JWT_IDENTITY_KEY"],
			SecretKey:   []byte(env["SEC_JWT_SECRET_KEY"]),
		}

		appConfig.JWT, err = jwt.New(&jwt.GinJWTMiddleware{
			Realm:       appConfig.Realm,
			Key:         appConfig.SecretKey,
			IdentityKey: appConfig.IdentityKey,
			Timeout:     time.Hour * 24 * 14, // 2 weeks
		})
	})
	return appConfig
}
