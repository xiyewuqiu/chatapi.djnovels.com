package apis

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"im-server/commons/configures"

	"github.com/gin-gonic/gin"
)

// AdminTokenHeader 是管理台接口的运维凭据头。
const AdminTokenHeader = "X-Admin-Token"

// RequireAdminToken 校验 /admingateway/v2/* 的运维凭据。
//
// 这批接口能建租户、改租户策略、撤回任意消息、导出审计明细，等同于整个平台的根权限，
// 口径与业务层 /jim/internal/* 的 ServiceAuth 保持一致：
//   - adminSecret 留空视为未开通，一律拒绝。不允许"没配就放行"这种反向默认值。
//   - 用常量时间比较，避免逐字节试探出密钥前缀。
//
// 为什么必须单独加：上游 console 自带的口令 + JWT 只挂在 consoleRouters 注册的路由上，
// v2 是本地新增的一组路由，注册时不经过它，天然绕开。
func RequireAdminToken(ctx *gin.Context) {
	expected := strings.TrimSpace(configures.Config.AdminSecret)
	if expected == "" {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"code": 503,
			"msg":  "adminSecret is not configured, /admingateway/v2 is disabled",
		})
		return
	}
	provided := ctx.GetHeader(AdminTokenHeader)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "invalid " + AdminTokenHeader,
		})
		return
	}
	ctx.Next()
}
