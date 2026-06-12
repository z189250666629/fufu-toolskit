package tokens

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"fufu/newapi"
	"sync"
)

func (s *Service) SearchTokenByKey(ctx context.Context, key string) (*Token, error) {
	if s == nil || s.Client == nil {
		return nil, fmt.Errorf("token service is not configured")
	}
	bare := BareKey(key)
	if bare == "" {
		return nil, nil
	}
	endpoint := "/api/token/search?keyword=&token=" + url.QueryEscape(bare) + "&p=0&size=10"
	res, data, err := s.Client.Request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, fmt.Errorf("查询 %s 失败", DisplayKey(key))
	}
	if !newapi.IsSuccess(data) {
		return nil, errors.New(newapi.ErrorMessage(data, res.StatusCode, "查询 "+DisplayKey(key)+" 失败"))
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
	if !newapi.IsSuccess(data) {
		return nil, errors.New(newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("查询 token 名称 %q 失败", name)))
	}
	for _, item := range DataList(data) {
		if getString(item, "name") == name {
			t := FromRaw(item)
			return &t, nil
		}
	}
	return nil, nil
}

// CountTokensByName reports how many tokens currently match the given name on
// the NewAPI site. NewAPI's /api/token/search applies a LIKE filter on the
// token name and returns the precise count in the paginated payload's total
// field, so a single size=1 request yields the stock without walking pages.
// Used by 补卡 to compute how many cards to top up (target − current).
func (s *Service) CountTokensByName(ctx context.Context, name string) (int, error) {
	if s == nil || s.Client == nil {
		return 0, fmt.Errorf("token service is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	endpoint := "/api/token/search?keyword=" + url.QueryEscape(name) + "&p=0&size=1"
	res, data, err := s.Client.Request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	if !res.OK() {
		return 0, fmt.Errorf("查询 token 名称 %q 库存失败", name)
	}
	if !newapi.IsSuccess(data) {
		return 0, errors.New(newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("查询 token 名称 %q 库存失败", name)))
	}
	if total, ok := payloadTotal(data); ok {
		return total, nil
	}
	// Older flat {data:[...]} payloads carry no total; fall back to the page
	// length (callers request size=1, so this only fires on tiny inventories).
	return len(DataList(data)), nil
}

type SearchResult struct {
	Key   string
	Found *Token
}

func (s *Service) BatchSearch(ctx context.Context, raw []string) ([]string, []Token, []string, error) {
	if s == nil || s.Client == nil {
		return nil, nil, nil, fmt.Errorf("token service is not configured")
	}
	keys := NormalizeKeys(raw)
	if len(keys) == 0 {
		return keys, nil, nil, nil
	}
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
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
				found, err := s.SearchTokenByKey(searchCtx, keys[idx])
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				results[idx] = SearchResult{Key: keys[idx], Found: found}
			}
		}()
	}
	for i := range keys {
		select {
		case jobs <- i:
		case err := <-errCh:
			cancel()
			close(jobs)
			wg.Wait()
			return nil, nil, nil, err
		case <-searchCtx.Done():
			close(jobs)
			wg.Wait()
			select {
			case err := <-errCh:
				return nil, nil, nil, err
			default:
			}
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
	found, missing := splitSearchResults(results)
	return keys, found, missing, nil
}
