package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fufu/newapi"
)

const DefaultSearchConcurrency = 6

type Token struct {
	ID            int            `json:"id"`
	Key           string         `json:"key"`
	Name          string         `json:"name"`
	RemainQuota   int64          `json:"remain_quota"`
	UsedQuota     int64          `json:"used_quota"`
	IntervalUnit  int            `json:"interval_unit"`
	IntervalQuota int64          `json:"interval_quota,omitempty"`
	Group         string         `json:"group"`
	Status        int            `json:"status"`
	CreatedTime   int64          `json:"created_time,omitempty"`
	Raw           map[string]any `json:"-"`
}

type Service struct {
	Client      *newapi.Client
	QuotaUnit   int64
	Concurrency int
}

func NewService(client *newapi.Client) *Service {
	quotaUnit := newapi.DefaultQuotaUnit
	if client != nil && client.Site.QuotaUnit > 0 {
		quotaUnit = client.Site.QuotaUnit
	}
	return &Service{Client: client, QuotaUnit: quotaUnit, Concurrency: DefaultSearchConcurrency}
}

func EnsureFullKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "sk-") {
		return key
	}
	return "sk-" + key
}

func BareKey(key string) string { return strings.TrimPrefix(EnsureFullKey(key), "sk-") }

func DisplayKey(key string) string {
	value := EnsureFullKey(key)
	if len(value) <= 14 {
		return value
	}
	return value[:7] + "..." + value[len(value)-5:]
}

func NormalizeKeys(raw []string) []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, item := range raw {
		for _, part := range strings.FieldsFunc(item, func(r rune) bool { return r == '\n' || r == '\r' || r == '\t' || r == ' ' || r == ',' || r == ';' }) {
			key := EnsureFullKey(part)
			if len(key) <= 10 {
				continue
			}
			bare := BareKey(key)
			if !seen[bare] {
				seen[bare] = true
				keys = append(keys, key)
			}
		}
	}
	return keys
}

func (s *Service) SearchTokenByKey(ctx context.Context, key string) (*Token, error) {
	if s == nil || s.Client == nil {
		return nil, fmt.Errorf("token service is not configured")
	}
	bare := BareKey(key)
	endpoint := "/api/token/search?keyword=&token=" + url.QueryEscape(bare) + "&p=0&size=10"
	res, data, err := s.Client.Request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, fmt.Errorf("查询 %s 失败", DisplayKey(key))
	}
	for _, item := range DataList(data) {
		if BareKey(getString(item, "key")) == bare {
			t := FromRaw(item)
			return &t, nil
		}
	}
	return nil, nil
}

func (s *Service) SearchTokenByName(ctx context.Context, name string) (*Token, error) {
	if s == nil || s.Client == nil {
		return nil, fmt.Errorf("token service is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	endpoint := "/api/token/search?keyword=" + url.QueryEscape(name) + "&p=0&size=10"
	res, data, err := s.Client.Request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, fmt.Errorf("查询 token 名称 %q 失败", name)
	}
	for _, item := range DataList(data) {
		if getString(item, "name") == name {
			t := FromRaw(item)
			return &t, nil
		}
	}
	return nil, nil
}

type SearchResult struct {
	Key   string
	Found *Token
}

func (s *Service) BatchSearch(ctx context.Context, raw []string) ([]string, []Token, []string, error) {
	keys := NormalizeKeys(raw)
	if len(keys) == 0 {
		return keys, nil, nil, nil
	}
	concurrency := s.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultSearchConcurrency
	}
	if concurrency > len(keys) {
		concurrency = len(keys)
	}
	results := make([]SearchResult, len(keys))
	jobs := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				found, err := s.SearchTokenByKey(ctx, keys[idx])
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}
				results[idx] = SearchResult{Key: keys[idx], Found: found}
			}
		}()
	}
	for i := range keys {
		select {
		case jobs <- i:
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, nil, nil, err
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, nil, nil, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, nil, nil, err
	default:
	}
	found := []Token{}
	missing := []string{}
	for _, r := range results {
		if r.Found != nil {
			found = append(found, *r.Found)
		} else {
			missing = append(missing, r.Key)
		}
	}
	return keys, found, missing, nil
}

