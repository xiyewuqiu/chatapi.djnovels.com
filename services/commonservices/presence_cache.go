package commonservices

import (
	"sync"
	"time"
)

type UserPresenceItem struct {
	UserId         string `json:"user_id"`
	Status         string `json:"status"` // online, busy, away, dnd, offline
	CustomStatus   string `json:"custom_status"`
	Platform       string `json:"platform"`
	LastActiveTime int64  `json:"last_active_time"`
	ExpireAt       int64  `json:"expire_at"` // 0: indefinite
}

var (
	presenceStore = sync.Map{} // key: appKey + "_" + userId, value: *UserPresenceItem
)

func getPresenceKey(appKey, userId string) string {
	return appKey + "_" + userId
}

func UpdateUserPresence(appKey, userId, status, customStatus, platform string, expireSec int64) *UserPresenceItem {
	now := time.Now().UnixMilli()
	var expireAt int64 = 0
	if expireSec > 0 {
		expireAt = now + expireSec*1000
	}

	item := &UserPresenceItem{
		UserId:         userId,
		Status:         status,
		CustomStatus:   customStatus,
		Platform:       platform,
		LastActiveTime: now,
		ExpireAt:       expireAt,
	}

	presenceStore.Store(getPresenceKey(appKey, userId), item)
	return item
}

func GetUserPresence(appKey, userId string) *UserPresenceItem {
	key := getPresenceKey(appKey, userId)
	val, ok := presenceStore.Load(key)
	if !ok {
		return &UserPresenceItem{
			UserId:         userId,
			Status:         "offline",
			CustomStatus:   "",
			Platform:       "",
			LastActiveTime: 0,
		}
	}

	item := val.(*UserPresenceItem)
	if item.ExpireAt > 0 && time.Now().UnixMilli() > item.ExpireAt {
		// Custom status expired, downgrade to online or offline
		item.CustomStatus = ""
		item.ExpireAt = 0
	}
	return item
}

