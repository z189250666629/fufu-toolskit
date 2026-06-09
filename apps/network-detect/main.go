package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fufu/combine"
	"fufu/config"
	"fufu/newapi"
	"fufu/webutil"
)

const (
	defaultPort              = "8080"
	logTypeConsume           = 2
	logTypeError             = 5
	channelStatusEnabled     = 1
	modelStatusWindowSeconds = 10 * 60
	modelStatusCacheTTL      = time.Duration(modelStatusWindowSeconds) * time.Second
	modelTestCooldown        = time.Hour
	modelLogPageSize         = 100
	modelLogMaxRowsPerType   = 1000
)

var rootDir string
var frontendDir string
var combineDir string
var combineApp http.Handler
var combineConfigErr error
var modelCache = struct {
	sync.Mutex
	Value   *ModelStatus
	Expires time.Time
}{}
var testCooldowns sync.Map
var testResults sync.Map

type apiResult struct {
	OK     bool
	Status int
	Error  string
	Data   map[string]any
}
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
	Currency string  `json:"currency"`
}
type SiteStatus struct {
	Site          newapi.PublicSite `json:"site"`
	Groups        []string          `json:"groups"`
	Status        string            `json:"status"`
	RequestCount  int               `json:"requestCount"`
	SuccessCount  int               `json:"successCount"`
	FailureCount  int               `json:"failureCount"`
	SuccessRate   *float64          `json:"successRate"`
	LastSeenAt    int64             `json:"lastSeenAt"`
	LogError      string            `json:"logError,omitempty"`
	ChannelsError string            `json:"channelsError,omitempty"`
	PricingError  string            `json:"pricingError,omitempty"`
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
type ModelStatus struct {
	Configured          bool           `json:"configured"`
	ConfigError         string         `json:"configError,omitempty"`
	GeneratedAt         int64          `json:"generatedAt"`
	ExpiresAt           int64          `json:"expiresAt"`
	WindowSeconds       int            `json:"windowSeconds"`
	RefreshEverySeconds int            `json:"refreshEverySeconds"`
	Sites               []SiteStatus   `json:"sites"`
	Models              []ModelRow     `json:"models"`
	Totals              map[string]int `json:"totals"`
}

type testRecord struct {
	OK            bool   `json:"ok"`
	Status        string `json:"status"`
	Stream        bool   `json:"stream"`
	TestedAt      int64  `json:"testedAt"`
	Message       string `json:"message"`
	NextAllowedAt int64  `json:"nextAllowedAt"`
}

func main() {
	wd, _ := os.Getwd()
	rootDir = wd
	frontendDir = filepath.Join(rootDir, "frontend")
	combineDir = filepath.Join(rootDir, "combine")
	_ = os.MkdirAll(filepath.Join(rootDir, "data"), 0755)
	setupCombine()
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", route)
	fmt.Printf("network-detect Go backend listening on :%s\n", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, mux); err != nil {
		panic(err)
	}
}

func setupCombine() {
	site, err := config.LoadPrimarySite(rootDir)
	if err != nil {
		combineConfigErr = err
		return
	}
	db, err := combine.InitTraceDB(filepath.Join(rootDir, "data", "combine-trace.db"))
	if err != nil {
		combineConfigErr = err
		return
	}
	cfg := combine.Config{Name: site.Name, URL: site.URL, Token: site.Token, UserID: site.UserID, QuotaUnit: site.QuotaUnit}
	combineApp = combine.NewApp(cfg, db)
}

func route(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if x := recover(); x != nil {
			writeJSON(w, 500, map[string]any{"error": "Internal server error"})
		}
	}()
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/") {
		handleAPI(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, 405, map[string]string{"error": "Only GET is supported"})
		return
	}
	if path == "/combine" || path == "/combine/" {
		serveFile(w, r, filepath.Join(combineDir, "index.html"), true)
		return
	}
	serveStatic(w, r, path)
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	if isCombineAPI(r.URL.Path) {
		if combineApp == nil {
			writeJSON(w, 503, map[string]string{"error": "combine is not configured: " + errString(combineConfigErr)})
			return
		}
		combineApp.ServeHTTP(w, r)
		return
	}
	path := r.URL.Path
	switch {
	case path == "/api/health":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	case path == "/api/client":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, 200, map[string]any{"ip": clientIP(r), "serverTime": time.Now().UnixMilli(), "origin": r.Header.Get("Origin"), "userAgent": r.UserAgent()})
	case path == "/api/connectivity/targets":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, 200, map[string]any{"groups": connectivityGroups()})
	case path == "/api/newapi/sites":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		sites, msg := config.LoadManagedSites(rootDir)
		publics := []newapi.PublicSite{}
		for _, s := range sites {
			publics = append(publics, s.Public())
		}
		status := 200
		if msg != "" && len(sites) == 0 {
			status = 500
		}
		writeJSON(w, status, map[string]any{"configured": len(sites) > 0, "error": msg, "sites": publics})
	case path == "/api/newapi/model-status":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		force := r.URL.Query().Get("refresh") == "1"
		status := getModelStatus(force)
		code := 200
		if status.ConfigError != "" && !status.Configured {
			code = 500
		}
		writeJSON(w, code, status)
	case path == "/api/newapi/overview":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		overview := buildOverview(r.URL.Query())
		writeJSON(w, 200, overview)
	case path == "/api/newapi/model-status/test":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		handleModelTest(w, r)
	default:
		writeJSON(w, 404, map[string]string{"error": "API not found"})
	}
}

