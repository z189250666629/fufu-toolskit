package tokens

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

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
	found, missing := splitSearchResults(results)
	return keys, found, missing, nil
}
