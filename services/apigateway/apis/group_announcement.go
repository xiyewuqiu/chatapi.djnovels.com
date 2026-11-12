package apis

import (
	"encoding/json"
	"time"

	"im-server/commons/bases"
	"im-server/commons/errs"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/commons/tools"
	"im-server/services/apigateway/services"
	"im-server/services/commonservices"
	"im-server/services/commonservices/msgdefines"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

type SetGroupAnnouncementReq struct {
	GroupId      string `json:"group_id" binding:"required"`
	PublisherId  string `json:"publisher_id" binding:"required"`
	Announcement string `json:"announcement" binding:"required"`
}

type GroupAnnouncementData struct {
	Content       string `json:"content"`
	PublisherId   string `json:"publisher_id"`
	PublishedTime int64  `json:"published_time"`
}

func SetGroupAnnouncement(ctx *gin.Context) {
	var req SetGroupAnnouncementReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupId == "" || req.Announcement == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	now := time.Now().UnixMilli()

	annoData := GroupAnnouncementData{
		Content:       req.Announcement,
		PublisherId:   req.PublisherId,
		PublishedTime: now,
	}
	annoBytes, _ := json.Marshal(annoData)

	// Persist announcement into groupinfoexts via upd_group_info actor
	rpcCtx := services.ToRpcCtx(ctx, req.PublisherId)
	bases.AsyncRpcCall(rpcCtx, "upd_group_info", req.GroupId, &pbobjs.GroupInfo{
		GroupId: req.GroupId,
		ExtFields: []*pbobjs.KvItem{
			{
				Key:     "grp_announcement",
				Value:   string(annoBytes),
				UpdTime: now,
			},
		},
	})

	// Broadcast group notification system message so client UI displays banner immediately
	notifyContent, _ := json.Marshal(map[string]interface{}{
		"type":           "grp_announcement_updated",
		"group_id":       req.GroupId,
		"publisher_id":   req.PublisherId,
		"announcement":   req.Announcement,
		"published_time": now,
	})

	flag := msgdefines.SetStoreMsg(0)
	flag = msgdefines.SetCountMsg(flag)
	commonservices.AsyncGroupMsg(rpcCtx, req.PublisherId, req.GroupId, &pbobjs.UpMsg{
		MsgType:    "jg:grp_ntf",
		MsgContent: notifyContent,
		Flags:      flag,
	})

	commonservices.EventNotify(appKey, "group_announcement_updated", map[string]interface{}{
		"group_id":       req.GroupId,
		"publisher_id":   req.PublisherId,
		"announcement":   req.Announcement,
		"published_time": now,
	})

	tools.SuccessHttpResp(ctx, gin.H{
		"group_id":       req.GroupId,
		"published_time": now,
	})
}

func GetGroupAnnouncement(ctx *gin.Context) {
	groupId := ctx.Query("group_id")
	if groupId == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_PARAM_REQUIRED)
		return
	}

	code, groupInfo, err := bases.SyncRpcCall(services.ToRpcCtx(ctx, ""), "qry_group_info", groupId, &pbobjs.GroupInfoReq{
		GroupId:    groupId,
		CareFields: []string{"grp_announcement"},
	}, func() proto.Message {
		return &pbobjs.GroupInfo{}
	})

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	info := groupInfo.(*pbobjs.GroupInfo)
	var annoData GroupAnnouncementData
	for _, kv := range info.ExtFields {
		if kv.Key == "grp_announcement" {
			_ = json.Unmarshal([]byte(kv.Value), &annoData)
			break
		}
	}

	tools.SuccessHttpResp(ctx, gin.H{
		"group_id":       groupId,
		"announcement":   annoData.Content,
		"publisher_id":   annoData.PublisherId,
		"published_time": annoData.PublishedTime,
	})
}
