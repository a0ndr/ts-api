package kernel

import (
	"context"
	"errors"
	"fmt"
	"github.com/appleboy/gin-jwt/v2"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
	"log"
	"os"
	"sync"
	"time"
)

var (
	once       sync.Once
	appRuntime *AppRuntime
)

type AppRuntime struct {
	Host string

	ServiceName           string
	ServiceVersion        string
	Debug                 bool
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

	Diagnostic *AppDiagnostic

	Context context.Context

	// Admin JWT
	Realm       string
	IdentityKey string
	SecretKey   []byte
	JWT         *jwt.GinJWTMiddleware
}

type appEnvArray map[string]string

func (env *appEnvArray) get(key string) string {
	if val, ok := (*env)[key]; ok {
		return val
	} else {
		return os.Getenv(key)
	}
}

func LoadConfig() *AppRuntime {
	once.Do(func() {
		appEnv := os.Getenv("API_ENV")
		if appEnv == "" {
			appEnv = "development"
		}

		var e *appEnvArray

		if _, err := os.Stat("./.env." + appEnv); !errors.Is(err, os.ErrNotExist) {
			env, err := godotenv.Read(".env." + appEnv)
			e = (*appEnvArray)(&env)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			e = &appEnvArray{}
		}

		appRuntime = &AppRuntime{
			Host:        e.get("API_HOST"),
			DatabaseDSN: e.get("DATABASE_DSN"),

			ServiceName:           e.get("SERVICE_NAME"),
			ServiceVersion:        e.get("SERVICE_VERSION"),
			Debug:                 e.get("DEBUG") == "true",
			DeploymentEnvironment: appEnv,

			JaegerEndpoint:     e.get("JAEGER_ENDPOINT"),
			PrometheusEndpoint: e.get("PROMETHEUS_ENDPOINT"),
			Insecure:           e.get("INSECURE") == "true",

			ClientID:              e.get("TB_API_CLIENT_ID"),
			ClientSecret:          e.get("TB_API_CLIENT_SECRET"),
			TbEnv:                 e.get("TB_API_ENV"),
			TbUrl:                 fmt.Sprintf("https://api.tatrabanka.sk/premium/%s", e.get("TB_API_ENV")),
			RedirectUri:           e.get("TB_REDIRECT_URI"),
			CodeChallenge:         e.get("TB_CODE_CHALLENGE"),
			CodeChallengeVerifier: e.get("TB_CODE_CHALLENGE_VERIFIER"),

			Diagnostic: &AppDiagnostic{
				Tracer: otel.Tracer(e.get("SERVICE_NAME") + "-tracer"),
				Meter:  otel.Meter(e.get("SERVICE_NAME") + "-meter"),
			},

			Realm:       e.get("SEC_JWT_REALM"),
			IdentityKey: e.get("SEC_JWT_IDENTITY_KEY"),
			SecretKey:   []byte(e.get("SEC_JWT_SECRET_KEY")),
		}

		jwt_, err := jwt.New(&jwt.GinJWTMiddleware{
			Realm:       appRuntime.Realm,
			Key:         appRuntime.SecretKey,
			IdentityKey: appRuntime.IdentityKey,
			Timeout:     time.Hour * 24 * 14, // 2 weeks
		})
		if err != nil {
		}
		appRuntime.JWT = jwt_
	})
	return appRuntime
}