func isCombineAPI(path string) bool {
	return path == "/api/auth" || path == "/api/session" || path == "/api/search-keys" || path == "/api/merge" || path == "/api/public-merge" || path == "/api/generate" || strings.HasPrefix(path, "/api/merge-status/") || strings.HasPrefix(path, "/api/token/")
}
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeJSON(w, 405, map[string]string{"error": "Only " + method + " is supported"})
		return false
	}
	return true
}
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}
func readJSON(r *http.Request, out any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.UseNumber()
	return dec.Decode(out)
}
func clientIP(r *http.Request) string {
	for _, h := range []string{"Cf-Connecting-Ip", "X-Real-Ip", "X-Forwarded-For"} {
		v := strings.TrimSpace(r.Header.Get(h))
		if v != "" {
			return strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func serveStatic(w http.ResponseWriter, r *http.Request, path string) {
	if path == "/" {
		path = "/index.html"
	}
	file, ok := webutil.SafePath(frontendDir, path)
	if !ok {
		http.Error(w, "Forbidden", 403)
		return
	}
	if _, err := os.Stat(file); err != nil {
		file = filepath.Join(frontendDir, "index.html")
	}
	serveFile(w, r, file, strings.HasSuffix(file, ".html"))
}
func serveFile(w http.ResponseWriter, r *http.Request, file string, noStore bool) {
	webutil.ServeFile(w, r, file, noStore)
}

func env(name string) string { return strings.TrimSpace(os.Getenv(name)) }
func splitList(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' })
	out := []string{}
	for _, p := range parts {
		if u := config.NormalizeBaseURL(p); u != "" {
			out = append(out, u)
		}
	}
	return out
}
func connectivityGroups() []map[string]any {
	if inline := env("CONNECTIVITY_TARGETS"); inline != "" {
		var arr []map[string]any
		if json.Unmarshal([]byte(inline), &arr) == nil {
			return arr
		}
	}
	groups := []map[string]any{}
	if urls := splitList(firstNonEmpty(env("CONNECTIVITY_API_URLS"), env("FUFU_API_URLS"), env("NEWAPI_API_SITE_URL"))); len(urls) > 0 {
		groups = append(groups, map[string]any{"id": "api", "name": firstNonEmpty(env("CONNECTIVITY_API_NAME"), "API 次数站"), "urls": urls})
	}
	if urls := splitList(firstNonEmpty(env("CONNECTIVITY_TOKEN_URLS"), env("FUFU_TOKEN_URLS"), env("NEWAPI_TOKEN_SITE_URL"))); len(urls) > 0 {
		groups = append(groups, map[string]any{"id": "token", "name": firstNonEmpty(env("CONNECTIVITY_TOKEN_NAME"), "Token 站"), "urls": urls})
	}
	if len(groups) > 0 {
		return groups
	}
	return []map[string]any{{"id": "api", "name": "API 次数站", "urls": []string{"https://api.fufuapi.top", "https://api.fufuflower.top"}}, {"id": "token", "name": "Token 站", "urls": []string{"https://token.fufuapi.top", "https://token.fufuflower.top"}}}
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func newAPIGet(ctx context.Context, site newapi.Site, endpoint string, timeout time.Duration) apiResult {
	if !strings.HasPrefix(endpoint, "/api/") {
		return apiResult{OK: false, Status: 400, Error: "不允许的 NewAPI 路径"}
	}
	c := newapi.NewClient(site)
	c.HTTPClient.Timeout = timeout
	res, data, err := c.Get(ctx, endpoint)
	if err != nil {
		return apiResult{OK: false, Status: 0, Error: "NewAPI 请求失败: " + err.Error()}
	}
	if !res.OK() {
		return apiResult{OK: false, Status: res.StatusCode, Error: upstreamError(data, res.StatusCode), Data: data}
	}
	if !newapi.IsSuccess(data) {
		return apiResult{OK: false, Status: res.StatusCode, Error: upstreamError(data, res.StatusCode), Data: data}
	}
	return apiResult{OK: true, Status: res.StatusCode, Data: data}
}
func upstreamError(data map[string]any, status int) string {
	return newapi.ErrorMessage(data, status, "NewAPI 请求失败")
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
func items(data map[string]any) []map[string]any {
	candidates := []any{data["data"], data["items"], data["logs"], data["channels"]}
	if nested, ok := data["data"].(map[string]any); ok {
		candidates = append(candidates, nested["data"], nested["items"], nested["logs"], nested["channels"])
	}
	for _, c := range candidates {
		if arr, ok := c.([]any); ok {
			out := []map[string]any{}
			for _, it := range arr {
				if obj, ok := it.(map[string]any); ok {
					out = append(out, obj)
				}
			}
			return out
		}
	}
	return nil
}
func toInt64(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		n, _ := x.Int64()
		return n
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(x), 10, 64)
		return n
	}
}
func toFloat(v any) float64 {
	switch x := v.(type) {
	case json.Number:
		n, _ := x.Float64()
		return n
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	default:
		n, _ := strconv.ParseFloat(fmt.Sprint(x), 64)
		return n
	}
}
func toInt(v any) int { return int(toInt64(v)) }

func loadSiteLogs(site newapi.Site, typ int, start, end int64) ([]LogRow, string) {
	q := url.Values{}
	q.Set("p", "1")
	q.Set("page", "1")
	q.Set("size", strconv.Itoa(modelLogPageSize))
	q.Set("page_size", strconv.Itoa(modelLogPageSize))
	q.Set("type", strconv.Itoa(typ))
	q.Set("start_timestamp", strconv.FormatInt(start, 10))
	q.Set("end_timestamp", strconv.FormatInt(end, 10))
	paths := []string{"/api/log/self?" + q.Encode(), "/api/log/?" + q.Encode()}
	var last string
	for _, p := range paths {
		res := newAPIGet(context.Background(), site, p, 12*time.Second)
		if res.OK {
			rows := []LogRow{}
			for _, raw := range items(res.Data) {
				rows = append(rows, sanitizeLog(raw))
			}
			if len(rows) > modelLogMaxRowsPerType {
				rows = rows[:modelLogMaxRowsPerType]
			}
			return rows, ""
		}
		last = res.Error
	}
	return nil, last
}
func sanitizeLog(raw map[string]any) LogRow {
	return LogRow{ModelName: firstNonEmpty(str(raw["model_name"]), str(raw["modelName"]), str(raw["model"])), TokenName: firstNonEmpty(str(raw["token_name"]), str(raw["tokenName"]), str(raw["token"])), Group: firstNonEmpty(str(raw["group"]), str(raw["groups"])), RequestID: firstNonEmpty(str(raw["request_id"]), str(raw["requestId"])), Quota: toInt64(raw["quota"]), CreatedAt: firstInt(raw, "created_at", "createdAt", "created_time", "createdTime"), Status: toInt(raw["status"]), Raw: raw}
}
func firstInt(raw map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := raw[k]; ok && str(v) != "" {
			return toInt64(v)
		}
	}
	return 0
}
func loadSiteChannels(site newapi.Site) ([]Channel, string) {
	endpoints := []string{}
	if site.ChannelListEndpoint != "" {
		endpoints = append(endpoints, site.ChannelListEndpoint)
	}
	endpoints = append(endpoints, "/api/channel/search?keyword=&p=1&page_size=500", "/api/channel/?p=1&page_size=500", "/api/channel/search?keyword=&p=0&size=500", "/api/channel/?p=0&size=500")
	var last string
	for _, ep := range endpoints {
		res := newAPIGet(context.Background(), site, ep, 12*time.Second)
		if res.OK {
			out := []Channel{}
			for _, raw := range items(res.Data) {
				out = append(out, sanitizeChannel(raw))
			}
			return out, ""
		}
		last = res.Error
	}
	return nil, last
}
func sanitizeChannel(raw map[string]any) Channel {
	return Channel{ID: toInt(raw["id"]), Name: firstNonEmpty(str(raw["name"]), str(raw["channel_name"])), Status: toInt(raw["status"]), Models: parseList(firstNonEmpty(str(raw["models"]), str(raw["model"]))), Groups: parseList(firstNonEmpty(str(raw["group"]), str(raw["groups"]))), ResponseTime: firstInt(raw, "response_time", "responseTime", "test_time"), Raw: raw}
}
func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &arr) == nil {
		return cleanList(arr)
	}
	return cleanList(strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '|' }))
}
func cleanList(in []string) []string {
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
func loadPricing(site newapi.Site) (map[string]Pricing, string) {
	res := newAPIGet(context.Background(), site, "/api/pricing", 12*time.Second)
	if !res.OK {
		return nil, res.Error
	}
	out := map[string]Pricing{}
	data := res.Data
	for _, raw := range items(data) {
		model := firstNonEmpty(str(raw["model_name"]), str(raw["modelName"]), str(raw["model"]), str(raw["name"]))
		if model == "" {
			continue
		}
		out[model] = pricingFromRaw(site, raw)
	}
	if len(out) == 0 {
		if models, ok := data["data"].(map[string]any); ok {
			for model, val := range models {
				if raw, ok := val.(map[string]any); ok {
					out[model] = pricingFromRaw(site, raw)
				}
			}
		}
	}
	return out, ""
}
func pricingFromRaw(site newapi.Site, raw map[string]any) Pricing {
	input := firstFloat(raw, "input", "prompt", "prompt_ratio", "model_ratio")
	output := firstFloat(raw, "output", "completion", "completion_ratio", "completionRatio")
	if output == 0 {
		output = input
	}
	return Pricing{Input: input * site.RechargeRatio, Output: output * site.RechargeRatio, Currency: site.Currency}
}
func firstFloat(raw map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := raw[k]; ok && str(v) != "" {
			return toFloat(v)
		}
	}
	return 0
}

