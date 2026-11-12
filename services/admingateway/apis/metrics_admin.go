package apis

import (
	"net/http"
	"time"

	"im-server/commons/dbcommons"
	"im-server/commons/metrics"

	"github.com/gin-gonic/gin"
)

func GetMetricsOverview(ctx *gin.Context) {
	machineStats, err := metrics.GetMachineMetrics()
	if err != nil {
		machineStats = metrics.MachineMetrics{}
	}

	var totalApps int64
	dbcommons.GetDb().Table("apps").Count(&totalApps)

	var totalUsers int64
	dbcommons.GetDb().Table("users").Count(&totalUsers)

	var totalGroups int64
	dbcommons.GetDb().Table("groupinfos").Count(&totalGroups)

	// Calculate messages today
	todayStart := time.Now().Truncate(24 * time.Hour).UnixMilli()
	var todayPrivateMsgs int64
	dbcommons.GetDb().Table("p_hismsgs").Where("send_time >= ?", todayStart).Count(&todayPrivateMsgs)
	var todayGroupMsgs int64
	dbcommons.GetDb().Table("g_hismsgs").Where("send_time >= ?", todayStart).Count(&todayGroupMsgs)

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"summary": gin.H{
				"total_apps":         totalApps,
				"total_users":        totalUsers,
				"total_groups":       totalGroups,
				"today_messages":     todayPrivateMsgs + todayGroupMsgs,
				"today_private_msgs": todayPrivateMsgs,
				"today_group_msgs":   todayGroupMsgs,
				"system_time_unix":   time.Now().Unix(),
			},
			"machine": machineStats,
		},
	})
}

