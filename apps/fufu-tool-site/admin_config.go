package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	activityapp "fufu-act"
	"fufu/activity"
	"fufu/auth"
	"fufu/config"
	"fufu/newapi"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const toolConfigFileName = "tool-config.json"
const adminSessionCookieName = "fufu_admin_session"

const adminSessionTTL = 12 * time.Hour

var unifiedConfig *toolConfigStore

type ToolConfig struct {
	NewAPI   NewAPIAdminConfig `json:"newapi"`
	Activity activity.Config   `json:"activity"`
	MCY      MCYAdminConfig    `json:"mcy"`
}

type NewAPIAdminConfig struct {
	Sites []ManagedAPISiteConfig `json:"sites"`
}

// MCYAdminConfig is the MCY shop login + endpoints, configured in the admin panel
// and stored in tool-config.db. The activity module reads it at runtime to query
// 补卡 stock and upload cards.
type MCYAdminConfig struct {
	BaseURL        string `json:"baseUrl"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	Cookie         string `json:"cookie,omitempty"`
	LoginEndpoint  string `json:"loginEndpoint,omitempty"`
	UploadEndpoint string `json:"uploadEndpoint,omitempty"`
}

// ManagedSiteURL is one base_url line of a site, with an optional display name
// shown on the homepage nav.
type ManagedSiteURL struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
}

// ManagedAPISiteConfig is one site: a single access token shared across one or
// more base_urls. URL mirrors URLs[0] for backward compatibility with readers
// and legacy single-url payloads/configs.
type ManagedAPISiteConfig struct {
	Name                string           `json:"name"`
	Category            string           `json:"category,omitempty"`
	URLs                []ManagedSiteURL `json:"urls"`
	URL                 string           `json:"url,omitempty"`
	Token               string           `json:"token,omitempty"`
	UserID              string           `json:"userId"`
	Kind                string           `json:"kind,omitempty"`
	SkipUserHeader      bool             `json:"skipUserHeader,omitempty"`
	QuotaUnit           int64            `json:"quotaUnit"`
	Currency            string           `json:"currency"`
	RechargeRatio       float64          `json:"rechargeRatio"`
	ChannelListEndpoint string           `json:"channelListEndpoint,omitempty"`
	Note                string           `json:"note,omitempty"`
}

type adminConfigPatch struct {
	NewAPI *struct {
		Sites []ManagedAPISiteConfig `json:"sites"`
	} `json:"newapi"`
	Activity *activity.Config `json:"activity"`
	MCY      *MCYAdminConfig  `json:"mcy"`
}

type toolConfigStore struct {
	mu   sync.RWMutex
	path string
	db   *sql.DB
	cfg  ToolConfig
}

func newToolConfigStore(path string) *toolConfigStore {
	return &toolConfigStore{path: path}
}

// Load opens the SQLite config database and resolves the active configuration.
// The database is the source of truth: once seeded, environment variables are
// ignored. On first boot the store seeds itself from the legacy tool-config.json
// (migrating existing deployments) or, failing that, from environment defaults,
// then persists the result so future redeploys no longer depend on env.
func (s *toolConfigStore) Load(root string) error {
	db, err := openToolConfigDB(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.db = db
	s.mu.Unlock()

	stored, ok, err := readToolConfigRow(db)
	if err != nil {
		return fmt.Errorf("read config database: %w", err)
	}

	cfg := defaultToolConfig(root)
	migratedFromFile := false
	legacyPath := filepath.Join(root, "data", toolConfigFileName)
	if ok {
		// Decode into a clean Sites slice so json never reuses a default site's
		// backing struct (fields absent in the stored JSON, e.g. urls, would
		// otherwise leak across entries). The default Sites only seed fresh boots.
		cfg.NewAPI.Sites = nil
		if err := json.Unmarshal(stored, &cfg); err != nil {
			return fmt.Errorf("%s 不是有效 JSON: %w", toolConfigDBName, err)
		}
	} else if raw, readErr := os.ReadFile(legacyPath); readErr == nil {
		cfg.NewAPI.Sites = nil
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("%s 不是有效 JSON: %w", toolConfigFileName, err)
		}
		migratedFromFile = true
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("%s 读取失败: %w", toolConfigFileName, readErr)
	}

	cfg, err = normalizeToolConfig(cfg, ToolConfig{})
	if err != nil {
		return err
	}

	if !ok {
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := writeToolConfigRow(db, data); err != nil {
			return fmt.Errorf("seed config database: %w", err)
		}
		if migratedFromFile {
			// Retain the migrated file as a backup but stop reading it.
			_ = os.Rename(legacyPath, legacyPath+".migrated")
		}
	}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	applyToolConfigSnapshot(cfg)
	return nil
}

// Close releases the underlying database handle.
func (s *toolConfigStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *toolConfigStore) Snapshot() ToolConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneToolConfig(s.cfg)
}

func (s *toolConfigStore) ManagedSites() []newapi.Site {
	cfg := s.Snapshot()
	sites := make([]newapi.Site, 0, len(cfg.NewAPI.Sites))
	for _, site := range cfg.NewAPI.Sites {
		sites = append(sites, site.toNewAPISites()...)
	}
	return sites
}

func (s *toolConfigStore) SavePatch(patch adminConfigPatch) (ToolConfig, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneToolConfig(s.cfg)
	newAPIChanged := false
	if patch.NewAPI != nil {
		sites, err := normalizeManagedAPISiteConfigs(patch.NewAPI.Sites, next.NewAPI.Sites)
		if err != nil {
			return ToolConfig{}, false, err
		}
		next.NewAPI.Sites = sites
		newAPIChanged = true
	}
	if patch.Activity != nil {
		next.Activity = activity.CloneConfig(*patch.Activity)
	}
	if patch.MCY != nil {
		next.MCY = *patch.MCY
	}
	normalized, err := normalizeToolConfig(next, s.cfg)
	if err != nil {
		return ToolConfig{}, false, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return ToolConfig{}, false, err
	}
	if s.db == nil {
		return ToolConfig{}, false, errors.New("config database is not initialized")
	}
	if err := writeToolConfigRow(s.db, data); err != nil {
		return ToolConfig{}, false, err
	}
	s.cfg = normalized
	return cloneToolConfig(normalized), newAPIChanged, nil
}

func defaultToolConfig(root string) ToolConfig {
	sites, _ := config.LoadManagedSites(root)
	return ToolConfig{
		NewAPI:   NewAPIAdminConfig{Sites: managedSiteConfigsFromSites(sites)},
		Activity: activity.DefaultConfig(),
		MCY:      mcyConfigFromEnv(),
	}
}

// mcyConfigFromEnv seeds the MCY admin config from environment variables on the
// first boot; after that the database is the source of truth.
func mcyConfigFromEnv() MCYAdminConfig {
	return MCYAdminConfig{
		BaseURL:        firstNonEmptyEnv("MCY_BASE_URL", "SHOP_BASE_URL"),
		Username:       firstNonEmptyEnv("MCY_USERNAME", "SHOP_USERNAME"),
		Password:       firstNonEmptyEnv("MCY_PASSWORD", "SHOP_PASSWORD"),
		Cookie:         os.Getenv("MCY_COOKIE"),
		LoginEndpoint:  os.Getenv("MCY_LOGIN_ENDPOINT"),
		UploadEndpoint: os.Getenv("MCY_UPLOAD_ENDPOINT"),
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func normalizeToolConfig(cfg ToolConfig, previous ToolConfig) (ToolConfig, error) {
	sites, err := normalizeManagedAPISiteConfigs(cfg.NewAPI.Sites, previous.NewAPI.Sites)
	if err != nil {
		return ToolConfig{}, err
	}
	cfg.NewAPI.Sites = sites
	cfg.Activity = activity.CloneConfig(cfg.Activity)
	cfg.MCY = normalizeMCYConfig(cfg.MCY, previous.MCY)
	return cfg, nil
}

// normalizeMCYConfig trims the MCY config and, like site tokens, keeps the
// previous password when the submitted one is blank (the UI never re-sends the
// masked password).
// normalizeMCYBaseURL reduces a shop URL to scheme://host and forces https. The
// MCY encrypted-POST protocol can't survive an http→https 301 (Go turns the POST
// into a GET and drops the body), and the admin URL users paste usually carries
// a path (…/admin) that would otherwise be appended to the login endpoint
// (…/admin/admin/login). So keep only scheme+host, upgrade http→https, and
// default a scheme-less host to https.
func normalizeMCYBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	return "https://" + u.Host
}

func normalizeMCYConfig(c, previous MCYAdminConfig) MCYAdminConfig {
	c.BaseURL = normalizeMCYBaseURL(c.BaseURL)
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.Cookie = strings.TrimSpace(c.Cookie)
	c.LoginEndpoint = strings.TrimSpace(c.LoginEndpoint)
	c.UploadEndpoint = strings.TrimSpace(c.UploadEndpoint)
	if c.Password == "" {
		c.Password = strings.TrimSpace(previous.Password)
	}
	return c
}

func cloneToolConfig(cfg ToolConfig) ToolConfig {
	out := ToolConfig{Activity: activity.CloneConfig(cfg.Activity), MCY: cfg.MCY}
	out.NewAPI.Sites = make([]ManagedAPISiteConfig, len(cfg.NewAPI.Sites))
	for i, site := range cfg.NewAPI.Sites {
		site.URLs = append([]ManagedSiteURL(nil), site.URLs...)
		out.NewAPI.Sites[i] = site
	}
	return out
}

// managedSiteConfigsFromSites groups flat newapi.Site entries (e.g. from env
// seed) into the grouped one-token-per-site config: sites sharing a
// (category, token) collapse into one record with a url per line.
func managedSiteConfigsFromSites(sites []newapi.Site) []ManagedAPISiteConfig {
	out := []ManagedAPISiteConfig{}
	index := map[string]int{}
	for _, site := range sites {
		category := strings.ToLower(strings.TrimSpace(site.Category))
		if category == "" {
			if strings.Contains(strings.ToLower(site.Name), "token") {
				category = "token"
			} else {
				category = "api"
			}
		}
		lineName := site.LineName
		if strings.TrimSpace(lineName) == "" {
			lineName = site.Name
		}
		url := ManagedSiteURL{Name: lineName, URL: site.URL}
		key := category + "\x00" + site.Token
		if at, ok := index[key]; ok {
			out[at].URLs = append(out[at].URLs, url)
			out[at].URL = out[at].URLs[0].URL
			continue
		}
		index[key] = len(out)
		out = append(out, ManagedAPISiteConfig{
			Name:                site.Name,
			Category:            category,
			URLs:                []ManagedSiteURL{url},
			URL:                 site.URL,
			Token:               site.Token,
			UserID:              site.UserID,
			Kind:                site.Kind,
			SkipUserHeader:      site.SkipUserHeader,
			QuotaUnit:           site.QuotaUnit,
			Currency:            site.Currency,
			RechargeRatio:       site.RechargeRatio,
			ChannelListEndpoint: site.ChannelListEndpoint,
			Note:                site.Note,
		})
	}
	return out
}

func normalizeManagedAPISiteConfigs(sites, previous []ManagedAPISiteConfig) ([]ManagedAPISiteConfig, error) {
	normalized := []ManagedAPISiteConfig{}
	for i, site := range sites {
		site.Name = strings.TrimSpace(site.Name)
		site.Token = strings.TrimSpace(site.Token)
		site.UserID = strings.TrimSpace(site.UserID)
		site.Kind = strings.TrimSpace(site.Kind)
		site.Currency = strings.TrimSpace(site.Currency)
		site.ChannelListEndpoint = strings.TrimSpace(site.ChannelListEndpoint)
		site.Note = strings.TrimSpace(site.Note)
		site.Category = strings.ToLower(strings.TrimSpace(site.Category))
		if site.Category == "" {
			if strings.Contains(strings.ToLower(site.Name), "token") {
				site.Category = "token"
			} else {
				site.Category = "api"
			}
		}
		if site.Category != "api" && site.Category != "token" {
			return nil, fmt.Errorf("第 %d 个站点类别不支持（只能 api 或 token）: %s", i+1, site.Category)
		}

		// A site holds one token across many base_urls. Accept the legacy singular
		// "url" by folding it in, then normalize the url list.
		urls := normalizeManagedSiteURLs(site.URLs, site.URL)
		if len(urls) == 0 {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点至少需要一个 base_url", i+1)
		}
		site.URLs = urls
		site.URL = urls[0].URL // mirror primary for backward-compatible readers

		if site.Token == "" {
			site.Token = matchingSiteToken(site, previous)
		}
		if site.Name == "" {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点缺少名称", i+1)
		}
		if site.Token == "" {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点缺少 token", i+1)
		}
		if site.UserID == "" {
			site.UserID = "1"
		}
		if site.Kind == "" {
			site.Kind = "api"
		}
		if !isSupportedAdminSiteKind(site.Kind) {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点 kind 不支持: %s", i+1, site.Kind)
		}
		if site.QuotaUnit <= 0 {
			site.QuotaUnit = newapi.DefaultQuotaUnit
		}
		if site.Currency == "" {
			site.Currency = "$"
		}
		if site.RechargeRatio <= 0 {
			site.RechargeRatio = 1
		}
		normalized = append(normalized, site)
	}

	// Collapse legacy per-line records (and any same-token duplicates) into one
	// site per (category, token): the urls merge, the single token stays.
	merged := mergeManagedSiteConfigsByToken(normalized)

	seen := map[string]bool{}
	for _, site := range merged {
		// Line/site names only need to be unique within a category — "线路 1"
		// may exist under both 次数站 and token 站.
		nameKey := site.Category + "\x00" + site.Name
		if seen[nameKey] {
			return nil, fmt.Errorf("%s 类站点名称重复: %s", site.Category, site.Name)
		}
		seen[nameKey] = true
	}
	return merged, nil
}

// normalizeManagedSiteURLs cleans a site's url list: it folds in the legacy
// singular url, drops blanks, dedupes, and labels each line ("线路 N" default).
func normalizeManagedSiteURLs(urls []ManagedSiteURL, legacyURL string) []ManagedSiteURL {
	combined := append([]ManagedSiteURL(nil), urls...)
	if strings.TrimSpace(legacyURL) != "" {
		combined = append(combined, ManagedSiteURL{URL: legacyURL})
	}
	out := []ManagedSiteURL{}
	seen := map[string]bool{}
	for _, entry := range combined {
		url := config.NormalizeBaseURL(entry.URL)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = fmt.Sprintf("线路 %d", len(out)+1)
		}
		out = append(out, ManagedSiteURL{Name: name, URL: url})
	}
	return out
}

// mergeManagedSiteConfigsByToken merges sites that share a (category, token) into
// a single site, concatenating their url lists (deduped). Distinct tokens stay
// separate, so different upstreams are never conflated.
func mergeManagedSiteConfigsByToken(sites []ManagedAPISiteConfig) []ManagedAPISiteConfig {
	out := []ManagedAPISiteConfig{}
	index := map[string]int{}
	for _, site := range sites {
		key := site.Category + "\x00" + site.Token
		if at, ok := index[key]; ok {
			out[at].URLs = mergeManagedSiteURLs(out[at].URLs, site.URLs)
			out[at].URL = out[at].URLs[0].URL
			continue
		}
		index[key] = len(out)
		out = append(out, site)
	}
	return out
}

func mergeManagedSiteURLs(existing, extra []ManagedSiteURL) []ManagedSiteURL {
	seen := map[string]bool{}
	for _, entry := range existing {
		seen[entry.URL] = true
	}
	for _, entry := range extra {
		if seen[entry.URL] {
			continue
		}
		seen[entry.URL] = true
		existing = append(existing, entry)
	}
	return existing
}

// matchingSiteToken resolves the token for a site whose token was submitted
// blank ("沿用原值", because the UI only ever holds the masked token). A site
// configures its access token once and the UI never renames it, so a re-save —
// even one that adds a url — matches its stored token by (name, category). A
// brand-new site with a blank token has no match and is correctly rejected
// (its token must be entered); the match is deliberately scoped to the same
// name to avoid silently pulling an unrelated site's token.
func matchingSiteToken(site ManagedAPISiteConfig, previous []ManagedAPISiteConfig) string {
	for _, candidate := range previous {
		if strings.TrimSpace(candidate.Name) == site.Name && strings.EqualFold(strings.TrimSpace(candidate.Category), site.Category) {
			if token := strings.TrimSpace(candidate.Token); token != "" {
				return token
			}
		}
	}
	return ""
}

func isSupportedAdminSiteKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "api", "managed-api", "managed_api", "admin":
		return true
	default:
		return false
	}
}

// toNewAPISites expands one grouped site (one token, many urls) into the flat
// []newapi.Site the runtime consumers (combine, status, connectivity, nav) read
// — one entry per base_url, all sharing the site token, each carrying its line
// name for homepage display.
func (site ManagedAPISiteConfig) toNewAPISites() []newapi.Site {
	out := make([]newapi.Site, 0, len(site.URLs))
	for _, u := range site.URLs {
		out = append(out, newapi.Site{
			Name:                site.Name,
			Category:            site.Category,
			LineName:            u.Name,
			URL:                 u.URL,
			Token:               site.Token,
			UserID:              site.UserID,
			Kind:                site.Kind,
			SkipUserHeader:      site.SkipUserHeader,
			QuotaUnit:           site.QuotaUnit,
			Currency:            site.Currency,
			RechargeRatio:       site.RechargeRatio,
			ChannelListEndpoint: site.ChannelListEndpoint,
			Note:                site.Note,
		})
	}
	return out
}

func applyToolConfigSnapshot(cfg ToolConfig) {
	activityapp.SetRuntimeConfig(cfg.Activity)
	activityapp.SetMCYRuntimeConfig(activityapp.MCYRuntimeConfig{
		BaseURL:        cfg.MCY.BaseURL,
		Username:       cfg.MCY.Username,
		Password:       cfg.MCY.Password,
		Cookie:         cfg.MCY.Cookie,
		LoginEndpoint:  cfg.MCY.LoginEndpoint,
		UploadEndpoint: cfg.MCY.UploadEndpoint,
	})
	resetModelStatusCache()
}

func managedSitesForRuntime() ([]newapi.Site, string) {
	if unifiedConfig != nil {
		return unifiedConfig.ManagedSites(), ""
	}
	return config.LoadManagedSites(rootDir)
}

func primarySiteForCombine() (newapi.Site, error) {
	sites, msg := managedSitesForRuntime()
	for _, site := range sites {
		if strings.EqualFold(site.Category, "api") {
			return site, nil
		}
	}
	if len(sites) > 0 {
		return sites[0], nil
	}
	if msg != "" {
		return newapi.Site{}, fmt.Errorf("%s", msg)
	}
	return newapi.Site{}, fmt.Errorf("missing NewAPI primary site config")
}

func resetModelStatusCache() {
	modelCache.Lock()
	defer modelCache.Unlock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
}

func isUnifiedAdminConfigAPI(path string) bool {
	return path == "/api/admin/config"
}

func isUnifiedAdminSessionAPI(path string) bool {
	return path == "/api/admin/session"
}

type adminSessionLoginRequest struct {
	Token string `json:"token"`
}

func handleUnifiedAdminSessionAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": validUnifiedAdminSession(r)})
	case http.MethodPost:
		var body adminSessionLoginRequest
		if err := readJSON(r, &body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "登录格式错误")
			return
		}
		if !auth.CheckAdminToken(body.Token, os.Getenv("ADMIN_TOKEN"), "") {
			clearUnifiedAdminSession(w)
			writeJSONError(w, http.StatusUnauthorized, "管理员口令不正确")
			return
		}
		http.SetCookie(w, newUnifiedAdminSessionCookie(time.Now()))
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
	case http.MethodDelete:
		clearUnifiedAdminSession(w)
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writeJSONError(w, http.StatusMethodNotAllowed, "Only GET, POST, DELETE")
	}
}

func handleUnifiedAdminConfigAPI(w http.ResponseWriter, r *http.Request) {
	if !requireUnifiedAdminToken(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, adminConfigResponse(unifiedConfig.Snapshot()))
	case http.MethodPut:
		var patch adminConfigPatch
		if err := readJSON(r, &patch); err != nil {
			writeJSONError(w, http.StatusBadRequest, "配置格式错误")
			return
		}
		cfg, newAPIChanged, err := unifiedConfig.SavePatch(patch)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		applyToolConfigSnapshot(cfg)
		if newAPIChanged {
			rebuildCombine()
		}
		writeJSON(w, http.StatusOK, adminConfigResponse(cfg))
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeJSONError(w, http.StatusMethodNotAllowed, "Only GET, PUT")
	}
}

func requireUnifiedAdminToken(w http.ResponseWriter, r *http.Request) bool {
	if auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") || validUnifiedAdminSession(r) {
		return true
	}
	writeJSONError(w, http.StatusUnauthorized, "未授权")
	return false
}

func withUnifiedAdminAuthorization(r *http.Request) *http.Request {
	if !strings.HasPrefix(r.URL.Path, "/api/admin/") || !validUnifiedAdminSession(r) {
		return r
	}
	token := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	if token == "" {
		return r
	}
	cloned := r.Clone(r.Context())
	cloned.Header = r.Header.Clone()
	cloned.Header.Set("Authorization", "Bearer "+token)
	return cloned
}

func adminBearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return ""
	}
	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func newUnifiedAdminSessionCookie(now time.Time) *http.Cookie {
	expiresAt := now.Add(adminSessionTTL).Unix()
	nonce := randomSessionNonce()
	value := encodeUnifiedAdminSession(expiresAt, nonce)
	return &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(adminSessionTTL.Seconds()),
		Expires:  now.Add(adminSessionTTL),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func clearUnifiedAdminSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func validUnifiedAdminSession(r *http.Request) bool {
	cookie, err := r.Cookie(adminSessionCookieName)
	if err != nil {
		return false
	}
	return verifyUnifiedAdminSession(cookie.Value, time.Now())
}

func encodeUnifiedAdminSession(expiresAt int64, nonce string) string {
	expires := strconv.FormatInt(expiresAt, 10)
	message := "v1|" + expires + "|" + nonce
	return "v1." + expires + "." + nonce + "." + signUnifiedAdminSession(message)
}

func verifyUnifiedAdminSession(value string, now time.Time) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return false
	}
	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expiresAt <= now.Unix() {
		return false
	}
	message := "v1|" + parts[1] + "|" + parts[2]
	expected := signUnifiedAdminSession(message)
	return expected != "" && hmac.Equal([]byte(expected), []byte(parts[3]))
}

func signUnifiedAdminSession(message string) string {
	secret := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomSessionNonce() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}

func adminConfigResponse(cfg ToolConfig) map[string]any {
	activityPayload := map[string]any{}
	if raw, err := json.Marshal(cfg.Activity); err == nil {
		_ = json.Unmarshal(raw, &activityPayload)
	}
	activityPayload["actualExpectedValue"] = activity.ActualExpectedValue(cfg.Activity)
	return map[string]any{
		"newapi": map[string]any{
			"sites": adminSiteResponses(cfg.NewAPI.Sites),
		},
		"activity": activityPayload,
		"mcy": map[string]any{
			"baseUrl":        cfg.MCY.BaseURL,
			"username":       cfg.MCY.Username,
			"loginEndpoint":  cfg.MCY.LoginEndpoint,
			"uploadEndpoint": cfg.MCY.UploadEndpoint,
			"passwordSet":    cfg.MCY.Password != "",
			"passwordMasked": maskSecret(cfg.MCY.Password),
		},
	}
}

func adminSiteResponses(sites []ManagedAPISiteConfig) []map[string]any {
	out := make([]map[string]any, 0, len(sites))
	for _, site := range sites {
		urls := make([]map[string]any, 0, len(site.URLs))
		for _, u := range site.URLs {
			urls = append(urls, map[string]any{"name": u.Name, "url": u.URL})
		}
		out = append(out, map[string]any{
			"name":                site.Name,
			"category":            site.Category,
			"urls":                urls,
			"url":                 site.URL,
			"userId":              site.UserID,
			"kind":                site.Kind,
			"skipUserHeader":      site.SkipUserHeader,
			"quotaUnit":           site.QuotaUnit,
			"currency":            site.Currency,
			"rechargeRatio":       site.RechargeRatio,
			"channelListEndpoint": site.ChannelListEndpoint,
			"note":                site.Note,
			"tokenSet":            site.Token != "",
			"tokenMasked":         maskSecret(site.Token),
		})
	}
	return out
}

func maskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	runes := []rune(secret)
	if len(runes) <= 8 {
		return "••••"
	}
	return string(runes[:4]) + "…" + string(runes[len(runes)-4:])
}
