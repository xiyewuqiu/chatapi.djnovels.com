package apis

import (
	"im-server/commons/bases"
	"im-server/commons/errs"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/commons/tools"
	"im-server/services/apigateway/services"
	"im-server/services/commonservices"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

type GroupAdminReq struct {
	GroupId   string   `json:"group_id" binding:"required"`
	MemberIds []string `json:"member_ids" binding:"required"`
}

type TransferGroupOwnerReq struct {
	GroupId    string `json:"group_id" binding:"required"`
	OldOwnerId string `json:"old_owner_id"`
	NewOwnerId string `json:"new_owner_id" binding:"required"`
}

func SetGroupAdmins(ctx *gin.Context) {
	var req GroupAdminReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupId == "" || len(req.MemberIds) == 0 {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	code, _, err := bases.SyncRpcCall(services.ToRpcCtx(ctx, ""), "grp_set_admin", req.GroupId, &pbobjs.GroupMembersReq{
		GroupId:   req.GroupId,
		MemberIds: req.MemberIds,
	}, nil)

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	commonservices.EventNotify(appKey, "group_admin_set", map[string]interface{}{
		"group_id":   req.GroupId,
		"member_ids": req.MemberIds,
	})

	tools.SuccessHttpResp(ctx, nil)
}

func DelGroupAdmins(ctx *gin.Context) {
	var req GroupAdminReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupId == "" || len(req.MemberIds) == 0 {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	code, _, err := bases.SyncRpcCall(services.ToRpcCtx(ctx, ""), "grp_del_admin", req.GroupId, &pbobjs.GroupMembersReq{
		GroupId:   req.GroupId,
		MemberIds: req.MemberIds,
	}, nil)

	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	commonservices.EventNotify(appKey, "group_admin_del", map[string]interface{}{
		"group_id":   req.GroupId,
		"member_ids": req.MemberIds,
	})

	tools.SuccessHttpResp(ctx, nil)
}

func QryGroupAdmins(ctx *gin.Context) {
	groupId := ctx.Query("group_id")
	if groupId == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_PARAM_REQUIRED)
		return
	}

	code, resp, err := bases.SyncRpcCall(services.ToRpcCtx(ctx, ""), "grp_qry_admins", groupId, &pbobjs.GroupMembersReq{
		GroupId: groupId,
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

	adminResp := resp.(*pbobjs.GroupMembersResp)
	adminIds := []string{}
	for _, item := range adminResp.Items {
		adminIds = append(adminIds, item.MemberId)
	}

	tools.SuccessHttpResp(ctx, gin.H{
		"group_id":  groupId,
		"admin_ids": adminIds,
		"count":     len(adminIds),
	})
}

func TransferGroupOwner(ctx *gin.Context) {
	var req TransferGroupOwnerReq
	if err := ctx.BindJSON(&req); err != nil || req.GroupId == "" || req.NewOwnerId == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}

	appKey := bases.GetAppKeyFromCtx(ctx)
	rpcCtx := services.ToRpcCtx(ctx, req.NewOwnerId)

	// Ensure new owner is in group
	bases.AsyncRpcCall(rpcCtx, "g_add_members", req.GroupId, &pbobjs.GroupMembersReq{
		GroupId:   req.GroupId,
		MemberIds: []string{req.NewOwnerId},
	})

	// Set new owner as Admin in groupmembers
	bases.AsyncRpcCall(rpcCtx, "grp_set_admin", req.GroupId, &pbobjs.GroupMembersReq{
		GroupId:   req.GroupId,
		MemberIds: []string{req.NewOwnerId},
	})

	// If old owner exists, degrade old owner to normal user
	if req.OldOwnerId != "" && req.OldOwnerId != req.NewOwnerId {
		bases.AsyncRpcCall(rpcCtx, "grp_del_admin", req.GroupId, &pbobjs.GroupMembersReq{
			GroupId:   req.GroupId,
			MemberIds: []string{req.OldOwnerId},
		})
	}

	commonservices.EventNotify(appKey, "group_owner_transferred", map[string]interface{}{
		"group_id":     req.GroupId,
		"old_owner_id": req.OldOwnerId,
		"new_owner_id": req.NewOwnerId,
	})

	tools.SuccessHttpResp(ctx, gin.H{
		"group_id":     req.GroupId,
		"new_owner_id": req.NewOwnerId,
	})
}

