package middleware

import (
	"errors"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rt := c.MustGet("rt").(*kernel.RequestRuntime)

		rt.StepInto("middleware.token")

		authHeader := c.GetHeader("X-Api-Key")
		if authHeader == "" {
			rt.Ef(401, "unauthorized: no auth header")
			return
		}

		hashedToken := kernel.Sha512(authHeader)

		token := models.Token{}
		res := rt.DB.WithContext(c.Request.Context()).First(&token, "token_hash = ?", hashedToken)
		if err := res.Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				rt.Ef(401, "unauthorized: invalid token")
				return
			}

			rt.Ef(500, "failed to authorize user: could not query database: %s", err)
			return
		}

		rt.Token = &token

		rt.StepBack()
		c.Next()
	}
}
