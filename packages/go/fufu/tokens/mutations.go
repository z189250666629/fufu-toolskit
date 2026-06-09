package tokens

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"fufu/newapi"
)

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
