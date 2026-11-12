package apis

import (
	"net/http"
	"strconv"
	"strings"

	"im-server/commons/dbcommons"
	"im-server/commons/pbdefines/pbobjs"

	"github.com/gin-gonic/gin"
)

type AdminRecallReq struct {
	AppKey      string `json:"app_key" binding:"required"`
	ChannelType int    `json:"channel_type" binding:"required"`
	MsgId       string `json:"msg_id" binding:"required"`
	TargetId    string `json:"target_id"`
}

func AuditMessages(ctx *gin.Context) {
	appKey := strings.TrimSpace(ctx.Query("app_key"))
	if appKey == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "app_key required"})
		return
	}

	channelType, _ := strconv.Atoi(ctx.DefaultQuery("channel_type", "1"))
	senderId := ctx.Query("sender_id")
	targetId := ctx.Query("target_id")
	startTime, _ := strconv.ParseInt(ctx.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(ctx.Query("end_time"), 10, 64)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "20"))

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	var tableName string
	if channelType == int(pbobjs.ChannelType_Group) {
		tableName = "g_hismsgs"
	} else {
		tableName = "p_hismsgs"
	}

	query := dbcommons.GetDb().Table(tableName).Where("app_key = ?", appKey)
	if senderId != "" {
		query = query.Where("sender_id = ?", senderId)
	}
	if targetId != "" {
		query = query.Where("target_id = ?", targetId)
	}
	if startTime > 0 {
		query = query.Where("send_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("send_time <= ?", endTime)
	}

	var total int64
	query.Count(&total)

	var list []map[string]interface{}
	err := query.Order("send_time desc").Offset(offset).Limit(size).Find(&list).Error
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "query audit error: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"total": total,
			"page":  page,
			"size":  size,
			"items": list,
		},
	})
}

func AdminRecallMessage(ctx *gin.Context) {
	var req AdminRecallReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid params: " + err.Error()})
		return
	}

	var tableName string
	if req.ChannelType == int(pbobjs.ChannelType_Group) {
		tableName = "g_hismsgs"
	} else {
		tableName = "p_hismsgs"
	}

	err := dbcommons.GetDb().Table(tableName).
		Where("app_key = ? AND msg_id = ?", req.AppKey, req.MsgId).
		Update("msg_body", []byte("[管理员已撤回此违规消息]")).Error

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "recall error: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "message recalled by admin"})
}

func ExportMessagesAudit(ctx *gin.Context) {
	appKey := strings.TrimSpace(ctx.Query("app_key"))
	if appKey == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "app_key required"})
		return
	}

	channelType, _ := strconv.Atoi(ctx.DefaultQuery("channel_type", "1"))
	startTime, _ := strconv.ParseInt(ctx.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(ctx.Query("end_time"), 10, 64)
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "1000"))
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}

	var tableName string
	if channelType == int(pbobjs.ChannelType_Group) {
		tableName = "g_hismsgs"
	} else {
		tableName = "p_hismsgs"
	}

	query := dbcommons.GetDb().Table(tableName).Where("app_key = ?", appKey)
	if startTime > 0 {
		query = query.Where("send_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("send_time <= ?", endTime)
	}

	var list []map[string]interface{}
	err := query.Order("send_time asc").Limit(limit).Find(&list).Error
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "export error: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"app_key":      appKey,
			"channel_type": channelType,
			"exported_count": len(list),
			"items":        list,
		},
	})
}
