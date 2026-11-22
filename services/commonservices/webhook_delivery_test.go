package commonservices

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// seedAppInfo 往缓存里塞一个租户，让投递链路能取到 secret 与 webhook 地址。
// 这些测试串行执行，用 t.Cleanup 保证不污染其他用例。
//
// 注意 lru.Add 的返回值语义：key 已存在时它会覆盖并返回 false，
// 所以这里不用 bool 判成败，改为回读确认。
func seedAppInfo(t *testing.T, appKey, secret string, mutate func(*AppInfo)) {
	t.Helper()
	info := &AppInfo{AppKey: appKey, AppSecret: secret}
	if mutate != nil {
		mutate(info)
	}
	appInfoCache.Add(appKey, info)
	t.Cleanup(func() { appInfoCache.Remove(appKey) })

	got, ok := appInfoCache.Get(appKey)
	if !ok {
		t.Fatalf("failed to seed app info cache for %s", appKey)
	}
	if got.(*AppInfo) != info {
		t.Fatalf("cache returned a different app info instance for %s", appKey)
	}
}

type signedRequest struct {
	timestamp string
	signature string
	appKey    string
	body      []byte
}

func captureSignedRequest(t *testing.T, got chan signedRequest, respBody interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- signedRequest{
			timestamp: r.Header.Get(WebhookTimestampHeader),
			signature: r.Header.Get(WebhookSignatureHeader),
			appKey:    r.Header.Get("X-App-Key"),
			body:      body,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(respBody)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// 断言的是一条真实链路：PreSendCheck → HTTP 投递 → 接收方独立验签。
func TestPreSendCheckDeliversVerifiableSignature(t *testing.T) {
	const appKey = "webhook-sign-presend"
	const secret = "app-secret-abc"

	got := make(chan signedRequest, 1)
	srv := captureSignedRequest(t, got, PreSendWebhookResp{Action: 0})
	seedAppInfo(t, appKey, secret, func(info *AppInfo) {
		info.PreSendWebhookUrl = srv.URL
	})

	allowed, _, code := PreSendCheck(context.Background(), appKey, "u1", "u2", 1, "jg:text", []byte(`{"content":"hi"}`))
	if !allowed || code != 0 {
		t.Fatalf("expected the message to be allowed, got allowed=%v code=%v", allowed, code)
	}

	select {
	case req := <-got:
		assertSignatureVerifies(t, secret, req)
	case <-time.After(2 * time.Second):
		t.Fatal("pre-send webhook was never delivered")
	}
}

// 异步事件投递同样要签名：事件型 webhook 是最容易被伪造来刷业务的一类。
func TestEventNotifyDeliversVerifiableSignature(t *testing.T) {
	const appKey = "webhook-sign-event"
	const secret = "event-secret-xyz"

	got := make(chan signedRequest, 1)
	srv := captureSignedRequest(t, got, map[string]any{"code": 0})
	seedAppInfo(t, appKey, secret, func(info *AppInfo) {
		info.EventWebhookUrl = srv.URL
	})

	EventNotify(appKey, "user_online", map[string]interface{}{"user_id": "u1"})

	select {
	case req := <-got:
		assertSignatureVerifies(t, secret, req)
	case <-time.After(3 * time.Second):
		t.Fatal("event webhook was never delivered")
	}
}

func assertSignatureVerifies(t *testing.T, secret string, req signedRequest) {
	t.Helper()

	if req.timestamp == "" {
		t.Fatalf("missing %s header", WebhookTimestampHeader)
	}
	ts, err := strconv.ParseInt(req.timestamp, 10, 64)
	if err != nil {
		t.Fatalf("%s is not an integer: %q", WebhookTimestampHeader, req.timestamp)
	}
	if req.signature == "" {
		t.Fatalf("missing %s header", WebhookSignatureHeader)
	}

	// 用接收方视角重算，确认签名覆盖的正是实际收到的 body。
	if want := receiverSignature(secret, ts, req.body); req.signature != want {
		t.Fatalf("signature does not verify against delivered body:\n got=%s\nwant=%s\nbody=%s",
			req.signature, want, string(req.body))
	}

	// 时间戳要贴近当前时间，否则接入方的 ±5 分钟窗口会把它当重放拒掉。
	drift := time.Now().UnixMilli() - ts
	if drift < -WebhookSignatureToleranceMillis || drift > WebhookSignatureToleranceMillis {
		t.Fatalf("timestamp drift %dms is outside the documented tolerance", drift)
	}
}
