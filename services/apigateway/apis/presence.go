package apis

import (
	"sync"
	"time"

	"im-server/commons/bases"
	"im-server/commons/errs"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/commons/tools"
	"im-server/services/apigateway/services"
	"im-server/services/commonservices"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

type SetPresenceReq struct {
	UserId       string `json:"user_id" binding:"required"`
	Status       string `json:"status" binding:"required"` // online, busy, away, dnd, offline
	CustomStatus string `json:"custom_status"`
	Platform     string `json:"platform"`
	ExpireSec    int64  `json:"expire_sec"`
}

type QryPresenceReq struct {
	UserIds []string `json:"user_ids" binding:"required"`
}

type RichPresenceItem struct {
	UserId         string `json:"user_id"`
	IsOnline       bool   `json:"is_online"`
	Status         string `json:"status"`
	CustomStatus   string `json:"custom_status"`
	Platform       string `json:"platform"`
	LastActiveTime int64  `json:"last_active_time"`
}

func SetUserPresence(ctx *gin.Context) {
	var req SetPresenceReq
	if err := ctx.BindJSON(&req); err != nil || req.UserId == "" || req.Status == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	item := commonservices.UpdateUserPresence(appKey, req.UserId, req.Status, req.CustomStatus, req.Platform, req.ExpireSec)

	// Broadcast presence event to webhook
	commonservices.EventNotify(appKey, "user_presence_change", map[string]interface{}{
		"user_id":          req.UserId,
		"status":           item.Status,
		"custom_status":    item.CustomStatus,
		"platform":         item.Platform,
		"last_active_time": item.LastActiveTime,
	})

	tools.SuccessHttpResp(ctx, item)
}

func QryUserPresence(ctx *gin.Context) {
	var req QryPresenceReq
	if err := ctx.BindJSON(&req); err != nil || len(req.UserIds) == 0 {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)

	// Query network socket online status from connectmanager
	networkOnlineMap := sync.Map{}
	groups := bases.GroupTargets("qry_online_status", req.UserIds)
	wg := sync.WaitGroup{}
	for _, ids := range groups {
		wg.Add(1)
		userIds := ids
		go func() {
			defer wg.Done()
			_, resp, err := bases.SyncRpcCall(services.ToRpcCtx(ctx, ""), "qry_online_status", userIds[0], &pbobjs.UserOnlineStatusReq{
				UserIds: userIds,
			}, func() proto.Message {
				return &pbobjs.UserOnlineStatusResp{}
			})
			if err == nil {
				onlineResp, ok := resp.(*pbobjs.UserOnlineStatusResp)
				if ok && len(onlineResp.Items) > 0 {
					for _, item := range onlineResp.Items {
						networkOnlineMap.Store(item.UserId, item.IsOnline)
					}
				}
			}
		}()
	}
	wg.Wait()

	// Aggregate presence cache with connection online status
	items := make([]*RichPresenceItem, 0, len(req.UserIds))
	for _, uid := range req.UserIds {
		cached := commonservices.GetUserPresence(appKey, uid)
		isSocketOnline := false
		if val, exists := networkOnlineMap.Load(uid); exists {
			isSocketOnline = val.(bool)
		}

		finalStatus := cached.Status
		if !isSocketOnline && (finalStatus == "online" || finalStatus == "") {
			finalStatus = "offline"
		} else if isSocketOnline && (finalStatus == "offline" || finalStatus == "") {
			finalStatus = "online"
		}

		lastActive := cached.LastActiveTime
		if lastActive == 0 && isSocketOnline {
			lastActive = time.Now().UnixMilli()
		}

		items = append(items, &RichPresenceItem{
			UserId:         uid,
			IsOnline:       isSocketOnline,
			Status:         finalStatus,
			CustomStatus:   cached.CustomStatus,
			Platform:       cached.Platform,
			LastActiveTime: lastActive,
		})
	}

	tools.SuccessHttpResp(ctx, gin.H{
		"items": items,
	})
}

