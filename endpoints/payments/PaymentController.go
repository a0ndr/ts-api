package payments

import (
	"github.com/gin-gonic/gin"
)

func RegisterController(rg *gin.RouterGroup) {
	g := rg.Group("/payments")

	g.POST("/init", InitializePayment)
	g.GET("/status/:id", PaymentStatus)
}
