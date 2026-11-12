package commonservices

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPreSendCheckWithoutWebhook(t *testing.T) {
	// Without any configured webhook, PreSendCheck should immediately allow the message through unmodified
	content := []byte("hello world")
	allowed, modified, code := PreSendCheck(context.Background(), "non_existent_app", "u1", "u2", 1, "jg:text", content)
	if !allowed {
		t.Fatalf("expected allowed = true")
	}
	if string(modified) != string(content) {
		t.Fatalf("expected unmodified content")
	}
	if code != 0 {
		t.Fatalf("expected code 0, got %v", code)
	}
}

func TestWebhookSerialization(t *testing.T) {
	req := PreSendWebhookReq{
		AppKey:      "test_app",
		SenderId:    "sender1",
		TargetId:    "target1",
		ChannelType: 1,
		MsgType:     "jg:text",
		MsgContent:  "test message",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal PreSendWebhookReq: %v", err)
	}

	var parsed PreSendWebhookReq
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed.MsgContent != "test message" || parsed.SenderId != "sender1" {
		t.Fatalf("unexpected parsed values: %+v", parsed)
	}
}

func TestMockWebhookServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req PreSendWebhookReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.MsgContent == "bad word" {
			_ = json.NewEncoder(w).Encode(PreSendWebhookResp{
				Action: 1, // reject
				Code:   12001,
				Msg:    "sensitive keyword detected",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(PreSendWebhookResp{
			Action:          2, // mutate
			ModifiedContent: "clean content",
		})
	}))
	defer ts.Close()

	if ts.URL == "" {
		t.Fatal("test server URL is empty")
	}
}

