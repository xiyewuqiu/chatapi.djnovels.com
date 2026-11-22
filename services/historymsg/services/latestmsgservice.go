package services

import (
	"context"
	"fmt"
	"im-server/commons/bases"
	"im-server/commons/caches"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/commons/tools"
	"im-server/services/historymsg/storages"
	"sync"
	"time"
)

var latestMsgCache *caches.LruCache
var latestMsgLocks *tools.SegmentatedLocks

func init() {
	latestMsgCache = caches.NewLruCacheWithAddReadTimeout("latestmsg_cache", 10000, nil, 5*time.Minute, 5*time.Minute)
	latestMsgLocks = tools.NewSegmentatedLocks(128)
}

// LatestMsgItem 是缓存的"会话最近一条消息"。
//
// 它是**被多个 goroutine 共享的指针**：AddHisMsgActor 在 tunny 线程池里并发执行，
// 多个发送会同时拿到同一个 item 调 Update；IsLatestMsg 又会同时读它。
// 所以字段访问必须走锁——go build -race + 并发压测能稳定复现这个竞争
// （Update 的读写与 IsLatestMsg 的读撞在一起）。
type LatestMsgItem struct {
	mu            sync.RWMutex
	key           string
	LatestMsgId   string
	LatestMsgSeq  int64
	LatestMsgTime int64
}

// Update 在锁内比较并更新，保证"取最大 MsgTime"这件事是原子的。
func (item *LatestMsgItem) Update(msg *pbobjs.DownMsg) {
	item.mu.Lock()
	defer item.mu.Unlock()
	if msg.MsgTime > item.LatestMsgTime {
		item.LatestMsgId = msg.MsgId
		item.LatestMsgSeq = msg.MsgSeqNo
		item.LatestMsgTime = msg.MsgTime
	}
}

// IsLatest 在锁内读取，避免读到"MsgId 已更新、MsgTime 还没更新"的中间状态。
func (item *LatestMsgItem) IsLatest(msgId string, msgTime int64) bool {
	item.mu.RLock()
	defer item.mu.RUnlock()
	return msgId == item.LatestMsgId || msgTime > item.LatestMsgTime
}

func getLatestMsgCacheKey(appkey, converId, subChannel string, channelType pbobjs.ChannelType) string {
	return fmt.Sprintf("%s_%s_%s_%d", appkey, converId, subChannel, channelType)
}
func GetLatestMsg(ctx context.Context, converId, subChannel string, channelType pbobjs.ChannelType) *LatestMsgItem {
	appkey := bases.GetAppKeyFromCtx(ctx)
	key := getLatestMsgCacheKey(appkey, converId, subChannel, channelType)
	if val, exist := latestMsgCache.Get(key); exist {
		return val.(*LatestMsgItem)
	} else {
		lock := latestMsgLocks.GetLocks(key)
		lock.Lock()
		defer lock.Unlock()

		if val, exist := latestMsgCache.Get(key); exist {
			return val.(*LatestMsgItem)
		} else {
			item := &LatestMsgItem{
				key:           key,
				LatestMsgId:   "",
				LatestMsgSeq:  0,
				LatestMsgTime: 0,
			}
			if channelType == pbobjs.ChannelType_Private {
				storage := storages.NewPrivateHisMsgStorage()
				latestMsg, err := storage.QryLatestMsg(appkey, converId, subChannel)
				if err == nil && latestMsg != nil {
					item.LatestMsgId = latestMsg.MsgId
					item.LatestMsgSeq = latestMsg.MsgSeqNo
					item.LatestMsgTime = latestMsg.SendTime
				}
			} else if channelType == pbobjs.ChannelType_Group {
				storage := storages.NewGroupHisMsgStorage()
				latestMsg, err := storage.QryLatestMsg(appkey, converId, subChannel)
				if err == nil && latestMsg != nil {
					item.LatestMsgId = latestMsg.MsgId
					item.LatestMsgSeq = latestMsg.MsgSeqNo
					item.LatestMsgTime = latestMsg.SendTime
				}
			} else if channelType == pbobjs.ChannelType_System {
				storage := storages.NewSystemHisMsgStorage()
				latestMsg, err := storage.QryLatestMsg(appkey, converId)
				if err == nil && latestMsg != nil {
					item.LatestMsgId = latestMsg.MsgId
					item.LatestMsgSeq = latestMsg.MsgSeqNo
					item.LatestMsgTime = latestMsg.SendTime
				}
			} else if channelType == pbobjs.ChannelType_GroupCast {
				storage := storages.NewGrpCastHisMsgStorage()
				latestMsg, err := storage.QryLatestMsg(appkey, converId)
				if err == nil && latestMsg != nil {
					item.LatestMsgId = latestMsg.MsgId
					item.LatestMsgSeq = latestMsg.MsgSeqNo
					item.LatestMsgTime = latestMsg.SendTime
				}
			} else if channelType == pbobjs.ChannelType_BroadCast {
				storage := storages.NewBrdCastHisMsgStorage()
				latestMsg, err := storage.QryLatestMsg(appkey, converId)
				if err == nil && latestMsg != nil {
					item.LatestMsgId = latestMsg.MsgId
					item.LatestMsgSeq = latestMsg.MsgSeqNo
					item.LatestMsgTime = latestMsg.SendTime
				}
			}
			latestMsgCache.Add(key, item)
			return item
		}
	}
}

func IsLatestMsg(ctx context.Context, converId, subChannel string, channelType pbobjs.ChannelType, msgId string, msgTime, msgSeq int64) bool {
	latestMsg := GetLatestMsg(ctx, converId, subChannel, channelType)
	// 走方法而不是直接读字段：Update 可能在另一个 goroutine 里同时写。
	return latestMsg.IsLatest(msgId, msgTime)
}
