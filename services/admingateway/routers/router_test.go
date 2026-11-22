package routers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"im-server/commons/configures"
	"im-server/services/admingateway/apis"

	"github.com/gin-gonic/gin"
)

// newV2Router 按生产方式装配：RouteV2 内部会把中间件挂到整个 v2 组上。
func newV2Router(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RouteV2(r)
	return r
}

// 这条断言的是接缝本身：中间件写对了但忘了挂，安全就等于零。
func TestRouteV2RejectsRequestWithoutToken(t *testing.T) {
	original := configures.Config.AdminSecret
	configures.Config.AdminSecret = "s3cret"
	t.Cleanup(func() { configures.Config.AdminSecret = original })

	r := newV2Router(t)

	// 覆盖 v2 组里的代表路由：写操作、读操作、审计导出各一条。
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/admingateway/v2/apps/create"},
		{http.MethodGet, "/admingateway/v2/apps/list"},
		{http.MethodGet, "/admingateway/v2/audit/export"},
		{http.MethodPost, "/admingateway/v2/audit/messages/recall"},
	}

	for _, p := range paths {
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			req := httptest.NewRequest(p.method, p.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("want 401 without token, got %d, body=%s", w.Code, w.Body.String())
			}
		})
	}
}

// 未配置 adminSecret 是明确的运维事故，必须给出可区分的状态码。
func TestRouteV2RejectsWhenSecretUnset(t *testing.T) {
	original := configures.Config.AdminSecret
	configures.Config.AdminSecret = ""
	t.Cleanup(func() { configures.Config.AdminSecret = original })

	r := newV2Router(t)
	req := httptest.NewRequest(http.MethodGet, "/admingateway/v2/metrics/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when adminSecret is empty, got %d", w.Code)
	}
}

// 管理页面本身不含数据，允许匿名打开；数据接口才需要凭据。
func TestAdminDashboardIsReachable(t *testing.T) {
	r := newV2Router(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for /admin, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), apis.AdminTokenHeader) {
		t.Fatal("admin dashboard should prompt for the admin token header")
	}
}
