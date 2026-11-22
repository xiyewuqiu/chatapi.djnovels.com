package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"im-server/commons/configures"

	"github.com/gin-gonic/gin"
)

func withAdminSecret(t *testing.T, secret string) {
	t.Helper()
	original := configures.Config.AdminSecret
	configures.Config.AdminSecret = secret
	t.Cleanup(func() { configures.Config.AdminSecret = original })
}

// newGuardedRouter 复现生产的路由形状：路由组统一挂 RequireAdminToken。
func newGuardedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v2 := r.Group("/admingateway/v2", RequireAdminToken)
	v2.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"code": 0})
	})
	return r
}

func TestRequireAdminToken(t *testing.T) {
	cases := []struct {
		name       string
		secret     string
		token      string
		useHeader  bool
		wantStatus int
	}{
		{name: "未配置密钥时一律拒绝，不允许反向默认值", secret: "", token: "anything", useHeader: true, wantStatus: http.StatusServiceUnavailable},
		{name: "缺少凭据头", secret: "s3cret", wantStatus: http.StatusUnauthorized},
		{name: "凭据错误", secret: "s3cret", token: "wrong", useHeader: true, wantStatus: http.StatusUnauthorized},
		{name: "凭据是正确值的前缀", secret: "s3cret", token: "s3cre", useHeader: true, wantStatus: http.StatusUnauthorized},
		{name: "凭据正确则放行", secret: "s3cret", token: "s3cret", useHeader: true, wantStatus: http.StatusOK},
		{name: "配置值带空白仍然匹配", secret: "  s3cret  ", token: "s3cret", useHeader: true, wantStatus: http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withAdminSecret(t, tc.secret)
			r := newGuardedRouter()

			req := httptest.NewRequest(http.MethodGet, "/admingateway/v2/ping", nil)
			if tc.useHeader {
				req.Header.Set(AdminTokenHeader, tc.token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("want status %d, got %d, body=%s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// 凭据失败时必须有可读的 msg，运维排查时不至于对着空 body 发呆。
func TestRequireAdminTokenErrorBody(t *testing.T) {
	withAdminSecret(t, "s3cret")
	r := newGuardedRouter()

	req := httptest.NewRequest(http.MethodGet, "/admingateway/v2/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
	if body := w.Body.String(); body == "" {
		t.Fatal("expected a non-empty error body")
	}
}
