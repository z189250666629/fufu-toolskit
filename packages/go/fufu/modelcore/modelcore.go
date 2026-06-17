package modelcore

import (
	"encoding/json"
	"fmt"
	"fufu/rawconv"
	"sort"
	"strings"
)

const (
	ChannelStatusEnabled = 1
	modelManualKeySep    = "\x00"
	modelGroupLogKeySep  = "\x00"
)

type LogRow struct {
	ModelName string
	TokenName string
	Group     string
	RequestID string
	Quota     int64
	CreatedAt int64
	Status    int
	Raw       map[string]any
}

type Channel struct {
	ID           int
	Name         string
	Status       int
	Models       []string
	Groups       []string
	ResponseTime int64
	Raw          map[string]any
}

type Pricing struct {
	Input    float64 `json:"input"`
	Output   float64 `json:"output"`
	Request  float64 `json:"request,omitempty"`
	Type     string  `json:"type,omitempty"`
	Currency string  `json:"currency"`
}

type PricingSite struct {
	Currency      string
	RechargeRatio float64
}

type ModelCell struct {
	Configured          bool                  `json:"configured"`
	SiteName            string                `json:"siteName"`
	Model               string                `json:"model"`
	Status              string                `json:"status"`
	RequestCount        int                   `json:"requestCount"`
	SuccessCount        int                   `json:"successCount"`
	FailureCount        int                   `json:"failureCount"`
	SuccessRate         *float64              `json:"successRate"`
	LastSuccessAt       int64                 `json:"lastSuccessAt"`
	LastFailureAt       int64                 `json:"lastFailureAt"`
	LastSeenAt          int64                 `json:"lastSeenAt"`
	EnabledChannelCount int                   `json:"enabledChannelCount"`
	TotalChannelCount   int                   `json:"totalChannelCount"`
	Groups              []string              `json:"groups"`
	GroupStats          map[string]*ModelCell `json:"groupStats,omitempty"`
	Pricing             *Pricing              `json:"pricing,omitempty"`
	ManualTest          any                   `json:"manualTest,omitempty"`
	NextTestAllowedAt   int64                 `json:"nextTestAllowedAt,omitempty"`
}

type ModelRow struct {
	Model            string                `json:"model"`
	Status           string                `json:"status"`
	OperationalSites int                   `json:"operationalSites"`
	ConfiguredSites  int                   `json:"configuredSites"`
	PerSite          map[string]*ModelCell `json:"perSite"`
}

type ChannelIndex struct {
	Groups          []string
	ChannelsByModel map[string][]Channel
}

func SanitizeLog(raw map[string]any) LogRow {
	return LogRow{
		ModelName: FirstNonEmpty(Str(raw["model_name"]), Str(raw["modelName"]), Str(raw["model"])),
		TokenName: FirstNonEmpty(Str(raw["token_name"]), Str(raw["tokenName"]), Str(raw["token"])),
		Group:     FirstNonEmpty(Str(raw["group"]), Str(raw["groups"])),
		RequestID: FirstNonEmpty(Str(raw["request_id"]), Str(raw["requestId"])),
		Quota:     rawconv.Int64(raw["quota"]),
		CreatedAt: FirstInt(raw, "created_at", "createdAt", "created_time", "createdTime"),
		Status:    rawconv.Int(raw["status"]),
		Raw:       raw,
	}
}

func FirstInt(raw map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := raw[k]; ok && Str(v) != "" {
			return rawconv.Int64(v)
		}
	}
	return 0
}

func SanitizeChannel(raw map[string]any) Channel {
	return Channel{
		ID:           rawconv.Int(raw["id"]),
		Name:         FirstNonEmpty(Str(raw["name"]), Str(raw["channel_name"])),
		Status:       rawconv.Int(raw["status"]),
		Models:       ParseListValue(raw["models"], raw["model"]),
		Groups:       ParseListValue(raw["group"], raw["groups"]),
		ResponseTime: FirstInt(raw, "response_time", "responseTime", "test_time"),
		Raw:          raw,
	}
}

func ParseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &arr) == nil {
		return CleanList(arr)
	}
	return CleanList(strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '|'
	}))
}

