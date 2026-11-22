package apis

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"im-server/commons/bases"
	"im-server/commons/errs"
	"im-server/commons/pbdefines/pbobjs"
	"im-server/commons/tools"
	"im-server/services/apigateway/services"
	"im-server/services/commonservices"
	"im-server/services/commonservices/msgdefines"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
)

const (
	// 关键词按 rune 计长度。UTF-8 下一个汉字占 3 字节，
	// 按字节算会让"两个字"的中文关键词被误判成超限。
	SearchKeywordMinRunes = 2
	SearchKeywordMaxRunes = 32

	// 单次搜索最多回多少条命中。
	SearchDefaultLimit = 20
	SearchMaxLimit     = 50

	// 拉多少条候选再在内存里过滤。
	//
	// 这是"第一步"方案的硬边界：搜索只能覆盖会话最近这么些条消息，
	// 不是全量检索。想搜更早的内容需要上内容索引（第二步），那是另一个工程。
	SearchCandidateLimit = 500

	// 摘要里关键词左右各保留多少字符。
	snippetPadding = 24
	// 关键词没命中时（理论上不该发生）退化成开头多少字。
	snippetFallbackRunes = 48

	// 搜索限流：同一 (appkey,user) 每分钟最多多少次。
	searchRatePerMinute = 20
	searchRateMaxKeys   = 20000
)

// SearchHisMsgsReq 会话内消息搜索请求。
type SearchHisMsgsReq struct {
	UserId      string `json:"user_id"`
	TargetId    string `json:"target_id"`
	ChannelType int    `json:"channel_type"` // 1 单聊 / 2 群聊
	SubChannel  string `json:"sub_channel"`
	Keyword     string `json:"keyword"`
	Limit       int32  `json:"limit"`
}

// SearchHit 是一条命中结果。
//
// 只回片段不回全文：列表里塞全文会让一次搜索返回几百 KB，
// 而且客户端点击后本来就该用 msg_id 去定位原消息。
type SearchHit struct {
	MsgId    string `json:"msg_id"`
	SenderId string `json:"sender_id"`
	MsgTime  int64  `json:"msg_time"`
	MsgType  string `json:"msg_type"`
	Snippet  string `json:"snippet"`
}

// SearchHisMsgs POST /apigateway/hismsgs/search
//
// 为什么在网关层过滤、而不是在数据库里 LIKE：
//  1. msg_body 是 DownMsg 的 protobuf 序列化结果，直接在字节流上 LIKE 会把
//     序列化结构也当成正文匹配，精度不可控；
//  2. 走现有的 qry_hismsgs RPC，可以自动继承那一套可见性规则（群成员、
//     入群前消息不可见、用户清除时间、租户保留期）——另写一条 SQL 就等于
//     再实现一遍这些规则，少一条就是后门；
//  3. 引擎的 actor 入参是 protobuf 消息，现有 QryHisMsgsReq 没有关键词字段，
//     新增字段要重新生成 539KB 的 pb.go，会把跟版 diff 彻底搅乱。
func SearchHisMsgs(ctx *gin.Context) {
	var req SearchHisMsgsReq
	if err := ctx.BindJSON(&req); err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_REQ_BODY_ILLEGAL)
		return
	}
	if req.UserId == "" || req.TargetId == "" {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_MSG_PARAM_ILLEGAL)
		return
	}
	keyword, ok := NormalizeSearchKeyword(req.Keyword)
	if !ok {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_MSG_PARAM_ILLEGAL)
		return
	}

	channelType := pbobjs.ChannelType_Private
	switch req.ChannelType {
	case 1:
		channelType = pbobjs.ChannelType_Private
	case 2:
		channelType = pbobjs.ChannelType_Group
	default:
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_MSG_PARAM_ILLEGAL)
		return
	}

	limit := req.Limit
	if limit <= 0 || limit > SearchMaxLimit {
		limit = SearchDefaultLimit
	}

	appkey := services.GetAppkeyFromCtx(ctx)
	if !allowSearch(appkey+"|"+req.UserId, nowMillis()) {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_RATE_LIMITED)
		return
	}

	converId := commonservices.GetConversationId(req.UserId, req.TargetId, channelType)
	code, resp, err := bases.SyncRpcCall(services.ToRpcCtx(ctx, req.UserId), "qry_hismsgs", converId, &pbobjs.QryHisMsgsReq{
		TargetId:    req.TargetId,
		ChannelType: channelType,
		SubChannel:  req.SubChannel,
		Count:       SearchCandidateLimit,
		Order:       0, // 倒序：从最新往回翻，最近的消息更可能被搜
		MsgTypes:    []string{msgdefines.InnerMsgType_Text},
	}, func() proto.Message {
		return &pbobjs.DownMsgSet{}
	})
	if err != nil {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode_API_INTERNAL_TIMEOUT)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		tools.ErrorHttpResp(ctx, errs.IMErrorCode(code))
		return
	}

	msgSet := resp.(*pbobjs.DownMsgSet)
	hits := make([]*SearchHit, 0, limit)
	for _, msg := range msgSet.Msgs {
		text := ExtractTextContent(msg.MsgContent)
		if text == "" {
			continue
		}
		if !containsFold(text, keyword) {
			continue
		}
		hits = append(hits, &SearchHit{
			MsgId:    msg.MsgId,
			SenderId: msg.SenderId,
			MsgTime:  msg.MsgTime,
			MsgType:  msg.MsgType,
			Snippet:  Snippet(text, keyword),
		})
		if int32(len(hits)) >= limit {
			break
		}
	}

	tools.SuccessHttpResp(ctx, gin.H{
		"hits":            hits,
		"count":           len(hits),
		"scanned":         len(msgSet.Msgs),
		"candidate_limit": SearchCandidateLimit,
	})
}

