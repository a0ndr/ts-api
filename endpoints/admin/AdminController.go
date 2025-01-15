package admin

import (
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"github.com/gin-gonic/gin"
	"log"
)

func RegisterController(rg *gin.Engine) {
	art := kernel.LoadConfig()

	g := rg.Group("/admin")
	g.POST("/login", art.JWT.LoginHandler)
	g.Use(func(c *gin.Context) {
		err := art.JWT.MiddlewareInit()
		if err != nil {
			log.Panicf("JWT MiddlewareInit err: %v", err)
		}
	})
	g.Use(art.JWT.MiddlewareFunc())
	//g.GET("/refresh_token", art.JWT.RefreshHandler)
	//g.POST("/logout", art.JWT.LogoutHandler)
}
