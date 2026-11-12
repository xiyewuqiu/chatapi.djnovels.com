package apis

import (
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

type BroadcastMsgReq struct {
	SenderId   string `json:"sender_id" binding:"required"`
	MsgType    string `json:"msg_type" binding:"required"`
	MsgContent string `json:"msg_content" binding:"required"`
}

type ForceCreateGroupReq struct {
	GroupId       string   `json:"group_id" binding:"required"`
	GroupName     string   `json:"group_name" binding:"required"`
	GroupOwner    string   `json:"group_owner" binding:"required"`
	GroupPortrait string   `json:"group_portrait"`
	MemberIds     []string `json:"member_ids"`
}

type ForceDissolveGroupReq struct {
	GroupId string `json:"group_id" binding:"required"`
}

type MuteGroupAllReq struct {
	GroupId string `json:"group_id" binding:"required"`
	IsMute  int32  `json:"is_mute"` // 1: mute all, 0: unmute
}

type ForceKickUserReq struct {
	UserId   string `json:"user_id" binding:"required"`
	DeviceId string `json:"device_id"`
}

type SilentMsgReq struct {
	SenderId    string `json:"sender_id" binding:"required"`
	TargetId    string `json:"target_id" binding:"required"`
	ChannelType int32  `json:"channel_type" binding:"required"`
	MsgType     string `json:"msg_type" binding:"required"`
	MsgContent  string `json:"msg_content" binding:"required"`
}

func BroadcastSystemMessage(ctx *gin.Context) {
	var req BroadcastMsgReq
	if err := ctx.BindJSON(&req); err != nil || req.SenderId == "" || req.MsgType == "" || req.MsgContent == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	rpcCtx := services.ToRpcCtx(ctx, req.SenderId)
	flag := msgdefines.SetStoreMsg(0)
	flag = msgdefines.SetCountMsg(flag)

	bases.AsyncRpcCall(rpcCtx, "bc_msg", req.SenderId, &pbobjs.UpMsg{
		MsgType:    req.MsgType,
		MsgContent: []byte(req.MsgContent),
		Flags:      flag,
	})

	commonservices.EventNotify(appKey, "system_broadcast", map[string]interface{}{
		"sender_id": req.SenderId,
		"msg_type":  req.MsgType,
	})

	tools.SuccessHttpResp(ctx, gin.H{"status": "broadcast_queued"})
}

func SendSilentMessage(ctx *gin.Context) {
	var req SilentMsgReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, req.SenderId)
	upMsg := &pbobjs.UpMsg{
		MsgType:    req.MsgType,
		MsgContent: []byte(req.MsgContent),
		Flags:      msgdefines.SetCmdMsg(0), // Command flag: delivered directly without unread counter
	}

	if req.ChannelType == int32(pbobjs.ChannelType_Private) {
		commonservices.AsyncPrivateMsg(rpcCtx, req.SenderId, req.TargetId, upMsg)
	} else if req.ChannelType == int32(pbobjs.ChannelType_Group) {
		commonservices.AsyncGroupMsg(rpcCtx, req.SenderId, req.TargetId, upMsg)
	}

	tools.SuccessHttpResp(ctx, nil)
}

func ForceCreateGroup(ctx *gin.Context) {
	var req ForceCreateGroupReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	memberMap := make(map[string]bool)
	if req.GroupOwner != "" {
		memberMap[req.GroupOwner] = true
	}
	for _, m := range req.MemberIds {
		if m != "" {
			memberMap[m] = true
		}
	}
	allMembers := make([]string, 0, len(memberMap))
	for m := range memberMap {
		allMembers = append(allMembers, m)
	}

	rpcCtx := services.ToRpcCtx(ctx, req.GroupOwner)
	code, _, err := bases.SyncRpcCall(rpcCtx, "g_add_members", req.GroupId, &pbobjs.GroupMembersReq{
		GroupId:       req.GroupId,
		GroupName:     req.GroupName,
		GroupPortrait: req.GroupPortrait,
		MemberIds:     allMembers,
	}, func() proto.Message {
		return &pbobjs.GroupMembersResp{}
	})

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	commonservices.EventNotify(appKey, "group_created", map[string]interface{}{
		"group_id":    req.GroupId,
		"group_name":  req.GroupName,
		"group_owner": req.GroupOwner,
		"members":     allMembers,
	})

	tools.SuccessHttpResp(ctx, gin.H{
		"group_id":   req.GroupId,
		"group_name": req.GroupName,
	})
}

func ForceDissolveGroup(ctx *gin.Context) {
	var req ForceDissolveGroupReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	rpcCtx := services.ToRpcCtx(ctx, "admin")
	code, _, err := bases.SyncRpcCall(rpcCtx, "g_dissolve", req.GroupId, &pbobjs.GroupMembersReq{
		GroupId: req.GroupId,
	}, func() proto.Message {
		return &pbobjs.GroupMembersResp{}
	})

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	commonservices.EventNotify(appKey, "group_dissolved", map[string]interface{}{
		"group_id": req.GroupId,
	})

	tools.SuccessHttpResp(ctx, nil)
}

func MuteGroupAll(ctx *gin.Context) {
	var req MuteGroupAllReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, "admin")
	code, _, err := bases.SyncRpcCall(rpcCtx, "group_mute", req.GroupId, &pbobjs.GroupMuteReq{
		GroupId: req.GroupId,
		IsMute:  req.IsMute,
	}, nil)

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	tools.SuccessHttpResp(ctx, nil)
}

func ForceKickUser(ctx *gin.Context) {
	var req ForceKickUserReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	var deviceIds []string
	if req.DeviceId != "" {
		deviceIds = []string{req.DeviceId}
	}

	code, _, err := bases.SyncRpcCall(rpcCtx, "kick_user", req.UserId, &pbobjs.KickUserReq{
		UserId:    req.UserId,
		DeviceIds: deviceIds,
	}, nil)

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	commonservices.EventNotify(appKey, "user_kicked", map[string]interface{}{
		"user_id":   req.UserId,
		"device_id": req.DeviceId,
	})

	tools.SuccessHttpResp(ctx, nil)
}
