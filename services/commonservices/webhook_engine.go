package commonservices

import (
	"context"
	"encoding/json"
	"time"

	"im-server/commons/errs"
	"im-server/commons/tools"
	"im-server/services/commonservices/logs"
)

func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// recover from panic in async webhook
			}
		}()
		fn()
	}()
}

type PreSendWebhookReq struct {
	AppKey      string `json:"app_key"`
	SenderId    string `json:"sender_id"`
	TargetId    string `json:"target_id"`
	ChannelType int    `json:"channel_type"`
	MsgType     string `json:"msg_type"`
	MsgContent  string `json:"msg_content"`
}

type PreSendWebhookResp struct {
	Action          int    `json:"action"` // 0: allow, 1: reject, 2: mutate
	Code            int32  `json:"code"`
	Msg             string `json:"msg"`
	ModifiedContent string `json:"modified_content"`
}

type PostSendWebhookReq struct {
	AppKey      string `json:"app_key"`
	MsgId       string `json:"msg_id"`
	SenderId    string `json:"sender_id"`
	TargetId    string `json:"target_id"`
	ChannelType int    `json:"channel_type"`
	MsgType     string `json:"msg_type"`
	SendTime    int64  `json:"send_time"`
}

type EventWebhookReq struct {
	AppKey    string                 `json:"app_key"`
	EventType string                 `json:"event_type"` // user_online, user_offline, group_create, group_dissolve
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// PreSendCheck runs synchronous pre-send webhook inspection if configured
func PreSendCheck(ctx context.Context, appKey, senderId, targetId string, channelType int, msgType string, content []byte) (bool, []byte, errs.IMErrorCode) {
	appInfo, ok := GetAppInfo(appKey)
	if !ok || appInfo == nil || appInfo.PreSendWebhookUrl == "" {
		return true, content, errs.IMErrorCode_SUCCESS
	}

	reqPayload := PreSendWebhookReq{
		AppKey:      appKey,
		SenderId:    senderId,
		TargetId:    targetId,
		ChannelType: channelType,
		MsgType:     msgType,
		MsgContent:  string(content),
	}

	bodyBytes, _ := json.Marshal(reqPayload)
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-App-Key":    appKey,
	}

	respBytes, httpCode, err := tools.HttpDoBytesWithTimeout("POST", appInfo.PreSendWebhookUrl, headers, string(bodyBytes), 500*time.Millisecond)
	if err != nil || httpCode != 200 {
		logs.WithContext(ctx).Warnf("PreSendWebhook call failed url:%s err:%v code:%d", appInfo.PreSendWebhookUrl, err, httpCode)
		// Default to allow on webhook network failure to avoid hard blocking
		return true, content, errs.IMErrorCode_SUCCESS
	}

	var hookResp PreSendWebhookResp
	if err := json.Unmarshal(respBytes, &hookResp); err != nil {
		return true, content, errs.IMErrorCode_SUCCESS
	}

	switch hookResp.Action {
	case 1: // Reject
		errorCode := errs.IMErrorCode_MSG_BLOCK
		if hookResp.Code > 0 {
			errorCode = errs.IMErrorCode(hookResp.Code)
		}
		return false, nil, errorCode
	case 2: // Mutate
		if hookResp.ModifiedContent != "" {
			return true, []byte(hookResp.ModifiedContent), errs.IMErrorCode_SUCCESS
		}
	}

	return true, content, errs.IMErrorCode_SUCCESS
}

// PostSendNotify fires asynchronous webhook after message successfully saved
func PostSendNotify(appKey, msgId, senderId, targetId string, channelType int, msgType string, sendTime int64) {
	appInfo, ok := GetAppInfo(appKey)
	if !ok || appInfo == nil || appInfo.PostSendWebhookUrl == "" {
		return
	}

	reqPayload := PostSendWebhookReq{
		AppKey:      appKey,
		MsgId:       msgId,
		SenderId:    senderId,
		TargetId:    targetId,
		ChannelType: channelType,
		MsgType:     msgType,
		SendTime:    sendTime,
	}

	safeGo(func() {
		bodyBytes, _ := json.Marshal(reqPayload)
		headers := map[string]string{
			"Content-Type": "application/json",
			"X-App-Key":    appKey,
		}
		_, _, _ = tools.HttpDoBytesWithTimeout("POST", appInfo.PostSendWebhookUrl, headers, string(bodyBytes), 2*time.Second)
	})
}

// EventNotify fires asynchronous event webhook
func EventNotify(appKey, eventType string, data map[string]interface{}) {
	appInfo, ok := GetAppInfo(appKey)
	if !ok || appInfo == nil || appInfo.EventWebhookUrl == "" {
		return
	}

	reqPayload := EventWebhookReq{
		AppKey:    appKey,
		EventType: eventType,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}

	safeGo(func() {
		bodyBytes, _ := json.Marshal(reqPayload)
		headers := map[string]string{
			"Content-Type": "application/json",
			"X-App-Key":    appKey,
		}
		_, _, _ = tools.HttpDoBytesWithTimeout("POST", appInfo.EventWebhookUrl, headers, string(bodyBytes), 2*time.Second)
	})
}
