package actors

import (
	"context"
	"im-server/commons/bases"
	"im-server/commons/errs"
	"im-server/commons/gmicro/actorsystem"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/services/commonservices/logs"
	"im-server/services/group/services"

	"google.golang.org/protobuf/proto"
)

type SetGroupAdminActor struct {
	bases.BaseActor
}

func (actor *SetGroupAdminActor) OnReceive(ctx context.Context, input proto.Message) {
	if req, ok := input.(*pbobjs.GroupMembersReq); ok {
		logs.WithContext(ctx).Infof("group_id:%s\tset_admin\tmember_ids:%v", req.GroupId, req.MemberIds)
		code := services.SetGroupMemberType(ctx, req.GroupId, req.MemberIds, pbobjs.UserType_Admin)
		ack := bases.CreateQueryAckWraper(ctx, code, nil)
		actor.Sender.Tell(ack, actorsystem.NoSender)
	} else {
		ack := bases.CreateQueryAckWraper(ctx, errs.IMErrorCode_PBILLEGAL, nil)
		actor.Sender.Tell(ack, actorsystem.NoSender)
	}
}

func (actor *SetGroupAdminActor) CreateInputObj() proto.Message {
	return &pbobjs.GroupMembersReq{}
}

type DelGroupAdminActor struct {
	bases.BaseActor
}

func (actor *DelGroupAdminActor) OnReceive(ctx context.Context, input proto.Message) {
	if req, ok := input.(*pbobjs.GroupMembersReq); ok {
		logs.WithContext(ctx).Infof("group_id:%s\tdel_admin\tmember_ids:%v", req.GroupId, req.MemberIds)
		code := services.SetGroupMemberType(ctx, req.GroupId, req.MemberIds, pbobjs.UserType_User)
		ack := bases.CreateQueryAckWraper(ctx, code, nil)
		actor.Sender.Tell(ack, actorsystem.NoSender)
	} else {
		ack := bases.CreateQueryAckWraper(ctx, errs.IMErrorCode_PBILLEGAL, nil)
		actor.Sender.Tell(ack, actorsystem.NoSender)
	}
}

func (actor *DelGroupAdminActor) CreateInputObj() proto.Message {
	return &pbobjs.GroupMembersReq{}
}

type QryGroupAdminsActor struct {
	bases.BaseActor
}

func (actor *QryGroupAdminsActor) OnReceive(ctx context.Context, input proto.Message) {
	if req, ok := input.(*pbobjs.GroupMembersReq); ok {
		code, admins := services.QryGroupAdmins(ctx, req.GroupId)
		resp := &pbobjs.GroupMembersResp{
			Items: admins,
		}
		ack := bases.CreateQueryAckWraper(ctx, code, resp)
		actor.Sender.Tell(ack, actorsystem.NoSender)
	} else {
		ack := bases.CreateQueryAckWraper(ctx, errs.IMErrorCode_PBILLEGAL, nil)
		actor.Sender.Tell(ack, actorsystem.NoSender)
	}
}

func (actor *QryGroupAdminsActor) CreateInputObj() proto.Message {
	return &pbobjs.GroupMembersReq{}
}

