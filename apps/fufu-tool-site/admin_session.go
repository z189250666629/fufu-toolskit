package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fufu/auth"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const adminSessionCookieName = "fufu_admin_session"

const adminSessionTTL = 12 * time.Hour

type adminSessionLoginRequest struct {
	Token string `json:"token"`
}

func isUnifiedAdminSessionAPI(path string) bool {
	return path == "/api/admin/session"
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
		// 管理员口令即 ADMIN_TOKEN（由部署环境/GitHub 配置注入）。未配置时无人能登录。
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
