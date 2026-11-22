package apis

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"im-server/commons/configures"
	"im-server/commons/logs"

	"github.com/gin-gonic/gin"
)

// 拒绝路径会写一条 warn 日志，而引擎的 logs 包在未初始化时是 nil logger，
// 直接调用会 panic。测试里先把它初始化到临时目录，避免"安全检查因为日志炸了"。
func TestMain(m *testing.M) {
	dir := filepath.Join(os.TempDir(), "im-ipwhitelist-test")
	_ = os.MkdirAll(dir, 0o755)
	configures.Config.Log.LogPath = dir
	configures.Config.Log.LogName = "ipwhitelist-test"
	logs.InitLogs()
	os.Exit(m.Run())
}

func newWhitelistRouter(whitelist []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/probe", IpWhitelist(whitelist), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	return r
}

func probeFrom(r *gin.Engine, remoteAddr, forwardedFor string) int {
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// 白名单留空表示不启用：这是上游默认行为，不能因为新增了配置就默认拦死。
func TestIpWhitelistDisabledWhenEmpty(t *testing.T) {
	r := newWhitelistRouter(nil)
	if got := probeFrom(r, "203.0.113.9:5555", ""); got != http.StatusOK {
		t.Fatalf("白名单为空时应放行，实际 HTTP %d", got)
	}
}

func TestIpWhitelistAllowsListedAddress(t *testing.T) {
	r := newWhitelistRouter([]string{"127.0.0.1", "10.0.0.5"})
	if got := probeFrom(r, "127.0.0.1:4321", ""); got != http.StatusOK {
		t.Fatalf("白名单内的地址应放行，实际 HTTP %d", got)
	}
}

func TestIpWhitelistBlocksOthers(t *testing.T) {
	r := newWhitelistRouter([]string{"127.0.0.1"})
	if got := probeFrom(r, "203.0.113.9:5555", ""); got == http.StatusOK {
		t.Fatal("白名单外的地址必须被拒绝")
	}
}

// 这条是重点：不能信 X-Forwarded-For。信了的话白名单形同虚设——
// 攻击者只需要加一个请求头就能冒充白名单地址。
func TestIpWhitelistIgnoresForwardedFor(t *testing.T) {
	r := newWhitelistRouter([]string{"127.0.0.1"})
	got := probeFrom(r, "203.0.113.9:5555", "127.0.0.1")
	if got == http.StatusOK {
		t.Fatal("伪造 X-Forwarded-For 不能绕过白名单")
	}
}

// 配置里带空白项的容错：写成 " 127.0.0.1 " 也应该生效。
func TestIpWhitelistTrimsWhitespace(t *testing.T) {
	r := newWhitelistRouter([]string{"  127.0.0.1  "})
	if got := probeFrom(r, "127.0.0.1:4321", ""); got != http.StatusOK {
		t.Fatalf("带空白的白名单项应生效，实际 HTTP %d", got)
	}
}
