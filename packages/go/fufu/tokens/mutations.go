package tokens

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"fufu/newapi"
)

type CreatedTokenResult struct {
	Token    Token
	Key      string
	Response newapi.Response
	Data     map[string]any
}

func (s *Service) configuredClient() (*newapi.Client, error) {
	if s == nil || s.Client == nil {
		return nil, fmt.Errorf("token service is not configured")
	}
	return s.Client, nil
}

func (s *Service) GetToken(ctx context.Context, id int) (Token, error) {
	client, err := s.configuredClient()
	if err != nil {
		return Token{}, err
	}
	res, data, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/api/token/%d", id), nil)
	if err != nil {
		return Token{}, err
	}
	if !res.OK() {
		return Token{}, fmt.Errorf("Token %d 不存在或无法访问", id)
	}
	if !newapi.IsSuccess(data) {
		return Token{}, fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("Token %d 数据异常", id)))
	}
	if raw, ok := data["data"].(map[string]any); ok {
		return FromRaw(raw), nil
	}
	return Token{}, fmt.Errorf("Token %d 数据异常", id)
}

func (s *Service) CreateToken(ctx context.Context, body map[string]any) (newapi.Response, map[string]any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return newapi.Response{}, nil, err
	}
	return client.Request(ctx, http.MethodPost, "/api/token/", body)
}

func (s *Service) CreateTokenAndResolveKey(ctx context.Context, body map[string]any, name string) (CreatedTokenResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = getString(body, "name")
	}
	res, data, err := s.CreateToken(ctx, body)
	out := CreatedTokenResult{Response: res, Data: data}
	if err != nil {
		return out, err
	}
	if !res.OK() {
		return out, fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, res.BodyOr(http.StatusText(res.StatusCode))))
	}
	if !newapi.IsSuccess(data) {
		return out, fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, "NewAPI 创建 token 失败"))
	}

	if token, ok := tokenFromCreatedPayload(data); ok {
		out.Token = token
		out.Key = fullUnmaskedKey(token.Key)
	}
	if out.Key != "" {
		out.Token = ensureTokenKey(out.Token, out.Key)
		if out.Token.ID <= 0 && name != "" {
			if found, err := s.SearchTokenByName(ctx, name); err == nil && found != nil {
				out.Token = ensureTokenKey(*found, out.Key)
			}
		}
		return out, nil
	}

	if name == "" {
		return out, fmt.Errorf("NewAPI 创建成功但未返回卡密，且缺少 token 名称，无法查回完整 key")
	}
	found, err := s.SearchTokenByName(ctx, name)
	if err != nil {
		return out, fmt.Errorf("创建后查找 token 失败: %v", err)
	}
	if found == nil || found.ID <= 0 {
		return out, fmt.Errorf("NewAPI 创建成功但未返回卡密，且无法按名称查回 token")
	}
	out.Token = *found
	if key := fullUnmaskedKey(found.Key); key != "" {
		out.Key = key
		out.Token = ensureTokenKey(out.Token, key)
		return out, nil
	}

	key, err := s.ResolveTokenKey(ctx, found.ID)
	if err != nil {
		return out, fmt.Errorf("创建后读取 token key 失败: %v", err)
	}
	out.Key = key
	out.Token = ensureTokenKey(out.Token, key)
	return out, nil
}

func (s *Service) CreateTokens(ctx context.Context, count int, body map[string]any) (newapi.Response, map[string]any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return newapi.Response{}, nil, err
	}
	return client.Request(ctx, http.MethodPost, fmt.Sprintf("/api/token/tokens?tokenCount=%d", count), body)
}

func (s *Service) ResolveTokenKey(ctx context.Context, id int) (string, error) {
	var batchErr error
	if keys, err := s.GetTokenKeysBatch(ctx, []int{id}); err == nil {
		if key := strings.TrimSpace(keys[id]); key != "" {
			return key, nil
		}
	} else {
		batchErr = err
	}
	var singleErr error
	key, err := s.GetTokenKey(ctx, id)
	if err != nil {
		singleErr = err
	} else {
		return key, nil
	}
	token, detailErr := s.GetToken(ctx, id)
	if detailErr == nil {
		if key := fullUnmaskedKey(token.Key); key != "" {
			return key, nil
		}
		detailErr = fmt.Errorf("Token %d detail key is empty or masked", id)
	}
	if batchErr != nil {
		return "", fmt.Errorf("batch: %v; single: %v; detail: %v", batchErr, singleErr, detailErr)
	}
	return "", fmt.Errorf("single: %v; detail: %v", singleErr, detailErr)
}

func (s *Service) GetTokenKey(ctx context.Context, id int) (string, error) {
	client, err := s.configuredClient()
	if err != nil {
		return "", err
	}
	res, data, err := client.Request(ctx, http.MethodPost, fmt.Sprintf("/api/token/%d/key", id), nil)
	if err != nil {
		return "", err
	}
	if !res.OK() {
		return "", fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("Token %d key 无法访问", id)))
	}
	if !newapi.IsSuccess(data) {
		return "", fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("Token %d key 获取失败", id)))
	}
	key := tokenKeyFromPayload(data)
	if key == "" {
		return "", fmt.Errorf("Token %d key 响应异常", id)
	}
	return EnsureFullKey(key), nil
}

