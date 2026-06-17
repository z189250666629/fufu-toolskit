package tokens

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fufu/newapi"
)

func TestNormalizeKeys(t *testing.T) {
	got := NormalizeKeys([]string{"abcdefghijkl sk-defghijkl\nabcdefghijkl"})
	if len(got) != 2 || got[0] != "sk-abcdefghijkl" || got[1] != "sk-defghijkl" {
		t.Fatalf("NormalizeKeys = %#v", got)
	}
}

func TestNormalizeKeysAcceptsFullWidthPunctuationAndQuotedJSONPaste(t *testing.T) {
	got := NormalizeKeys([]string{`["sk-alpha123456789"， "beta123456789"；"sk-gamma123456789"]`})
	want := []string{"sk-alpha123456789", "sk-beta123456789", "sk-gamma123456789"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeKeys = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeKeys = %#v, want %#v", got, want)
		}
	}
}

func TestIsMaskedKey(t *testing.T) {
	for _, key := range []string{"sk-********masked", "sk-abcd…wxyz"} {
		if !IsMaskedKey(key) {
			t.Fatalf("IsMaskedKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"sk-full-key-value", "full-key-value", ""} {
		if IsMaskedKey(key) {
			t.Fatalf("IsMaskedKey(%q) = true, want false", key)
		}
	}
}

func TestBatchSearchFoundAndMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("token")
		if key == "foundxxxxxx" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"id": 1, "key": "foundxxxxxx", "name": "card", "remain_quota": 500000, "interval_unit": 3, "status": 1}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))
	_, found, missing, err := svc.BatchSearch(context.Background(), []string{"foundxxxxxx", "missingxxxxx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Key != "sk-foundxxxxxx" {
		t.Fatalf("found = %#v", found)
	}
	if len(missing) != 1 || missing[0] != "sk-missingxxxxx" {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestQuotaConversion(t *testing.T) {
	svc := &Service{QuotaUnit: 500000}
	if svc.DollarsToQuota(2.5) != 1250000 {
		t.Fatalf("bad dollars to quota")
	}
	if svc.QuotaToDollars(1000000) != 2 {
		t.Fatalf("bad quota to dollars")
	}
}

func TestQuotaConversionUsesDefaultQuotaUnitForNilService(t *testing.T) {
	var svc *Service
	defer func() {
		if x := recover(); x != nil {
			t.Fatalf("quota conversion should use default unit, not panic: %v", x)
		}
	}()

	if got := svc.DollarsToQuota(2); got != int64(newapi.DefaultQuotaUnit*2) {
		t.Fatalf("DollarsToQuota nil service = %d", got)
	}
	if got := svc.QuotaToDollars(int64(newapi.DefaultQuotaUnit * 3)); got != 3 {
		t.Fatalf("QuotaToDollars nil service = %v", got)
	}
}

func TestFromRawNormalizesKeyStatusAndNumbers(t *testing.T) {
	token := FromRaw(map[string]any{
		"id":             float64(9),
		"key":            "abc123456789",
		"name":           "card",
		"remain_quota":   json.Number("123"),
		"used_quota":     "45",
		"interval_unit":  float64(60),
		"interval_quota": "1000",
		"group":          "vip",
		"status":         float64(0),
		"created_time":   "99",
	})

	if token.ID != 9 || token.Key != "sk-abc123456789" || token.Name != "card" {
		t.Fatalf("unexpected token identity: %#v", token)
	}
	if token.RemainQuota != 123 || token.UsedQuota != 45 || token.IntervalUnit != 60 || token.IntervalQuota != 1000 || token.CreatedTime != 99 {
		t.Fatalf("unexpected token counters: %#v", token)
	}
	if token.Group != "vip" || token.Status != 1 {
		t.Fatalf("unexpected token defaults: %#v", token)
	}
}

func TestFromRawTreatsNilStringFieldsAsBlank(t *testing.T) {
	token := FromRaw(map[string]any{
		"key":   nil,
		"name":  nil,
		"group": nil,
	})

	if token.Key != "" || token.Name != "" || token.Group != "" {
		t.Fatalf("nil string fields should be blank, got %#v", token)
	}
}

func TestRawNumberInt64ParsesDecimalJSONNumberConsistently(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want int64
	}{
		{name: "decimal json number", raw: json.Number("42.9"), want: 42},
		{name: "decimal string", raw: "8.9", want: 8},
		{name: "trimmed integer string", raw: "  123  ", want: 123},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInt64(tt.raw); got != tt.want {
				t.Fatalf("toInt64(%#v) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFromRawStatusDefaultSemantics(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
		want int
	}{
		{name: "missing", raw: map[string]any{}, want: 1},
		{name: "nil", raw: map[string]any{"status": nil}, want: 1},
		{name: "zero", raw: map[string]any{"status": json.Number("0")}, want: 1},
		{name: "invalid", raw: map[string]any{"status": "not-a-number"}, want: 1},
		{name: "string number", raw: map[string]any{"status": "2"}, want: 2},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromRaw(tt.raw).Status; got != tt.want {
				t.Fatalf("FromRaw(%#v).Status = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDataListReadsNestedCandidates(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"items": []any{
				map[string]any{"id": float64(1)},
				"skip",
				map[string]any{"id": float64(2)},
			},
		},
	}

	got := DataList(data)
	if len(got) != 2 || got[0]["id"] != float64(1) || got[1]["id"] != float64(2) {
		t.Fatalf("DataList = %#v", got)
	}
}

func TestMajorityGroupTieBreaksAndFallback(t *testing.T) {
	if got := MajorityGroup([]Token{{Group: "vip"}, {Group: "default"}, {Group: "vip"}}); got != "vip" {
		t.Fatalf("majority group = %q", got)
	}
	if got := MajorityGroup([]Token{{Group: "zeta"}, {Group: "alpha"}}); got != "alpha" {
		t.Fatalf("tie majority group = %q", got)
	}
	if got := MajorityGroup([]Token{{}}); got != "default" {
		t.Fatalf("fallback majority group = %q", got)
	}
}

func TestSearchCreateUpdateDelete(t *testing.T) {
	var created map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			_ = json.NewDecoder(r.Body).Decode(&created)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			keyword := r.URL.Query().Get("keyword")
			if keyword == "created-name" {
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"id": 2, "key": "createdkeyxx", "name": "created-name", "remain_quota": 10, "status": 1}}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/token/":
			var raw map[string]any
			_ = json.NewDecoder(r.Body).Decode(&raw)
			if raw["name"] != "renamed" {
				t.Fatalf("bad update body: %#v", raw)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/token/2":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/batch/keys":
			var raw map[string][]int
			_ = json.NewDecoder(r.Body).Decode(&raw)
			if len(raw["ids"]) != 1 || raw["ids"][0] != 2 {
				t.Fatalf("bad batch key body: %#v", raw)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"keys": map[string]any{"2": "createdkeyxx"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/2/key":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"key": "createdkeyxx"}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))
	if res, _, err := svc.CreateToken(context.Background(), map[string]any{"name": "created-name"}); err != nil || !res.OK() || created["name"] != "created-name" {
		t.Fatalf("CreateToken res=%+v created=%#v err=%v", res, created, err)
	}
	found, err := svc.SearchTokenByName(context.Background(), "created-name")
	if err != nil || found == nil || found.ID != 2 {
		t.Fatalf("SearchTokenByName found=%#v err=%v", found, err)
	}
	found.Raw["name"] = "renamed"
	if res, _, err := svc.UpdateTokenRaw(context.Background(), found.Raw); err != nil || !res.OK() {
		t.Fatalf("UpdateTokenRaw res=%+v err=%v", res, err)
	}
	key, err := svc.GetTokenKey(context.Background(), found.ID)
	if err != nil || key != "sk-createdkeyxx" {
		t.Fatalf("GetTokenKey key=%q err=%v", key, err)
	}
	keys, err := svc.GetTokenKeysBatch(context.Background(), []int{found.ID})
	if err != nil || keys[found.ID] != "sk-createdkeyxx" {
		t.Fatalf("GetTokenKeysBatch keys=%#v err=%v", keys, err)
	}
	if ok, res, err := svc.DeleteToken(context.Background(), 2); err != nil || !ok || !res.OK() {
		t.Fatalf("DeleteToken ok=%v res=%+v err=%v", ok, res, err)
	}
}

func TestCreateTokenAndResolveKeyUsesCreateResponseKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/token/" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"id":           9,
				"name":         "created-direct",
				"key":          "direct-key",
				"remain_quota": 100,
				"group":        "mix",
			},
		})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	created, err := svc.CreateTokenAndResolveKey(context.Background(), map[string]any{"name": "created-direct"}, "created-direct")

	if err != nil {
		t.Fatalf("CreateTokenAndResolveKey err=%v", err)
	}
	if created.Key != "sk-direct-key" || created.Token.ID != 9 || created.Token.Key != "sk-direct-key" || created.Token.Raw["key"] != "sk-direct-key" {
		t.Fatalf("created=%#v", created)
	}
}

func TestCreateTokenAndResolveKeyUsesUnmaskedSearchKey(t *testing.T) {
	var createdName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			createdName = getString(body, "name")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			if r.URL.Query().Get("keyword") != createdName {
				t.Fatalf("keyword=%q, want %q", r.URL.Query().Get("keyword"), createdName)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"id": 12, "name": createdName, "key": "legacy-full-key", "remain_quota": 100, "group": "mix"},
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	created, err := svc.CreateTokenAndResolveKey(context.Background(), map[string]any{"name": "created-search"}, "")

	if err != nil {
		t.Fatalf("CreateTokenAndResolveKey err=%v", err)
	}
	if created.Key != "sk-legacy-full-key" || created.Token.ID != 12 || created.Token.Raw["key"] != "sk-legacy-full-key" {
		t.Fatalf("created=%#v", created)
	}
}

func TestCreateTokenAndResolveKeyFallsBackToKeyEndpointForMaskedSearchKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"id": 13, "name": "masked-search", "key": "sk-********abcd", "remain_quota": 100, "group": "mix"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/batch/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"keys": map[string]any{"13": "resolved-key"}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	created, err := svc.CreateTokenAndResolveKey(context.Background(), map[string]any{"name": "masked-search"}, "")

	if err != nil {
		t.Fatalf("CreateTokenAndResolveKey err=%v", err)
	}
	if created.Key != "sk-resolved-key" || created.Token.ID != 13 || created.Token.Raw["key"] != "sk-resolved-key" {
		t.Fatalf("created=%#v", created)
	}
}