func getModelStatus(force bool) *ModelStatus {
	now := time.Now()
	modelCache.Lock()
	if !force && modelCache.Value != nil && now.Before(modelCache.Expires) {
		v := modelCache.Value
		modelCache.Unlock()
		return v
	}
	modelCache.Unlock()
	status := buildModelStatus()
	modelCache.Lock()
	modelCache.Value = status
	modelCache.Expires = now.Add(modelStatusCacheTTL)
	modelCache.Unlock()
	return status
}
func buildModelStatus() *ModelStatus {
	sites, msg := config.LoadManagedSites(rootDir)
	now := time.Now().Unix()
	status := &ModelStatus{Configured: len(sites) > 0, ConfigError: msg, GeneratedAt: now, ExpiresAt: now + modelStatusWindowSeconds, WindowSeconds: modelStatusWindowSeconds, RefreshEverySeconds: modelStatusWindowSeconds, Totals: map[string]int{"siteCount": len(sites), "modelCount": 0, "requestCount": 0, "successCount": 0, "failureCount": 0, "operational": 0, "degraded": 0, "down": 0, "unknown": 0}}
	modelRows := map[string]*ModelRow{}
	start := now - modelStatusWindowSeconds
	for _, site := range sites {
		successLogs, logErr := loadSiteLogs(site, logTypeConsume, start, now)
		errorLogs, errErr := loadSiteLogs(site, logTypeError, start, now)
		if logErr == "" {
			logErr = errErr
		}
		channels, chErr := loadSiteChannels(site)
		pricing, priceErr := loadPricing(site)
		groupSet := map[string]bool{}
		modelChannelStats := map[string][]Channel{}
		for _, ch := range channels {
			for _, g := range ch.Groups {
				groupSet[g] = true
			}
			for _, m := range ch.Models {
				modelChannelStats[m] = append(modelChannelStats[m], ch)
			}
		}
		groups := keys(groupSet)
		ss := SiteStatus{Site: site.Public(), Groups: groups, LogError: logErr, ChannelsError: chErr, PricingError: priceErr}
		ss.SuccessCount = len(successLogs)
		ss.FailureCount = len(errorLogs)
		ss.RequestCount = ss.SuccessCount + ss.FailureCount
		ss.SuccessRate = rate(ss.SuccessCount, ss.FailureCount)
		ss.LastSeenAt = maxLogTime(append(successLogs, errorLogs...))
		ss.Status = siteStatusFromCounts(ss.SuccessCount, ss.FailureCount)
		status.Sites = append(status.Sites, ss)
		status.Totals["requestCount"] += ss.RequestCount
		status.Totals["successCount"] += ss.SuccessCount
		status.Totals["failureCount"] += ss.FailureCount
		successByModel := groupLogs(successLogs)
		errorByModel := groupLogs(errorLogs)
		successByMG := groupLogsByModelGroup(successLogs)
		errorByMG := groupLogsByModelGroup(errorLogs)
		for model, chans := range modelChannelStats {
			row := modelRows[model]
			if row == nil {
				row = &ModelRow{Model: model, PerSite: map[string]*ModelCell{}}
				modelRows[model] = row
			}
			cell := buildCell(site.Name, model, chans, successByModel[model], errorByModel[model], pricing[model])
			cell.GroupStats = map[string]*ModelCell{}
			for _, g := range groups {
				groupChans := []Channel{}
				for _, ch := range chans {
					if contains(ch.Groups, g) {
						groupChans = append(groupChans, ch)
					}
				}
				if len(groupChans) == 0 {
					continue
				}
				gs := buildCell(site.Name, model, groupChans, successByMG[model+"\x00"+g], errorByMG[model+"\x00"+g], pricing[model])
				gs.Groups = []string{g}
				cell.GroupStats[g] = gs
			}
			if rec, ok := testResults.Load(site.Name + "\x00" + model); ok {
				cell.ManualTest = rec
			}
			if until, ok := testCooldowns.Load(site.Name + "\x00" + model); ok {
				cell.NextTestAllowedAt = until.(int64)
			}
			row.PerSite[site.Name] = cell
		}
	}
	rows := []ModelRow{}
	for _, row := range modelRows {
		cells := []*ModelCell{}
		for _, c := range row.PerSite {
			cells = append(cells, c)
		}
		row.Status = modelRowStatus(cells)
		for _, c := range cells {
			if c.Configured {
				row.ConfiguredSites++
				if c.Status == "operational" {
					row.OperationalSites++
				}
			}
		}
		rows = append(rows, *row)
		status.Totals[row.Status]++
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Model < rows[j].Model })
	status.Models = rows
	status.Totals["modelCount"] = len(rows)
	return status
}
func buildCell(siteName, model string, chans []Channel, success, errorRows []LogRow, pricing Pricing) *ModelCell {
	enabled := 0
	groups := map[string]bool{}
	for _, ch := range chans {
		if ch.Status == channelStatusEnabled {
			enabled++
		}
		for _, g := range ch.Groups {
			groups[g] = true
		}
	}
	cell := &ModelCell{Configured: true, SiteName: siteName, Model: model, RequestCount: len(success) + len(errorRows), SuccessCount: len(success), FailureCount: len(errorRows), SuccessRate: rate(len(success), len(errorRows)), LastSuccessAt: maxLogTime(success), LastFailureAt: maxLogTime(errorRows), TotalChannelCount: len(chans), EnabledChannelCount: enabled, Groups: keys(groups)}
	cell.LastSeenAt = maxInt64(cell.LastSuccessAt, cell.LastFailureAt)
	cell.Status = statusFromCounts(cell.SuccessCount, cell.FailureCount)
	if pricing.Input != 0 || pricing.Output != 0 {
		p := pricing
		cell.Pricing = &p
	}
	return cell
}
func keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func rate(s, f int) *float64 {
	total := s + f
	if total == 0 {
		return nil
	}
	r := float64(s) / float64(total)
	return &r
}
func statusFromCounts(s, f int) string {
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
func siteStatusFromCounts(s, f int) string { return statusFromCounts(s, f) }
func modelRowStatus(cells []*ModelCell) string {
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
func maxLogTime(rows []LogRow) int64 {
	var m int64
	for _, r := range rows {
		if r.CreatedAt > m {
			m = r.CreatedAt
		}
	}
	return m
}
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func groupLogs(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		if r.ModelName != "" {
			m[r.ModelName] = append(m[r.ModelName], r)
		}
	}
	return m
}
func groupLogsByModelGroup(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		for _, g := range parseList(r.Group) {
			if r.ModelName != "" && g != "" {
				m[r.ModelName+"\x00"+g] = append(m[r.ModelName+"\x00"+g], r)
			}
		}
	}
	return m
}

func handleModelTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteName string `json:"siteName"`
		Model    string `json:"model"`
		Group    string `json:"group"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求体无效"})
		return
	}
	body.SiteName = strings.TrimSpace(body.SiteName)
	body.Model = strings.TrimSpace(body.Model)
	body.Group = strings.TrimSpace(body.Group)
	if body.SiteName == "" || body.Model == "" {
		writeJSON(w, 400, map[string]string{"error": "siteName 和 model 必填"})
		return
	}
	result, err := testModel(body.SiteName, body.Model, body.Group)
	if err != nil {
		var e *httpError
		if errors.As(err, &e) {
			writeJSON(w, e.Status, map[string]any{"error": e.Message, "nextAllowedAt": e.NextAllowedAt})
		} else {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, 200, result)
}

type httpError struct {
	Status        int
	Message       string
	NextAllowedAt int64
}

func (e *httpError) Error() string { return e.Message }
func testModel(siteName, model, group string) (map[string]any, error) {
	sites, _ := config.LoadManagedSites(rootDir)
	var site *newapi.Site
	for i := range sites {
		if sites[i].Name == siteName {
			site = &sites[i]
			break
		}
	}
	if site == nil {
		return nil, &httpError{Status: 404, Message: "站点不存在"}
	}
	key := siteName + "\x00" + model
	now := time.Now().Unix()
	if v, ok := testCooldowns.Load(key); ok && v.(int64) > now {
		return nil, &httpError{Status: 429, Message: "该模型测试仍在冷却中", NextAllowedAt: v.(int64)}
	}
	channels, errMsg := loadSiteChannels(*site)
	if errMsg != "" {
		return nil, &httpError{Status: 502, Message: errMsg}
	}
	candidates := []Channel{}
	for _, ch := range channels {
		if ch.Status == channelStatusEnabled && contains(ch.Models, model) && (group == "" || contains(ch.Groups, group)) {
			candidates = append(candidates, ch)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ResponseTime < candidates[j].ResponseTime })
	if len(candidates) == 0 {
		return nil, &httpError{Status: 400, Message: "当前单元格没有启用通道可测试"}
	}
	next := now + int64(modelTestCooldown/time.Second)
	testCooldowns.Store(key, next)
	stream := supportsStream(model)
	var res apiResult
	for _, ch := range candidates {
		ep := fmt.Sprintf("/api/channel/test/%d?model=%s", ch.ID, url.QueryEscape(model))
		if stream {
			ep += "&stream=true"
		}
		res = newAPIGet(context.Background(), *site, ep, 45*time.Second)
		if res.OK {
			break
		}
	}
	rec := testRecord{OK: res.OK, Status: map[bool]string{true: "operational", false: "down"}[res.OK], Stream: stream, TestedAt: time.Now().Unix(), Message: truncate(testMessage(res), 180), NextAllowedAt: next}
	testResults.Store(key, rec)
	modelCache.Lock()
	if modelCache.Value != nil {
		applyManual(modelCache.Value, siteName, model, rec, next)
	}
	modelCache.Unlock()
	return map[string]any{"siteName": siteName, "model": model, "test": rec}, nil
}
func supportsStream(model string) bool {
	name := strings.ToLower(model)
	return !(strings.Contains(name, "rerank") || strings.Contains(name, "embedding") || strings.Contains(name, "embed") || strings.HasPrefix(name, "m3e") || strings.Contains(name, "bge-") || strings.Contains(name, "seedream"))
}
func testMessage(r apiResult) string {
	if r.OK {
		return "测试通过"
	}
	if r.Error != "" {
		return r.Error
	}
	return "测试失败"
}
func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
func applyManual(ms *ModelStatus, siteName, model string, rec testRecord, next int64) {
	for i := range ms.Models {
		if ms.Models[i].Model == model {
			if c := ms.Models[i].PerSite[siteName]; c != nil {
				c.ManualTest = rec
				c.NextTestAllowedAt = next
				if rec.OK {
					c.SuccessCount++
					c.LastSuccessAt = rec.TestedAt
				} else {
					c.FailureCount++
					c.LastFailureAt = rec.TestedAt
				}
				c.RequestCount = c.SuccessCount + c.FailureCount
				c.SuccessRate = rate(c.SuccessCount, c.FailureCount)
				c.Status = statusFromCounts(c.SuccessCount, c.FailureCount)
			}
		}
	}
}

func buildOverview(q url.Values) map[string]any {
	status := getModelStatus(false)
	return map[string]any{"configured": status.Configured, "configError": status.ConfigError, "generatedAt": status.GeneratedAt, "sites": status.Sites, "totals": status.Totals, "modelAvailability": status.Models, "allLogs": []any{}}
}
