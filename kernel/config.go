package kernel

import (
	"context"
	"errors"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/matthewhartstonge/argon2"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
	"log"
	"os"
	"reflect"
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

type login struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func authenticator() func(c *gin.Context) (interface{}, error) {
	return func(c *gin.Context) (interface{}, error) {
		rt := c.MustGet("rt").(*RequestRuntime)
		rt.StepInto("authenticator")
		var loginVals login
		if err := c.ShouldBind(&loginVals); err != nil {
			rt.StepBack()
			return "", errors.New("missing email or password")
		}
		email := loginVals.Email
		password := loginVals.Password

		admin := models.Admin{}
		found, err := rt.First(&admin, "email = ?", email)
		if !found || err != nil {
			rt.StepBack()
			return "", errors.New("invalid email or password")
		}
		if ok, err := argon2.VerifyEncoded([]byte(password), []byte(admin.PasswordHash)); err != nil || !ok {
			rt.StepBackWithMessage("pwd hash not valid")
			return "", errors.New("invalid email or password")
		}
		rt.StepBack()
		return &admin, nil
	}
}

func identityHandler() func(c *gin.Context) interface{} {
	return func(c *gin.Context) interface{} {
		claims := jwt.ExtractClaims(c)
		rt := c.MustGet("rt").(*RequestRuntime)
		admin := &models.Admin{}
		found, err := rt.First(&admin, "email = ?", claims["identityKey"])
		if !found {
			rt.Ef(401, "unauthorized: admin not found")
			return nil
		}
		if err != nil {
			rt.Ef(500, "internal server error: failed to query database")
			return nil
		}
		return admin
	}
}

func payloadFunc() func(data interface{}) jwt.MapClaims {
	return func(data interface{}) jwt.MapClaims {
		if v, ok := data.(*models.Admin); ok {
			return jwt.MapClaims{
				"identityKey": v.Email,
				"fullName":    v.FullName,
				"role":        v.Role,
			}
		}
		return jwt.MapClaims{}
	}
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
			Realm:           appRuntime.Realm,
			Key:             appRuntime.SecretKey,
			IdentityKey:     appRuntime.IdentityKey,
			Timeout:         time.Hour * 24 * 14, // 2 weeks
			PayloadFunc:     payloadFunc(),
			IdentityHandler: identityHandler(),
			Authenticator:   authenticator(),
		})
		if err != nil {
		}
		appRuntime.JWT = jwt_
	})
	return appRuntime
}

func (art *AppRuntime) PrintConfig() {
	v := reflect.ValueOf(*art)
	typeOfS := v.Type()

	fmt.Println("=== CONFIG DATA")
	for i := 0; i < v.NumField(); i++ {
		fmt.Printf(" | %s : %v\n", typeOfS.Field(i).Name, v.Field(i).Interface())
	}
}
