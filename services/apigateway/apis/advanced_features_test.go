package apis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGroupAdminValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/groups/admins/set", SetGroupAdmins)

	// Missing member_ids should fail validation
	req, _ := http.NewRequest(http.MethodPost, "/groups/admins/set", strings.NewReader(`{"group_id":"grp1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] == 0 {
		t.Fatalf("expected error for empty member_ids")
	}
}

func TestGroupAnnouncementValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/groups/announcement/set", SetGroupAnnouncement)

	// Missing announcement content
	req, _ := http.NewRequest(http.MethodPost, "/groups/announcement/set", strings.NewReader(`{"group_id":"grp1","publisher_id":"u1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] == 0 {
		t.Fatalf("expected error for empty announcement content")
	}
}

func TestPresenceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/users/presence/set", SetUserPresence)

	// Valid presence setting
	req, _ := http.NewRequest(http.MethodPost, "/users/presence/set", strings.NewReader(`{"user_id":"u_test_99","status":"busy","custom_status":"Meeting"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("appkey", "test_app_key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", resp["code"])
	}
}

