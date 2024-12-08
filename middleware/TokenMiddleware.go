package middleware

import (
	"errors"
	"git.sr.ht/~aondrejcak/ts-api/models"
	u "git.sr.ht/~aondrejcak/ts-api/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := u.LoadConfig()
		ctx, span := config.Tracer.Start(c.Request.Context(), "middleware.token")
		defer span.End()

		authHeader := c.GetHeader("X-Api-Key")
		if authHeader == "" {
			u.SpanGinErrf(span, c, 401, "unauthorized: no auth header")
			return
		}

		_, querySpan := config.Tracer.Start(ctx, "middleware.token.query")
		defer querySpan.End()

		hashedToken := u.Sha512(authHeader)

		token := models.Token{}
		res := config.DatabaseClient.WithContext(c.Request.Context()).First(&token, "token_hash = ?", hashedToken)
		if err := res.Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				u.SpanGinErrf(querySpan, c, 401, "unauthorized: invalid token")
				return
			}

			u.SpanGinErrf(querySpan, c, 500, "failed to authorize user: could not query database: %s", err)
			return
		}

		c.Set("token", token)
		c.Next()
	}
}
