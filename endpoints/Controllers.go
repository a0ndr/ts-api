package endpoints

import (
	"git.sr.ht/~aondrejcak/ts-api/endpoints/admin"
	"github.com/gin-gonic/gin"
)

func RegisterControllers(r *gin.Engine) {
	admin.RegisterController(r)
}
