package admin

import (
	"git.sr.ht/~aondrejcak/ts-api/endpoints/admin/company"
	"github.com/gin-gonic/gin"
)

func RegisterController(r *gin.Engine) {
	g := r.Group("/admin")

	company.RegisterController(g)
}
