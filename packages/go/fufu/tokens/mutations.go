package tokens

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"fufu/newapi"
)

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

func (s *Service) CreateTokens(ctx context.Context, count int, body map[string]any) (newapi.Response, map[string]any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return newapi.Response{}, nil, err
	}
	return client.Request(ctx, http.MethodPost, fmt.Sprintf("/api/token/tokens?tokenCount=%d", count), body)
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
