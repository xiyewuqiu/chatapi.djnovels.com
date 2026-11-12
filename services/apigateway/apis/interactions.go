package apis

import (
	"encoding/json"
	"net/http"
	"strconv"

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

type ReactionReq struct {
	UserId      string `json:"user_id" binding:"required"`
	TargetId    string `json:"target_id" binding:"required"`
	ChannelType int32  `json:"channel_type" binding:"required"`
	MsgId       string `json:"msg_id" binding:"required"`
	Reaction    string `json:"reaction" binding:"required"`
	SubChannel  string `json:"sub_channel"`
}

type QryReactionReq struct {
	TargetId    string   `json:"target_id" binding:"required"`
	ChannelType int32    `json:"channel_type" binding:"required"`
	MsgIds      []string `json:"msg_ids" binding:"required"`
	SubChannel  string   `json:"sub_channel"`
}

type TopMsgReq struct {
	UserId      string `json:"user_id" binding:"required"`
	TargetId    string `json:"target_id" binding:"required"`
	ChannelType int32  `json:"channel_type" binding:"required"`
	MsgId       string `json:"msg_id" binding:"required"`
	SubChannel  string `json:"sub_channel"`
}

type TypingReq struct {
	UserId      string `json:"user_id" binding:"required"`
	TargetId    string `json:"target_id" binding:"required"`
	ChannelType int32  `json:"channel_type" binding:"required"`
	Focus       bool   `json:"focus"`
}

type CreateMergedMsgReq struct {
	UserId      string               `json:"user_id" binding:"required"`
	TargetId    string               `json:"target_id" binding:"required"`
	ChannelType int32                `json:"channel_type" binding:"required"`
	ParentMsgId string               `json:"parent_msg_id" binding:"required"`
	SubChannel  string               `json:"sub_channel"`
	Msgs        []*MergedMsgItemReq `json:"msgs" binding:"required"`
}

type MergedMsgItemReq struct {
	MsgId      string `json:"msg_id"`
	SenderId   string `json:"sender_id"`
	MsgType    string `json:"msg_type"`
	MsgContent string `json:"msg_content"`
	SendTime   int64  `json:"send_time"`
}

type QryMergedMsgReq struct {
	ParentMsgId string `json:"parent_msg_id" binding:"required"`
	StartTime   int64  `json:"start_time"`
	Count       int32  `json:"count"`
	Order       int32  `json:"order"`
}

func AddMsgReaction(ctx *gin.Context) {
	var req ReactionReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	if appInfo, ok := commonservices.GetAppInfo(appKey); ok && appInfo != nil && !appInfo.OpenReaction {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_CONNECT_FUNCTIONDISABLED)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	converId := commonservices.GetConversationId(req.UserId, req.TargetId, pbobjs.ChannelType(req.ChannelType))

	code, _, err := bases.SyncRpcCall(rpcCtx, "msg_exset", converId, &pbobjs.MsgExt{
		MsgId:       req.MsgId,
		TargetId:    req.TargetId,
		ChannelType: pbobjs.ChannelType(req.ChannelType),
		SubChannel:  req.SubChannel,
		Ext: &pbobjs.MsgExtItem{
			Key:   req.Reaction,
			Value: req.UserId,
		},
	}, nil)

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	tools.SuccessHttpResp(ctx, nil)
}

func DelMsgReaction(ctx *gin.Context) {
	var req ReactionReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	converId := commonservices.GetConversationId(req.UserId, req.TargetId, pbobjs.ChannelType(req.ChannelType))

	code, _, err := bases.SyncRpcCall(rpcCtx, "del_msg_exset", converId, &pbobjs.MsgExt{
		MsgId:       req.MsgId,
		TargetId:    req.TargetId,
		ChannelType: pbobjs.ChannelType(req.ChannelType),
		SubChannel:  req.SubChannel,
		Ext: &pbobjs.MsgExtItem{
			Key:   req.Reaction,
			Value: req.UserId,
		},
	}, nil)

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	tools.SuccessHttpResp(ctx, nil)
}

func QryMsgReactions(ctx *gin.Context) {
	var req QryReactionReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, "admin")
	converId := commonservices.GetConversationId("admin", req.TargetId, pbobjs.ChannelType(req.ChannelType))

	code, respObj, err := bases.SyncRpcCall(rpcCtx, "qry_msg_exset", converId, &pbobjs.QryMsgExtReq{
		TargetId:    req.TargetId,
		ChannelType: pbobjs.ChannelType(req.ChannelType),
		SubChannel:  req.SubChannel,
		MsgIds:      req.MsgIds,
	}, func() proto.Message {
		return &pbobjs.MsgExtItemsList{}
	})

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	resp, ok := respObj.(*pbobjs.MsgExtItemsList)
	if !ok || resp == nil {
		tools.SuccessHttpResp(ctx, []gin.H{})
		return
	}

	result := make([]gin.H, 0)
	for _, items := range resp.Items {
		reactionMap := make(map[string][]string)
		for _, item := range items.Exts {
			reactionMap[item.Key] = append(reactionMap[item.Key], item.Value)
		}
		result = append(result, gin.H{
			"msg_id":    items.MsgId,
			"reactions": reactionMap,
		})
	}

	tools.SuccessHttpResp(ctx, result)
}

