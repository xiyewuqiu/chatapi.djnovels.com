package commonservices

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
)

// receiverSignature 按接入方视角独立重算签名，故意不调用被测函数。
// 自己验自己等于没验：只有两份独立实现对上，才说明契约写得死。
func receiverSignature(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestSignWebhookPayloadMatchesReceiverSide(t *testing.T) {
	const secret = "app-secret-abc"
	const ts int64 = 1762900000000
	body := []byte(`{"app_key":"demo","msg_id":"m1","channel_type":2}`)

	got := SignWebhookPayload(secret, ts, body)
	want := receiverSignature(secret, ts, body)
	if got != want {
		t.Fatalf("signature mismatch with receiver-side computation:\n got=%s\nwant=%s", got, want)
	}
}

// 时间戳必须在签名覆盖范围内，否则同一份 body 可以被无限期重放。
func TestSignWebhookPayloadBindsTimestamp(t *testing.T) {
	const secret = "app-secret-abc"
	body := []byte(`{"msg_id":"m1"}`)

	a := SignWebhookPayload(secret, 1762900000000, body)
	b := SignWebhookPayload(secret, 1762900000001, body)
	if a == b {
		t.Fatal("signature must change when timestamp changes")
	}
}

func TestSignWebhookPayloadBindsBodyAndSecret(t *testing.T) {
	const ts int64 = 1762900000000

	base := SignWebhookPayload("secret-1", ts, []byte(`{"msg_id":"m1"}`))
	otherBody := SignWebhookPayload("secret-1", ts, []byte(`{"msg_id":"m2"}`))
	otherSecret := SignWebhookPayload("secret-2", ts, []byte(`{"msg_id":"m1"}`))

	if base == otherBody {
		t.Fatal("signature must change when body changes")
	}
	if base == otherSecret {
		t.Fatal("signature must change when secret changes")
	}
}

// "." 分隔符的意义：避免 (ts="1", body="23") 与 (ts="12", body="3") 撞成同一个负载。
func TestWebhookSignedPayloadAvoidsConcatAmbiguity(t *testing.T) {
	a := string(WebhookSignedPayload(1, []byte("23")))
	b := string(WebhookSignedPayload(12, []byte("3")))
	if a == b {
		t.Fatalf("payload concatenation is ambiguous: %q", a)
	}
	if a != "1.23" {
		t.Fatalf("unexpected payload format: %q", a)
	}
}

func TestBuildWebhookHeadersSignsWhenSecretPresent(t *testing.T) {
	const secret = "app-secret-abc"
	const ts int64 = 1762900000000
	body := []byte(`{"msg_id":"m1"}`)

	headers := BuildWebhookHeaders("demo", secret, ts, body)

	if headers["X-App-Key"] != "demo" {
		t.Fatalf("missing app key header: %v", headers)
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("missing content type: %v", headers)
	}
	if headers[WebhookTimestampHeader] != strconv.FormatInt(ts, 10) {
		t.Fatalf("unexpected timestamp header: %q", headers[WebhookTimestampHeader])
	}
	if want := receiverSignature(secret, ts, body); headers[WebhookSignatureHeader] != want {
		t.Fatalf("signature header mismatch:\n got=%s\nwant=%s", headers[WebhookSignatureHeader], want)
	}
}

// app_secret 为空是租户配置异常。平台不编假签名：让接收方按契约拒绝，问题才会暴露。
func TestBuildWebhookHeadersSkipsSignatureWithoutSecret(t *testing.T) {
	headers := BuildWebhookHeaders("demo", "", 1762900000000, []byte(`{}`))

	if _, ok := headers[WebhookSignatureHeader]; ok {
		t.Fatal("must not emit a signature header when app secret is empty")
	}
	if _, ok := headers[WebhookTimestampHeader]; ok {
		t.Fatal("must not emit a timestamp header when app secret is empty")
	}
	if headers["X-App-Key"] != "demo" {
		t.Fatal("app key header must still be present")
	}
}
