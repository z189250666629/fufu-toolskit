package combine

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"fufu/tokens"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func upstreamStatusMessage(r APIResponse, fallback string) string {
	if r.StatusCode > 0 {
		return fmt.Sprintf("%s（上游状态 %d）", fallback, r.StatusCode)
	}
	return fallback
}

func resolvedFromToken(t tokens.Token) ResolvedToken {
	return ResolvedToken{ID: t.ID, Key: t.Key, Name: t.Name, RemainQuota: t.RemainQuota, UsedQuota: t.UsedQuota, IntervalUnit: t.IntervalUnit, Group: t.Group, Status: t.Status, Raw: t.Raw}
}

func tokenFromRaw(raw map[string]any) ResolvedToken {
	if raw == nil {
		raw = map[string]any{}
	}
	return ResolvedToken{ID: toInt(raw["id"]), Key: ensureFullKey(getString(raw, "key")), Name: getString(raw, "name"), RemainQuota: toInt64(raw["remain_quota"]), UsedQuota: toInt64(raw["used_quota"]), IntervalUnit: toInt(raw["interval_unit"]), Group: stringOrDefault(getString(raw, "group"), "mix"), Status: intOrDefault(toIntDefault(raw["status"], 1), 1), Raw: raw}
}

func cloneMap(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		out[k] = v
	}
	return out
}

func ensureFullKey(key string) string {
	s := strings.TrimSpace(key)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "sk-") {
		return s
	}
	return "sk-" + s
}

func displayKey(key string) string {
	full := ensureFullKey(key)
	bare := strings.TrimPrefix(full, "sk-")
	r := []rune(bare)
	if len(r) <= 8 {
		return full
	}
	return "sk-" + string(r[:4]) + "…" + string(r[len(r)-4:])
}

func normalizeKeys(raw []string) []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, item := range raw {
		key := ensureFullKey(strings.TrimSpace(item))
		if key == "" || key == "sk-" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func majorityGroup(tokens []ResolvedToken) string {
	counts := map[string]int{}
	for _, t := range tokens {
		g := t.Group
		if g == "" {
			g = "mix"
		}
		counts[g]++
	}
	winner := "mix"
	max := 0
	for g, c := range counts {
		if c > max {
			winner = g
			max = c
		}
	}
	return winner
}

func evaluatePublicMergeEligibility(tokens []ResolvedToken) PublicMergeEligibility {
	reasons := []string{}
	if len(tokens) < 2 {
		reasons = append(reasons, "至少需要 2 张天卡才能合卡")
	}
	for _, t := range tokens {
		if t.Status != 1 {
			reasons = append(reasons, fmt.Sprintf("%s 已被禁用", displayKey(t.Key)))
		}
		if t.IntervalUnit != publicSourceUnit {
			reasons = append(reasons, fmt.Sprintf("%s 不是天卡", displayKey(t.Key)))
		}
		if t.UsedQuota > 0 {
			reasons = append(reasons, fmt.Sprintf("%s 已经使用过", displayKey(t.Key)))
		}
		if t.RemainQuota <= 0 {
			reasons = append(reasons, fmt.Sprintf("%s 没有剩余额度", displayKey(t.Key)))
		}
	}
	return PublicMergeEligibility{Eligible: len(reasons) == 0, Reasons: reasons}
}

func dataList(data map[string]any) []map[string]any {
	raw, ok := data["data"].([]any)
	if !ok {
		return nil
	}
	out := []map[string]any{}
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func findTokenByName(data map[string]any, name string) map[string]any {
	for _, item := range dataList(data) {
		if getString(item, "name") == name {
			return item
		}
	}
	return nil
}

func findResolvedByID(tokens []ResolvedToken, id int) *ResolvedToken {
	for i := range tokens {
		if tokens[i].ID == id {
			return &tokens[i]
		}
	}
	return nil
}

func uniqueIDs(tokens []ResolvedToken) []int {
	seen := map[int]bool{}
	ids := []int{}
	for _, t := range tokens {
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}

func decodeJSON(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(out)
}

func sha256Hex(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }

func keyHash(key string) string { return sha256Hex(ensureFullKey(key)) }

func keyMask(key string) string { return displayKey(key) }

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func randomBase36(n int) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b)
}

func getString(obj map[string]any, key string) string {
	if obj == nil || obj[key] == nil {
		return ""
	}
	switch v := obj[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func toInt(v any) int { return int(toInt64(v)) }

func toIntDefault(v any, fallback int) int {
	if v == nil {
		return fallback
	}
	return toInt(v)
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return int64(f)
		}
	case string:
		s := strings.TrimSpace(x)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
	}
	return 0
}

func intOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func int64OrDefault(v, fallback int64) int64 {
	if v == 0 {
		return fallback
	}
	return v
}

func stringOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func strp(v string) *string { return &v }

func intp(v int) *int { return &v }

func statusOrDefault(status, fallback int) int {
	if status >= 100 && status <= 599 {
		return status
	}
	return fallback
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
