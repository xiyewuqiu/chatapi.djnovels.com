package apis

import (
	"net"
	"strings"

	"im-server/commons/errs"
	"im-server/commons/tools"
	"im-server/services/commonservices/logs"

	"github.com/gin-gonic/gin"
)

// IpWhitelist 按 apiGateway.ipWhitelist 限制服务端 API 的来源地址。
//
// 白名单为空表示不启用，保持与上游默认行为一致；非空时只放行列出的地址。
// 这是防火墙之外的应用级兜底：防火墙被误开时，非回环来源仍然进不来。
//
// 为什么取 RemoteAddr 而不是 gin 的 ClientIP()：ClientIP() 会优先看
// X-Forwarded-For，而 65101 这条链路上没有可信代理，信任它可以被伪造请求头
// 直接绕过白名单——安全检查做成能绕过的，比没有更糟。
func IpWhitelist(whitelist []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(whitelist))
	for _, ip := range whitelist {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return func(ctx *gin.Context) { ctx.Next() }
	}

	return func(ctx *gin.Context) {
		host, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
		if err != nil {
			host = ctx.Request.RemoteAddr
		}
		if _, ok := allowed[host]; !ok {
			// 对外统一返回"签名不匹配"：不告诉调用方"你在黑名单里"，
			// 免得被拿来做拓扑探测。真实原因写日志。
			logs.WithContext(ctx).Warnf("apigateway ip not allowed: remote=%s path=%s", ctx.Request.RemoteAddr, ctx.Request.URL.Path)
			tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_SIGNATURE_FAIL)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
