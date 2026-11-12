package routers

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed admin.html
var adminDashboardHTML string

func LoadAdminWeb(eng *gin.Engine) {
	eng.GET("/admin", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusOK, adminDashboardHTML)
	})
	eng.GET("/admin/", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusOK, adminDashboardHTML)
	})
}
