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
}

type NewAPIAdminConfig struct {
	Sites []ManagedAPISiteConfig `json:"sites"`
}

type ManagedAPISiteConfig struct {
	Name                string  `json:"name"`
	URL                 string  `json:"url"`
	Token               string  `json:"token,omitempty"`
	UserID              string  `json:"userId"`
	Kind                string  `json:"kind,omitempty"`
	SkipUserHeader      bool    `json:"skipUserHeader,omitempty"`
	QuotaUnit           int64   `json:"quotaUnit"`
	Currency            string  `json:"currency"`
	RechargeRatio       float64 `json:"rechargeRatio"`
	ChannelListEndpoint string  `json:"channelListEndpoint,omitempty"`
	Note                string  `json:"note,omitempty"`
}

type adminConfigPatch struct {
	NewAPI *struct {
		Sites []ManagedAPISiteConfig `json:"sites"`
	} `json:"newapi"`
	Activity *activity.Config `json:"activity"`
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
		if err := json.Unmarshal(stored, &cfg); err != nil {
			return fmt.Errorf("%s 不是有效 JSON: %w", toolConfigDBName, err)
		}
	} else if raw, readErr := os.ReadFile(legacyPath); readErr == nil {
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
		sites = append(sites, site.toNewAPISite())
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
	}
}

func normalizeToolConfig(cfg ToolConfig, previous ToolConfig) (ToolConfig, error) {
	sites, err := normalizeManagedAPISiteConfigs(cfg.NewAPI.Sites, previous.NewAPI.Sites)
	if err != nil {
		return ToolConfig{}, err
	}
	cfg.NewAPI.Sites = sites
	cfg.Activity = activity.CloneConfig(cfg.Activity)
	return cfg, nil
}

func cloneToolConfig(cfg ToolConfig) ToolConfig {
	out := ToolConfig{Activity: activity.CloneConfig(cfg.Activity)}
	out.NewAPI.Sites = append([]ManagedAPISiteConfig(nil), cfg.NewAPI.Sites...)
	return out
}

func managedSiteConfigsFromSites(sites []newapi.Site) []ManagedAPISiteConfig {
	out := make([]ManagedAPISiteConfig, 0, len(sites))
	for _, site := range sites {
		out = append(out, ManagedAPISiteConfig{
			Name:                site.Name,
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
	out := []ManagedAPISiteConfig{}
	seen := map[string]bool{}
	for i, site := range sites {
		site.Name = strings.TrimSpace(site.Name)
		site.URL = config.NormalizeBaseURL(site.URL)
		site.Token = strings.TrimSpace(site.Token)
		site.UserID = strings.TrimSpace(site.UserID)
		site.Kind = strings.TrimSpace(site.Kind)
		site.Currency = strings.TrimSpace(site.Currency)
		site.ChannelListEndpoint = strings.TrimSpace(site.ChannelListEndpoint)
		site.Note = strings.TrimSpace(site.Note)
		if site.Token == "" {
			site.Token = matchingSiteToken(site, previous)
		}
		if site.Name == "" {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点缺少名称", i+1)
		}
		if seen[site.Name] {
			return nil, fmt.Errorf("NewAPI 站点名称重复: %s", site.Name)
		}
		if site.URL == "" {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点 base_url 无效", i+1)
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
		seen[site.Name] = true
		out = append(out, site)
	}
	return out, nil
}

func matchingSiteToken(site ManagedAPISiteConfig, previous []ManagedAPISiteConfig) string {
	for _, candidate := range previous {
		if strings.TrimSpace(candidate.Name) == site.Name && config.NormalizeBaseURL(candidate.URL) == site.URL {
			return strings.TrimSpace(candidate.Token)
		}
	}
	for _, candidate := range previous {
		if strings.TrimSpace(candidate.Name) == site.Name {
			return strings.TrimSpace(candidate.Token)
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

func (site ManagedAPISiteConfig) toNewAPISite() newapi.Site {
	return newapi.Site{
		Name:                site.Name,
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
	}
}

func applyToolConfigSnapshot(cfg ToolConfig) {
	activityapp.SetRuntimeConfig(cfg.Activity)
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
	}
}

func adminSiteResponses(sites []ManagedAPISiteConfig) []map[string]any {
	out := make([]map[string]any, 0, len(sites))
	for _, site := range sites {
		out = append(out, map[string]any{
			"name":                site.Name,
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
