package routers

import (
	"im-server/services/admingateway/apis"

	"github.com/gin-gonic/gin"
)

func RouteV2(eng *gin.Engine) {
	LoadAdminWeb(eng)

	// v2 是本地新增的运维接口，能力等同根权限，必须自己挂闸。
	// 上游 console 的 consoleRouters 鉴权只覆盖它自己注册的那批路由，管不到这里。
	v2 := eng.Group("/admingateway/v2", apis.RequireAdminToken)
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

