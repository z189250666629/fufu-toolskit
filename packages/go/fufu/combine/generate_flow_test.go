package combine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandleGenerateCreatesRenamesVerifiesAndReturnsKeys(t *testing.T) {
	var createHits, renameHits, verifyHits atomic.Int32
	tokenNames := map[int]string{}
	tokenKeys := map[int]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			name := strings.TrimSpace(fmt.Sprint(body["name"]))
			if name == "" || len([]rune(name)) > 30 {
				t.Fatalf("temporary token name should be nonblank and <= 30 runes, got %q", name)
			}
			if got, want := int64(body["remain_quota"].(float64)), int64(55_500_000); got != want {
				t.Fatalf("remain_quota=%d, want %d; body=%#v", got, want, body)
			}
			if body["group"] != "mix" || int(body["interval_unit"].(float64)) != 9 {
				t.Fatalf("create body=%#v", body)
			}
			id := int(createHits.Add(1))
			tokenNames[id] = name
			tokenKeys[id] = fmt.Sprintf("generated-key-%d", id)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"id": id, "name": name, "key": tokenKeys[id], "remain_quota": body["remain_quota"], "interval_unit": 9, "group": "mix", "status": 1,
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/token/":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			id := int(body["id"].(float64))
			if id <= 0 || tokenNames[id] == "" {
				t.Fatalf("rename unknown token body=%#v", body)
			}
			if body["name"] != "111" || body["key"] != "sk-"+tokenKeys[id] {
				t.Fatalf("rename body=%#v", body)
			}
			renameHits.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/token/"):
			var id int
			if _, err := fmt.Sscanf(r.URL.Path, "/api/token/%d", &id); err != nil {
				t.Fatalf("bad verify path %s", r.URL.Path)
			}
			if tokenNames[id] == "" {
				t.Fatalf("verify unknown token id %d", id)
			}
			verifyHits.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"id": id, "name": "111", "key": tokenKeys[id], "remain_quota": 55_500_000, "interval_unit": 9, "group": "mix", "status": 1,
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	app := NewApp(Config{URL: server.URL, Token: "test-token", UserID: "1", QuotaUnit: 500000}, nil)
	app.authFailureDelay = 0
	app.sessions["admin-session"] = SessionInfo{Expiry: time.Now().Add(time.Hour), Role: RoleAdmin}
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"count":3,"quota":111,"intervalUnit":9,"group":"mix"}`))
	req.Header.Set("X-Session-Token", "admin-session")
	rec := httptest.NewRecorder()

	app.handleAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Keys   []string `json:"keys"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	if strings.Join(body.Keys, ",") != "sk-generated-key-1,sk-generated-key-2,sk-generated-key-3" || len(body.Errors) != 0 {
		t.Fatalf("response=%#v", body)
	}
	if createHits.Load() != 3 || renameHits.Load() != 3 || verifyHits.Load() != 3 {
		t.Fatalf("create/rename/verify hits=%d/%d/%d, want 3/3/3", createHits.Load(), renameHits.Load(), verifyHits.Load())
	}
}