func SetGroupTopMsg(ctx *gin.Context) {
	var req TopMsgReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	code, _, err := bases.SyncRpcCall(rpcCtx, "set_top_msg", req.TargetId, &pbobjs.TopMsgReq{
		TargetId:    req.TargetId,
		ChannelType: pbobjs.ChannelType(req.ChannelType),
		SubChannel:  req.SubChannel,
		MsgId:       req.MsgId,
	}, nil)

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	tools.SuccessHttpResp(ctx, nil)
}

func DelGroupTopMsg(ctx *gin.Context) {
	var req TopMsgReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	code, _, err := bases.SyncRpcCall(rpcCtx, "del_top_msg", req.TargetId, &pbobjs.TopMsgReq{
		TargetId:    req.TargetId,
		ChannelType: pbobjs.ChannelType(req.ChannelType),
		SubChannel:  req.SubChannel,
		MsgId:       req.MsgId,
	}, nil)

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	tools.SuccessHttpResp(ctx, nil)
}

func GetGroupTopMsg(ctx *gin.Context) {
	targetId := ctx.Query("target_id")
	channelType, _ := strconv.ParseInt(ctx.DefaultQuery("channel_type", "2"), 10, 32)
	if targetId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "target_id required"})
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, "admin")
	code, respObj, err := bases.SyncRpcCall(rpcCtx, "get_top_msg", targetId, &pbobjs.GetTopMsgReq{
		TargetId:    targetId,
		ChannelType: pbobjs.ChannelType(channelType),
	}, func() proto.Message {
		return &pbobjs.TopMsg{}
	})

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	tools.SuccessHttpResp(ctx, respObj)
}

func SendTypingIndicator(ctx *gin.Context) {
	var req TypingReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	payload, _ := json.Marshal(gin.H{
		"sender_id": req.UserId,
		"target_id": req.TargetId,
		"focus":     req.Focus,
	})

	upMsg := &pbobjs.UpMsg{
		MsgType:    msgdefines.CmdMsgType_Typing,
		MsgContent: payload,
		Flags:      msgdefines.SetCmdMsg(0),
	}

	if req.ChannelType == int32(pbobjs.ChannelType_Private) {
		commonservices.AsyncPrivateMsg(rpcCtx, req.UserId, req.TargetId, upMsg)
	} else if req.ChannelType == int32(pbobjs.ChannelType_Group) {
		commonservices.AsyncGroupMsg(rpcCtx, req.UserId, req.TargetId, upMsg)
	}

	tools.SuccessHttpResp(ctx, nil)
}

func CreateMergedMsgs(ctx *gin.Context) {
	var req CreateMergedMsgReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	pbMsgs := make([]*pbobjs.SimpleMsg, 0, len(req.Msgs))
	for _, m := range req.Msgs {
		pbMsgs = append(pbMsgs, &pbobjs.SimpleMsg{
			MsgId:   m.MsgId,
			MsgTime: m.SendTime,
		})
	}

	rpcCtx := services.ToRpcCtx(ctx, req.UserId)
	bases.AsyncRpcCall(rpcCtx, "merge_msgs", req.ParentMsgId, &pbobjs.MergeMsgReq{
		ParentMsgId: req.ParentMsgId,
		MergedMsgs: &pbobjs.MergedMsgs{
			Msgs:        pbMsgs,
			UserId:      req.UserId,
			TargetId:    req.TargetId,
			ChannelType: pbobjs.ChannelType(req.ChannelType),
		},
	})

	tools.SuccessHttpResp(ctx, nil)
}

func QryMergedMsgs(ctx *gin.Context) {
	var req QryMergedMsgReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	if req.Count <= 0 || req.Count > 100 {
		req.Count = 50
	}

	rpcCtx := services.ToRpcCtx(ctx, "admin")
	code, respObj, err := bases.SyncRpcCall(rpcCtx, "qry_merged_msgs", req.ParentMsgId, &pbobjs.QryMergedMsgsReq{
		StartTime: req.StartTime,
		Count:     req.Count,
		Order:     req.Order,
	}, func() proto.Message {
		return &pbobjs.DownMsgSet{}
	})

	if err != nil || code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, code)
		return
	}

	downSet, ok := respObj.(*pbobjs.DownMsgSet)
	if !ok || downSet == nil {
		tools.SuccessHttpResp(ctx, []gin.H{})
		return
	}

	list := make([]gin.H, 0)
	for _, msg := range downSet.Msgs {
		list = append(list, gin.H{
			"msg_id":      msg.MsgId,
			"sender_id":   msg.SenderId,
			"msg_type":    msg.MsgType,
			"msg_content": string(msg.MsgContent),
			"send_time":   msg.MsgTime,
		})
	}

	tools.SuccessHttpResp(ctx, gin.H{
		"parent_msg_id": req.ParentMsgId,
		"msgs":          list,
	})
}