func (s *Service) GetToken(ctx context.Context, id int) (Token, error) {
	res, data, err := s.Client.Request(ctx, http.MethodGet, fmt.Sprintf("/api/token/%d", id), nil)
	if err != nil {
		return Token{}, err
	}
	if !res.OK() {
		return Token{}, fmt.Errorf("Token %d 不存在或无法访问", id)
	}
	if raw, ok := data["data"].(map[string]any); ok {
		return FromRaw(raw), nil
	}
	return Token{}, fmt.Errorf("Token %d 数据异常", id)
}

func (s *Service) CreateToken(ctx context.Context, body map[string]any) (newapi.Response, map[string]any, error) {
	return s.Client.Request(ctx, http.MethodPost, "/api/token/", body)
}
func (s *Service) CreateTokens(ctx context.Context, count int, body map[string]any) (newapi.Response, map[string]any, error) {
	return s.Client.Request(ctx, http.MethodPost, fmt.Sprintf("/api/token/tokens?tokenCount=%d", count), body)
}
func (s *Service) UpdateTokenRaw(ctx context.Context, raw map[string]any) (newapi.Response, map[string]any, error) {
	return s.Client.Request(ctx, http.MethodPut, "/api/token/", raw)
}
func (s *Service) DeleteToken(ctx context.Context, id int) (bool, newapi.Response, error) {
	res, _, err := s.Client.Request(ctx, http.MethodDelete, fmt.Sprintf("/api/token/%d", id), nil)
	if err != nil {
		return false, newapi.Response{}, err
	}
	return res.OK(), res, nil
}

func (s *Service) AddQuota(ctx context.Context, key string, dollars int64) error {
	t, err := s.SearchTokenByKey(ctx, key)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("token not found on fufu")
	}
	add := dollars * s.QuotaUnit
	raw := map[string]any{}
	for k, v := range t.Raw {
		raw[k] = v
	}
	raw["remain_quota"] = t.RemainQuota + add
	if name := getString(raw, "name"); len(name) > 30 {
		raw["name"] = name[:30]
	}
	res, _, err := s.UpdateTokenRaw(ctx, raw)
	if err != nil {
		return err
	}
	if !res.OK() {
		return fmt.Errorf("fufu PUT failed: %s", res.BodyOr(strconv.Itoa(res.StatusCode)))
	}
	return nil
}

func (s *Service) DollarsToQuota(dollars float64) int64 {
	unit := s.QuotaUnit
	if unit <= 0 {
		unit = newapi.DefaultQuotaUnit
	}
	return int64(math.Round(dollars * float64(unit)))
}
func (s *Service) QuotaToDollars(quota int64) float64 {
	unit := s.QuotaUnit
	if unit <= 0 {
		unit = newapi.DefaultQuotaUnit
	}
	return float64(quota) / float64(unit)
}

func FromRaw(raw map[string]any) Token {
	return Token{ID: toInt(raw["id"]), Key: EnsureFullKey(getString(raw, "key")), Name: getString(raw, "name"), RemainQuota: toInt64(raw["remain_quota"]), UsedQuota: toInt64(raw["used_quota"]), IntervalUnit: toInt(raw["interval_unit"]), IntervalQuota: toInt64(raw["interval_quota"]), Group: getString(raw, "group"), Status: statusOrDefault(toInt(raw["status"]), 1), CreatedTime: toInt64(raw["created_time"]), Raw: raw}
}

func DataList(data map[string]any) []map[string]any {
	candidates := []any{data["data"], data["items"], data["tokens"]}
	if nested, ok := data["data"].(map[string]any); ok {
		candidates = append(candidates, nested["data"], nested["items"], nested["tokens"])
	}
	for _, c := range candidates {
		if arr, ok := c.([]any); ok {
			out := []map[string]any{}
			for _, item := range arr {
				if obj, ok := item.(map[string]any); ok {
					out = append(out, obj)
				}
			}
			return out
		}
	}
	return nil
}

func getString(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if v, ok := obj[key]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}
func statusOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
func toInt(v any) int { return int(toInt64(v)) }
func toInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(x), 10, 64)
		return n
	}
}

func MajorityGroup(tokens []Token) string {
	counts := map[string]int{}
	for _, t := range tokens {
		if t.Group != "" {
			counts[t.Group]++
		}
	}
	groups := make([]string, 0, len(counts))
	for g := range counts {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if counts[groups[i]] == counts[groups[j]] {
			return groups[i] < groups[j]
		}
		return counts[groups[i]] > counts[groups[j]]
	})
	if len(groups) > 0 {
		return groups[0]
	}
	return "default"
}
