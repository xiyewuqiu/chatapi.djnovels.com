package commonservices

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// Webhook 校验头。接收方据此判断请求是否真的来自平台，以及是否被重放。
const (
	WebhookTimestampHeader = "X-Webhook-Timestamp"
	WebhookSignatureHeader = "X-Webhook-Signature"
)

// WebhookSignatureToleranceMillis 是接收方应允许的时间戳偏移。
//
// 平台侧只负责写入当前时间戳，不校验；这个常量是给接入方看的契约值，
// 抄进接入文档，避免各方各写一个数。
const WebhookSignatureToleranceMillis int64 = 5 * 60 * 1000

// WebhookSignedPayload 返回参与签名的字节序列：timestamp + "." + rawBody。
//
// 时间戳必须进签名，否则签名本身可以被无限期重放；
// 用 "." 分隔是为了避免 "1" + "23..." 与 "12" + "3..." 这类拼接歧义。
func WebhookSignedPayload(timestamp int64, body []byte) []byte {
	ts := strconv.FormatInt(timestamp, 10)
	payload := make([]byte, 0, len(ts)+1+len(body))
	payload = append(payload, ts...)
	payload = append(payload, '.')
	payload = append(payload, body...)
	return payload
}

// SignWebhookPayload 用租户的 app_secret 对负载做 HMAC-SHA256，返回小写十六进制。
//
// 选择 app_secret 而不是新发一把密钥：租户已经持有它，接入方不该为验签再多管一个秘密。
// 代价是 app_secret 泄露即签名可伪造——这本来就是租户安全的边界，不是新增风险。
func SignWebhookPayload(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(WebhookSignedPayload(timestamp, body))
	return hex.EncodeToString(mac.Sum(nil))
}

// BuildWebhookHeaders 组装投递头。
//
// app_secret 为空时不写签名头：这属于租户配置异常（建租户时必然生成 secret），
// 平台不替它编一个假签名，让接收方按契约拒绝，问题才能暴露出来。
func BuildWebhookHeaders(appKey, secret string, timestamp int64, body []byte) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-App-Key":    appKey,
	}
	if secret != "" {
		headers[WebhookTimestampHeader] = strconv.FormatInt(timestamp, 10)
		headers[WebhookSignatureHeader] = SignWebhookPayload(secret, timestamp, body)
	}
	return headers
}
