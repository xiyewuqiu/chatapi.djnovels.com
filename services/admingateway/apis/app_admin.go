package apis

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"im-server/commons/dbcommons"
	"im-server/commons/tools"
	"im-server/services/commonservices"
	"im-server/services/commonservices/dbs"

	"github.com/gin-gonic/gin"
)

type CreateAppReq struct {
	AppName      string            `json:"app_name" binding:"required"`
	AppKey       string            `json:"app_key"`
	AppSecret    string            `json:"app_secret"`
	AppSecureKey string            `json:"app_secure_key"`
	Policies     map[string]string `json:"policies"`
}

type SetAppPoliciesReq struct {
	AppKey             string `json:"app_key" binding:"required"`
	OpenReadReceipt    *bool  `json:"open_read_receipt"`
	OpenReaction       *bool  `json:"open_reaction"`
	RecallTimeLimitSec *int   `json:"recall_time_limit_sec"`
	MultiDeviceMode    *int   `json:"multi_device_mode"`
	SensitiveMode      *int   `json:"sensitive_mode"`
	PreSendWebhookUrl  string `json:"pre_send_webhook_url"`
	PostSendWebhookUrl string `json:"post_send_webhook_url"`
	EventWebhookUrl    string `json:"event_webhook_url"`
	MaxUsersQuota      *int   `json:"max_users_quota"`
	MaxGroupsQuota     *int   `json:"max_groups_quota"`
	QpsLimitQuota      *int   `json:"qps_limit_quota"`
}

type UpdateAppStatusReq struct {
	AppKey    string `json:"app_key" binding:"required"`
	AppStatus int    `json:"app_status"`
}

func CreateApp(ctx *gin.Context) {
	var req CreateAppReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid params: " + err.Error()})
		return
	}

	appKey := strings.TrimSpace(req.AppKey)
	if appKey == "" {
		appKey = strings.ToLower(tools.RandStr(16))
	}
	appSecret := strings.TrimSpace(req.AppSecret)
	if appSecret == "" {
		appSecret = tools.RandStr(32)
	}
	appSecureKey := strings.TrimSpace(req.AppSecureKey)
	if appSecureKey == "" {
		appSecureKey = tools.RandStr(32)
	}

	appDao := dbs.AppInfoDao{}
	existing := appDao.FindByAppkey(appKey)
	if existing != nil {
		ctx.JSON(http.StatusConflict, gin.H{"code": 409, "msg": "app_key already exists"})
		return
	}

	now := time.Now()
	newApp := dbs.AppInfoDao{
		AppName:      req.AppName,
		AppKey:       appKey,
		AppSecret:    appSecret,
		AppSecureKey: appSecureKey,
		AppStatus:    int(dbs.AppStatus_Normal),
		AppType:      int(dbs.AppType_Private),
		CreatedTime:  now,
		UpdatedTime:  now,
	}

	if err := appDao.Create(newApp); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to create app: " + err.Error()})
		return
	}

	// Apply default policies
	extDao := dbs.AppExtDao{}
	defaultPolicies := map[string]string{
		"open_read_receipt":      "true",
		"open_reaction":          "true",
		"recall_time_limit_sec":  "120",
		"multi_device_mode":      "3",
		"sensitive_mode":         "1",
		"pre_send_webhook_url":   "",
		"post_send_webhook_url":  "",
		"event_webhook_url":      "",
		"max_users_quota":        "100000",
		"max_groups_quota":       "10000",
		"qps_limit_quota":        "2000",
	}

	for k, v := range req.Policies {
		defaultPolicies[strings.ToLower(k)] = v
	}

	for k, v := range defaultPolicies {
		_ = extDao.CreateOrUpdate(appKey, k, v)
	}

	commonservices.InvalidateAppInfoCache(appKey)

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"app_name":       req.AppName,
			"app_key":        appKey,
			"app_secret":     appSecret,
			"app_secure_key": appSecureKey,
			"created_time":   now,
			"policies":       defaultPolicies,
		},
	})
}

func ListApps(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	offset := (page - 1) * size

	var total int64
	dbcommons.GetDb().Table("apps").Count(&total)

	var appList []*dbs.AppInfoDao
	err := dbcommons.GetDb().Table("apps").Order("id desc").Offset(offset).Limit(size).Find(&appList).Error
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "query apps error: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"total": total,
			"page":  page,
			"size":  size,
			"items": appList,
		},
	})
}

