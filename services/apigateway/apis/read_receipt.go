package apis

import (
	"im-server/commons/bases"
	"im-server/commons/errs"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/commons/tools"
	"im-server/services/apigateway/services"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

type QryReadDetailReq struct {
	TargetId    string `json:"target_id" binding:"required"`
	ChannelType int32  `json:"channel_type" binding:"required"`
	MsgId       string `json:"msg_id" binding:"required"`
	SubChannel  string `json:"sub_channel"`
}

type MarkGrpReadReq struct {
	UserId      string   `json:"user_id" binding:"required"`
	GroupId     string   `json:"group_id" binding:"required"`
	MsgIds      []string `json:"msg_ids" binding:"required"`
	SubChannel  string   `json:"sub_channel"`
}

func QryReadDetail(ctx *gin.Context) {
	var req QryReadDetailReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	rpcCtx := services.ToRpcCtx(ctx, "admin")

	code, respObj, err := bases.SyncRpcCall(rpcCtx, "qry_read_detail", req.TargetId, &pbobjs.QryReadDetailReq{
		TargetId:    req.TargetId,
		ChannelType: pbobjs.ChannelType(req.ChannelType),
		MsgId:       req.MsgId,
		SubChannel:  req.SubChannel,
	}, func() proto.Message {
		return &pbobjs.QryReadDetailResp{}
	})

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	resp, ok := respObj.(*pbobjs.QryReadDetailResp)
	if !ok || resp == nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_DEFAULT)
		return
	}

	readMembers := make([]gin.H, 0)
	for _, m := range resp.ReadMembers {
		readMembers = append(readMembers, gin.H{
			"member_id": m.Member.GetUserId(),
			"nickname":  m.Member.GetNickname(),
			"avatar":    m.Member.GetUserPortrait(),
			"read_time": m.Time,
		})
	}

	unreadMembers := make([]gin.H, 0)
	for _, m := range resp.UnreadMembers {
		unreadMembers = append(unreadMembers, gin.H{
			"member_id": m.Member.GetUserId(),
			"nickname":  m.Member.GetNickname(),
			"avatar":    m.Member.GetUserPortrait(),
		})
	}

	tools.SuccessHttpResp(ctx, gin.H{
		"app_key":        appKey,
		"msg_id":         req.MsgId,
		"read_count":     resp.ReadCount,
		"member_count":   resp.MemberCount,
		"unread_count":   resp.MemberCount - resp.ReadCount,
		"read_members":   readMembers,
		"unread_members": unreadMembers,
	})
}

func MarkGroupMsgRead(ctx *gin.Context) {
	var req MarkGrpReadReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	bases.AsyncRpcCall(services.ToRpcCtx(ctx, req.UserId), "mark_grp_msg_read", req.GroupId, &pbobjs.MarkGrpMsgReadReq{
		GroupId:    req.GroupId,
		MsgIds:     req.MsgIds,
		SubChannel: req.SubChannel,
	})

	tools.SuccessHttpResp(ctx, nil)
}
