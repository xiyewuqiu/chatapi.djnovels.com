package apis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCreateAppValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/admingateway/v2/apps/create", CreateApp)

	// Case 1: Missing app_name should fail validation with HTTP 400
	req, _ := http.NewRequest(http.MethodPost, "/admingateway/v2/apps/create", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty payload, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if resp["code"].(float64) != 400 {
		t.Fatalf("expected code 400, got %v", resp["code"])
	}
}

func TestSetAppPoliciesValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/admingateway/v2/apps/policies/set", SetAppPolicies)

	// Case 1: Missing app_key should fail validation with HTTP 400
	req, _ := http.NewRequest(http.MethodPost, "/admingateway/v2/apps/policies/set", strings.NewReader(`{"open_read_receipt": true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when app_key is missing, got %d", w.Code)
	}
}