func GetAppDetail(ctx *gin.Context) {
	appKey := ctx.Query("app_key")
	if appKey == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "app_key required"})
		return
	}

	appDao := dbs.AppInfoDao{}
	app := appDao.FindByAppkey(appKey)
	if app == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "app not found"})
		return
	}

	extDao := dbs.AppExtDao{}
	extList := extDao.FindListByAppkey(appKey)
	extMap := make(map[string]string)
	for _, item := range extList {
		extMap[item.AppItemKey] = item.AppItemValue
	}

	// Also get user count for this app
	var userCount int64
	dbcommons.GetDb().Table("users").Where("app_key = ?", appKey).Count(&userCount)

	// Also get group count for this app
	var groupCount int64
	dbcommons.GetDb().Table("groupinfos").Where("app_key = ?", appKey).Count(&groupCount)

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"app":         app,
			"policies":    extMap,
			"user_count":  userCount,
			"group_count": groupCount,
		},
	})
}

func UpdateAppStatus(ctx *gin.Context) {
	var req UpdateAppStatusReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid params: " + err.Error()})
		return
	}

	err := dbcommons.GetDb().Table("apps").Where("app_key = ?", req.AppKey).Update("app_status", req.AppStatus).Error
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to update status: " + err.Error()})
		return
	}

	commonservices.InvalidateAppInfoCache(req.AppKey)

	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func SetAppPolicies(ctx *gin.Context) {
	var req SetAppPoliciesReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid params: " + err.Error()})
		return
	}

	appDao := dbs.AppInfoDao{}
	if appDao.FindByAppkey(req.AppKey) == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "app not found"})
		return
	}

	extDao := dbs.AppExtDao{}
	saveBool := func(k string, v *bool) {
		if v != nil {
			_ = extDao.CreateOrUpdate(req.AppKey, k, fmt.Sprintf("%v", *v))
		}
	}
	saveInt := func(k string, v *int) {
		if v != nil {
			_ = extDao.CreateOrUpdate(req.AppKey, k, fmt.Sprintf("%d", *v))
		}
	}
	saveStr := func(k string, v string) {
		if v != "" {
			_ = extDao.CreateOrUpdate(req.AppKey, k, v)
		}
	}

	saveBool("open_read_receipt", req.OpenReadReceipt)
	saveBool("open_reaction", req.OpenReaction)
	saveInt("recall_time_limit_sec", req.RecallTimeLimitSec)
	saveInt("multi_device_mode", req.MultiDeviceMode)
	saveInt("sensitive_mode", req.SensitiveMode)
	saveStr("pre_send_webhook_url", req.PreSendWebhookUrl)
	saveStr("post_send_webhook_url", req.PostSendWebhookUrl)
	saveStr("event_webhook_url", req.EventWebhookUrl)
	saveInt("max_users_quota", req.MaxUsersQuota)
	saveInt("max_groups_quota", req.MaxGroupsQuota)
	saveInt("qps_limit_quota", req.QpsLimitQuota)

	commonservices.InvalidateAppInfoCache(req.AppKey)

	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

func GetAppPolicies(ctx *gin.Context) {
	appKey := ctx.Query("app_key")
	if appKey == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "app_key required"})
		return
	}

	appInfo, ok := commonservices.GetAppInfo(appKey)
	if !ok || appInfo == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "app not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{
			"app_key":               appKey,
			"open_read_receipt":     appInfo.OpenReadReceipt,
			"open_reaction":         appInfo.OpenReaction,
			"recall_time_limit_sec": appInfo.RecallTimeLimitSec,
			"multi_device_mode":     appInfo.MultiDeviceMode,
			"sensitive_mode":        appInfo.SensitiveMode,
			"pre_send_webhook_url":  appInfo.PreSendWebhookUrl,
			"post_send_webhook_url": appInfo.PostSendWebhookUrl,
			"event_webhook_url":     appInfo.EventWebhookUrl,
			"max_users_quota":       appInfo.MaxUsersQuota,
			"max_groups_quota":      appInfo.MaxGroupsQuota,
			"qps_limit_quota":       appInfo.QpsLimitQuota,
		},
	})
}

