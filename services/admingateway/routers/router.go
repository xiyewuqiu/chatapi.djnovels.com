package routers

import (
	"im-server/services/admingateway/apis"

	"github.com/gin-gonic/gin"
)

func RouteV2(eng *gin.Engine) {
	LoadAdminWeb(eng)

	v2 := eng.Group("/admingateway/v2")
	{
		v2.POST("/apps/create", apis.CreateApp)
		v2.GET("/apps/list", apis.ListApps)
		v2.GET("/apps/detail", apis.GetAppDetail)
		v2.POST("/apps/status", apis.UpdateAppStatus)
		v2.POST("/apps/policies/set", apis.SetAppPolicies)
		v2.GET("/apps/policies/get", apis.GetAppPolicies)

		v2.GET("/metrics/overview", apis.GetMetricsOverview)
		v2.GET("/audit/messages", apis.AuditMessages)
		v2.POST("/audit/messages/recall", apis.AdminRecallMessage)
		v2.GET("/audit/export", apis.ExportMessagesAudit)
	}
}