func ParseListValue(values ...any) []string {
	for _, value := range values {
		switch x := value.(type) {
		case []string:
			if out := CleanList(x); len(out) > 0 {
				return out
			}
		case []any:
			items := make([]string, 0, len(x))
			for _, item := range x {
				items = append(items, Str(item))
			}
			if out := CleanList(items); len(out) > 0 {
				return out
			}
		default:
			if out := ParseList(Str(value)); len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func CleanList(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range in {
		item = strings.TrimSpace(strings.Trim(item, "\"'"))
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func PricingFromRaw(site PricingSite, raw map[string]any) Pricing {
	input := FirstFloat(raw, "input", "prompt", "prompt_ratio", "model_ratio")
	output := FirstFloat(raw, "output", "completion", "completion_ratio", "completionRatio")
	request := FirstFloat(raw, "request", "model_price", "modelPrice", "request_price", "requestPrice")
	if rawconv.Int(raw["quota_type"]) == 1 || (request != 0 && input == 0 && output == 0) {
		return Pricing{Request: request * site.RechargeRatio, Type: "request", Currency: site.Currency}
	}
	if output == 0 {
		output = input
	}
	return Pricing{Input: input * site.RechargeRatio, Output: output * site.RechargeRatio, Currency: site.Currency}
}

func FirstFloat(raw map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := raw[k]; ok && Str(v) != "" {
			return rawconv.Float64(v)
		}
	}
	return 0
}

func Keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func Contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func Rate(s, f int) *float64 {
	total := s + f
	if total == 0 {
		return nil
	}
	r := float64(s) / float64(total)
	return &r
}

func StatusFromCounts(s, f int) string {
	if s+f == 0 {
		return "unknown"
	}
	r := float64(s) / float64(s+f)
	if r >= 0.8 {
		return "operational"
	}
	if r > 0 {
		return "degraded"
	}
	return "down"
}

func ModelRowStatus(cells []*ModelCell) string {
	configured := 0
	op := 0
	degraded := 0
	down := 0
	for _, c := range cells {
		if c.Configured {
			configured++
			switch c.Status {
			case "operational":
				op++
			case "degraded":
				degraded++
			case "down":
				down++
			}
		}
	}
	if configured == 0 {
		return "unknown"
	}
	if down > 0 {
		return "down"
	}
	if degraded > 0 {
		return "degraded"
	}
	if op > 0 {
		return "operational"
	}
	return "unknown"
}

func RecomputeModelRowSummary(row *ModelRow) {
	row.ConfiguredSites = 0
	row.OperationalSites = 0
	cells := []*ModelCell{}
	for _, c := range row.PerSite {
		cells = append(cells, c)
	}
	row.Status = ModelRowStatus(cells)
	for _, c := range cells {
		if c.Configured {
			row.ConfiguredSites++
			if c.Status == "operational" {
				row.OperationalSites++
			}
		}
	}
}

func MaxLogTime(rows []LogRow) int64 {
	var m int64
	for _, r := range rows {
		if r.CreatedAt > m {
			m = r.CreatedAt
		}
	}
	return m
}

func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func ModelGroupLogKey(model, group string) string {
	return model + modelGroupLogKeySep + group
}

func GroupLogs(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		if r.ModelName != "" {
			m[r.ModelName] = append(m[r.ModelName], r)
		}
	}
	return m
}

func GroupLogsByModelGroup(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		for _, g := range ParseList(r.Group) {
			if r.ModelName != "" && g != "" {
				key := ModelGroupLogKey(r.ModelName, g)
				m[key] = append(m[key], r)
			}
		}
	}
	return m
}

func IndexChannelsForModelStatus(channels []Channel) ChannelIndex {
	groupSet := map[string]bool{}
	channelsByModel := map[string][]Channel{}
	for _, ch := range channels {
		for _, g := range ch.Groups {
			groupSet[g] = true
		}
		for _, m := range ch.Models {
			channelsByModel[m] = append(channelsByModel[m], ch)
		}
	}
	return ChannelIndex{
		Groups:          Keys(groupSet),
		ChannelsByModel: channelsByModel,
	}
}

func SelectModelTestChannels(channels []Channel, model, group string) []Channel {
	candidates := []Channel{}
	for _, ch := range channels {
		if ch.Status == ChannelStatusEnabled && Contains(ch.Models, model) && (group == "" || Contains(ch.Groups, group)) {
			candidates = append(candidates, ch)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := ChannelResponseTimeRank(candidates[i].ResponseTime)
		right := ChannelResponseTimeRank(candidates[j].ResponseTime)
		if left != right {
			return left < right
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates
}

func ChannelResponseTimeRank(responseTime int64) int64 {
	if responseTime <= 0 {
		return 1<<62 - 1
	}
	return responseTime
}

func ModelManualKey(siteName, model, group string) string {
	return siteName + modelManualKeySep + model + modelManualKeySep + group
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func Str(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}