// NormalizeSearchKeyword 清理并校验关键词。
func NormalizeSearchKeyword(raw string) (string, bool) {
	keyword := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(keyword)
	if n < SearchKeywordMinRunes || n > SearchKeywordMaxRunes {
		return "", false
	}
	return keyword, true
}

// ExtractTextContent 从 jg:text 的 MsgContent 里取出正文。
//
// MsgContent 是 JSON（{"content":"..."}）而不是裸文本：
// 上游 TextMsg 结构定义在 services/commonservices/msgdefines/msgtype.go。
// 解析失败返回空串，让调用方跳过这条，而不是把原始 JSON 当正文展示给用户。
func ExtractTextContent(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return payload.Content
}

// Snippet 截取关键词附近的片段。
func Snippet(text, keyword string) string {
	runes := []rune(text)
	kw := []rune(keyword)
	at := -1
	for i := 0; i+len(kw) <= len(runes); i++ {
		if strings.EqualFold(string(runes[i:i+len(kw)]), keyword) {
			at = i
			break
		}
	}
	if at < 0 {
		return TrimToRunes(text, snippetFallbackRunes)
	}
	start := at - snippetPadding
	if start < 0 {
		start = 0
	}
	end := at + len(kw) + snippetPadding
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}

// TrimToRunes 按字符数截断，超出部分用省略号表示。
func TrimToRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// containsFold 忽略大小写判断是否包含关键词，语义与 ES 的简单包含一致。
func containsFold(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

// ---------------- 进程内限流 ----------------
//
// 引擎没有现成的限流组件：QpsLimitQuota 只是租户配置项，没有执行点。
// 而搜索是"拉几百条再过滤"的重操作，没有闸门等于给数据库装了个放大器。
// 单进程部署下进程内计数够用；多实例部署时要换成共享存储（同业务层已知限制）。

var (
	searchRateMu   sync.Mutex
	searchRateSeen = map[string][]int64{}
)

// nowMillis 抽成变量只为让限流逻辑可测，生产路径就是当前毫秒时间。
var nowMillis = func() int64 { return time.Now().UnixMilli() }

func allowSearch(key string, now int64) bool {
	searchRateMu.Lock()
	defer searchRateMu.Unlock()

	windowStart := now - 60*1000
	previous := searchRateSeen[key]
	kept := make([]int64, 0, len(previous))
	for _, ts := range previous {
		if ts > windowStart {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= searchRatePerMinute {
		searchRateSeen[key] = kept
		return false
	}
	searchRateSeen[key] = append(kept, now)

	// 键数量兜底：限流是尽力而为，超过阈值整体清空好过无限增长。
	if len(searchRateSeen) > searchRateMaxKeys {
		searchRateSeen = map[string][]int64{}
	}
	return true
}