func tokenFromCreatedPayload(data map[string]any) (Token, bool) {
	if raw, ok := data["data"].(map[string]any); ok && tokenPayloadHasIdentity(raw) {
		return FromRaw(raw), true
	}
	if tokenPayloadHasIdentity(data) {
		return FromRaw(data), true
	}
	if items := DataList(data); len(items) > 0 {
		return FromRaw(items[0]), true
	}
	return Token{}, false
}

func tokenPayloadHasIdentity(raw map[string]any) bool {
	return getString(raw, "key") != "" || getString(raw, "name") != "" || toInt(raw["id"]) > 0
}

func fullUnmaskedKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" || IsMaskedKey(key) {
		return ""
	}
	return EnsureFullKey(key)
}

func ensureTokenKey(token Token, key string) Token {
	key = EnsureFullKey(key)
	token.Key = key
	if token.Raw == nil {
		token.Raw = map[string]any{}
	}
	token.Raw["key"] = key
	return token
}

func (s *Service) GetTokenKeysBatch(ctx context.Context, ids []int) (map[int]string, error) {
	client, err := s.configuredClient()
	if err != nil {
		return nil, err
	}
	clean := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			clean = append(clean, id)
		}
	}
	if len(clean) == 0 {
		return map[int]string{}, nil
	}
	res, data, err := client.Request(ctx, http.MethodPost, "/api/token/batch/keys", map[string]any{"ids": clean})
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, "批量读取 token key 失败"))
	}
	if !newapi.IsSuccess(data) {
		return nil, fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, "批量读取 token key 失败"))
	}
	keys := tokenKeysMapFromPayload(data)
	if len(keys) == 0 {
		return nil, fmt.Errorf("批量读取 token key 响应异常")
	}
	return keys, nil
}

func tokenKeyFromPayload(data map[string]any) string {
	if value := getString(data, "key"); value != "" {
		return value
	}
	if nested, ok := data["data"].(map[string]any); ok {
		return getString(nested, "key")
	}
	return ""
}

func tokenKeysMapFromPayload(data map[string]any) map[int]string {
	if keys := tokenKeysMap(data["keys"]); len(keys) > 0 {
		return keys
	}
	if nested, ok := data["data"].(map[string]any); ok {
		return tokenKeysMap(nested["keys"])
	}
	return map[int]string{}
}

func tokenKeysMap(value any) map[int]string {
	out := map[int]string{}
	if raw, ok := value.(map[string]any); ok {
		for id, key := range raw {
			n := toInt(id)
			if n <= 0 {
				continue
			}
			if text := strings.TrimSpace(fmt.Sprint(key)); text != "" && text != "<nil>" {
				out[n] = EnsureFullKey(text)
			}
		}
	}
	return out
}

func (s *Service) UpdateTokenRaw(ctx context.Context, raw map[string]any) (newapi.Response, map[string]any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return newapi.Response{}, nil, err
	}
	return client.Request(ctx, http.MethodPut, "/api/token/", raw)
}

func (s *Service) DeleteToken(ctx context.Context, id int) (bool, newapi.Response, error) {
	client, err := s.configuredClient()
	if err != nil {
		return false, newapi.Response{}, err
	}
	res, data, err := client.Request(ctx, http.MethodDelete, fmt.Sprintf("/api/token/%d", id), nil)
	if err != nil {
		return false, newapi.Response{}, err
	}
	if !res.OK() {
		return false, res, nil
	}
	if !newapi.IsSuccess(data) {
		return false, res, fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("Token %d 删除失败", id)))
	}
	return true, res, nil
}

func (s *Service) AddQuota(ctx context.Context, key string, dollars int64) error {
	t, err := s.SearchTokenByKey(ctx, key)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("token not found on fufu")
	}
	add := s.DollarsToQuota(float64(dollars))
	raw := map[string]any{}
	for k, v := range t.Raw {
		raw[k] = v
	}
	raw["remain_quota"] = t.RemainQuota + add
	if name := getString(raw, "name"); name != "" {
		if truncated := truncateTokenName(name, 30); truncated != name {
			raw["name"] = truncated
		}
	}
	res, data, err := s.UpdateTokenRaw(ctx, raw)
	if err != nil {
		return err
	}
	if !res.OK() {
		return fmt.Errorf("fufu PUT failed: %s", res.BodyOr(strconv.Itoa(res.StatusCode)))
	}
	if !newapi.IsSuccess(data) {
		return fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, "fufu quota update failed"))
	}
	return nil
}

func (s *Service) DollarsToQuota(dollars float64) int64 {
	unit := int64(0)
	if s != nil {
		unit = s.QuotaUnit
	}
	if unit <= 0 {
		unit = newapi.DefaultQuotaUnit
	}
	return int64(math.Round(dollars * float64(unit)))
}

func (s *Service) QuotaToDollars(quota int64) float64 {
	unit := int64(0)
	if s != nil {
		unit = s.QuotaUnit
	}
	if unit <= 0 {
		unit = newapi.DefaultQuotaUnit
	}
	return float64(quota) / float64(unit)
}
