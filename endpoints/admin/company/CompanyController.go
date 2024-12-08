package company

import (
	"git.sr.ht/~aondrejcak/ts-api/endpoints/admin/company/rest"
	"github.com/gin-gonic/gin"
)

func RegisterController(r *gin.RouterGroup) {
	g := r.Group("/company")

	g.POST("", rest.CreateCompany)
}