func TestCreateTokenAndResolveKeyFallsBackToTokenDetailWhenKeyEndpointsAreUnsupported(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"id": 14, "name": "legacy-detail", "key": "sk-********masked", "remain_quota": 100, "group": "mix"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/batch/keys":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Invalid URL (POST /api/token/batch/keys)"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/14/key":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Invalid URL (POST /api/token/14/key)"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/14":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"id": 14, "name": "legacy-detail", "key": "legacy-detail-full-key", "remain_quota": 100, "group": "mix",
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	created, err := svc.CreateTokenAndResolveKey(context.Background(), map[string]any{"name": "legacy-detail"}, "")

	if err != nil {
		t.Fatalf("CreateTokenAndResolveKey err=%v", err)
	}
	if created.Key != "sk-legacy-detail-full-key" || created.Token.ID != 14 || created.Token.Raw["key"] != "sk-legacy-detail-full-key" {
		t.Fatalf("created=%#v", created)
	}
	got := strings.Join(paths, " | ")
	for _, want := range []string{"POST /api/token/batch/keys", "POST /api/token/14/key", "GET /api/token/14"} {
		if !strings.Contains(got, want) {
			t.Fatalf("paths=%q, missing %q", got, want)
		}
	}
}

func TestDeleteTokenTreatsSuccessFalseAsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/token/2" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "delete denied"})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	ok, res, err := svc.DeleteToken(context.Background(), 2)
	if err == nil || !strings.Contains(err.Error(), "delete denied") {
		t.Fatalf("expected payload error, got ok=%v res=%+v err=%v", ok, res, err)
	}
	if ok || !res.OK() {
		t.Fatalf("ok=%v res=%+v", ok, res)
	}
}

