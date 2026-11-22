package apis

import (
	"encoding/json"
	"sort"
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

	// 拉多少条候选再在内存里过滤（单页大小）。
	//
	// 不建内容索引的代价就在这里：只能靠翻页覆盖时间范围。
	SearchCandidateLimit = 500

	// 最多往回翻几页。5 × 500 = 2500 条，覆盖时间窗内的绝大部分会话；
	// 翻满还没到底时响应里 truncated=true，如实告诉调用方"还有更早的没扫"。
	searchMaxBatches = 5

	// 搜索时间窗：缺省 7 天（与默认保留期一致），上限 30 天。
	searchDefaultWindowMillis int64 = 7 * 24 * 60 * 60 * 1000
	searchMaxWindowMillis     int64 = 30 * 24 * 60 * 60 * 1000

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
	// FromTime / ToTime 是 Unix 毫秒的搜索窗口，留空表示最近 7 天。
	FromTime int64 `json:"from_time"`
	ToTime   int64 `json:"to_time"`
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
	rpcCtx := services.ToRpcCtx(ctx, req.UserId)

	from, to := normalizeSearchWindow(req.FromTime, req.ToTime, nowMillis())

	// 分批往回翻页。单次 RPC 只回一页，想覆盖更长的时间范围只能翻页——
	// 这是"不建内容索引"的代价：用扫描量换存储结构的稳定。
	//
	// 翻页游标用 MsgTime（服务端会尽量对齐 send_time），但**不能假设它与排序严格一致**：
	// 历史数据里 MsgTime 可能全部相同（例如导入的消息），此时游标会原地不动、
	// 每页返回同一批数据。所以这里同时做 msg_id 去重 + 游标停滞检测，
	// 保证最坏情况下也只是少扫，不会重复扫 5 遍。
	seen := make(map[string]struct{}, SearchCandidateLimit*searchMaxBatches)
	collected := make([]*pbobjs.DownMsg, 0, SearchCandidateLimit)
	cursor := to
	lastCursor := int64(-1)
	truncated := false
	for batch := 0; batch < searchMaxBatches; batch++ {
		code, resp, err := bases.SyncRpcCall(rpcCtx, "qry_hismsgs", converId, &pbobjs.QryHisMsgsReq{
			TargetId:    req.TargetId,
			ChannelType: channelType,
			SubChannel:  req.SubChannel,
			StartTime:   cursor,
			Count:       SearchCandidateLimit,
			Order:       0, // 倒序：从新往旧翻
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

		page := resp.(*pbobjs.DownMsgSet).Msgs
		added := 0
		for _, msg := range page {
			if _, dup := seen[msg.MsgId]; dup {
				continue
			}
			seen[msg.MsgId] = struct{}{}
			collected = append(collected, msg)
			added++
		}
		// 整页都是重复的，或根本没有数据 —— 再翻也是原地打转。
		if len(page) < SearchCandidateLimit || added == 0 {
			break
		}
		// DAO 会把倒序结果重新排成升序，所以这一页里最老的是 page[0]。
		oldest := page[0].MsgTime
		if from > 0 && oldest <= from {
			break // 已翻到时间窗下界
		}
		next := oldest - 1 // 减一毫秒，避免下一页把边界那条重复算进来
		if next >= cursor || next == lastCursor {
			break // 游标不前进，停止，避免空转
		}
		lastCursor = cursor
		cursor = next
		if batch == searchMaxBatches-1 {
			truncated = true // 还有更早的没扫，如实告诉调用方
		}
	}

	// collected 是按"每页升序 + 页序由新到旧"拼起来的，最终结果统一按时间倒序输出。
	sort.SliceStable(collected, func(i, j int) bool {
		return collected[i].MsgTime > collected[j].MsgTime
	})

	hits := make([]*SearchHit, 0, limit)
	for _, msg := range collected {
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

	result := gin.H{
		"hits":            hits,
		"count":           len(hits),
		"scanned":         len(collected),
		"truncated":       truncated,
		"candidate_limit": SearchCandidateLimit * searchMaxBatches,
		"window_from":     from,
		"window_to":       to,
	}
	// truncated=true 表示"更早的还没扫"。这里直接给出下一次该用的 to_time，
	// 客户端拿它再发一次请求就能继续往前搜——等价于无限翻页，
	// 而服务端每次请求的扫描量仍然有界。
	if truncated && len(collected) > 0 {
		result["next_to_time"] = collected[len(collected)-1].MsgTime - 1
	}
	tools.SuccessHttpResp(ctx, result)
}

// normalizeSearchWindow 钳制搜索时间窗。
//
// 缺省 7 天、上限 30 天：与租户保留期口径一致，也避免一次请求把整库扫一遍。
// to 不允许超过当前时间，防止用未来时间窗绕出"全部消息"。
func normalizeSearchWindow(from, to, now int64) (int64, int64) {
	if to <= 0 || to > now {
		to = now
	}
	if from <= 0 || from > to {
		from = to - searchDefaultWindowMillis
	}
	if to-from > searchMaxWindowMillis {
		from = to - searchMaxWindowMillis
	}
	return from, to
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