func TestSearchTokenByKeySkipsBlankKeyWithoutRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("blank key should not issue request: %s", r.URL.String())
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	found, err := svc.SearchTokenByKey(context.Background(), "  sk-  ")
	if err != nil || found != nil {
		t.Fatalf("found=%#v err=%v", found, err)
	}
}

func TestBatchSearchReturnsConfigurationErrorForNilService(t *testing.T) {
	var svc *Service
	defer func() {
		if x := recover(); x != nil {
			t.Fatalf("BatchSearch should return an error, not panic: %v", x)
		}
	}()
	_, _, _, err := svc.BatchSearch(context.Background(), []string{"sk-valid-key-123"})
	if err == nil {
		t.Fatal("BatchSearch should report missing token service")
	}
}

func TestMutationMethodsReturnConfigurationErrorForNilService(t *testing.T) {
	var svc *Service
	cases := []struct {
		name string
		call func() error
	}{
		{name: "GetToken", call: func() error {
			_, err := svc.GetToken(context.Background(), 1)
			return err
		}},
		{name: "CreateToken", call: func() error {
			_, _, err := svc.CreateToken(context.Background(), map[string]any{"name": "card"})
			return err
		}},
		{name: "CreateTokens", call: func() error {
			_, _, err := svc.CreateTokens(context.Background(), 2, map[string]any{"name": "card"})
			return err
		}},
		{name: "GetTokenKey", call: func() error {
			_, err := svc.GetTokenKey(context.Background(), 1)
			return err
		}},
		{name: "GetTokenKeysBatch", call: func() error {
			_, err := svc.GetTokenKeysBatch(context.Background(), []int{1})
			return err
		}},
		{name: "UpdateTokenRaw", call: func() error {
			_, _, err := svc.UpdateTokenRaw(context.Background(), map[string]any{"id": 1})
			return err
		}},
		{name: "DeleteToken", call: func() error {
			ok, _, err := svc.DeleteToken(context.Background(), 1)
			if ok {
				t.Fatal("DeleteToken should not report success without a configured service")
			}
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if x := recover(); x != nil {
					t.Fatalf("%s should return an error, not panic: %v", tc.name, x)
				}
			}()
			err := tc.call()
			if err == nil || !strings.Contains(err.Error(), "token service is not configured") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestSearchTokenByKeyReturnsPayloadErrorOnSuccessFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "bad token"})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	found, err := svc.SearchTokenByKey(context.Background(), "sk-failed-token")
	if found != nil || err == nil || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("found=%#v err=%v", found, err)
	}
}

func TestSearchTokenByKeyMasksKeyInErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	secretKey := "sk-shortkey"
	found, err := svc.SearchTokenByKey(context.Background(), secretKey)

	if found != nil || err == nil {
		t.Fatalf("found=%#v err=%v", found, err)
	}
	if strings.Contains(err.Error(), secretKey) || strings.Contains(err.Error(), "shortkey") {
		t.Fatalf("error should mask key %q, got %q", secretKey, err.Error())
	}
}

func TestSearchTokenByNameReturnsPayloadErrorOnSuccessFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "name rejected"})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	found, err := svc.SearchTokenByName(context.Background(), "bad-name")
	if found != nil || err == nil || !strings.Contains(err.Error(), "name rejected") {
		t.Fatalf("found=%#v err=%v", found, err)
	}
}

func TestAddQuotaUsesDefaultQuotaUnitWhenServiceQuotaUnitUnset(t *testing.T) {
	var updated map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"id": 7, "key": "quota-key-123", "name": "quota-card", "remain_quota": 10, "status": 1}}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/token/":
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := &Service{Client: newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"})}

	if err := svc.AddQuota(context.Background(), "sk-quota-key-123", 1); err != nil {
		t.Fatal(err)
	}
	got := int64(updated["remain_quota"].(float64))
	want := int64(10 + newapi.DefaultQuotaUnit)
	if got != want {
		t.Fatalf("remain_quota=%d, want %d; body=%#v", got, want, updated)
	}
}

func TestAddQuotaReturnsPayloadErrorOnSuccessFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"id": 8, "key": "quota-key-456", "name": "quota-card", "remain_quota": 10, "status": 1}}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "quota update denied"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	err := svc.AddQuota(context.Background(), "sk-quota-key-456", 1)
	if err == nil || !strings.Contains(err.Error(), "quota update denied") {
		t.Fatalf("expected payload error, got %v", err)
	}
}

func TestGetTokenReturnsPayloadErrorOnSuccessFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/9" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "token disabled"})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))

	got, err := svc.GetToken(context.Background(), 9)
	if err == nil || !strings.Contains(err.Error(), "token disabled") || got.ID != 0 || got.Key != "" {
		t.Fatalf("token=%#v err=%v", got, err)
	}
}
