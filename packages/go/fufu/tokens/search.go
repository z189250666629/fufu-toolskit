package tokens

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

func (s *Service) SearchTokensByNamePrefix(ctx context.Context, prefix string, size int) ([]Token, error) {
	if s == nil || s.Client == nil {
		return nil, fmt.Errorf("token service is not configured")
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, nil
	}
	if size <= 0 {
		size = 10
	}
	endpoint := "/api/token/search?keyword=" + url.QueryEscape(prefix) + "&p=0&size=" + strconv.Itoa(size)
	res, data, err := s.Client.Request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if !res.OK() {
		return nil, fmt.Errorf("查询 token 名称前缀 %q 失败", prefix)
	}
	if !newapi.IsSuccess(data) {
		return nil, errors.New(newapi.ErrorMessage(data, res.StatusCode, fmt.Sprintf("查询 token 名称前缀 %q 失败", prefix)))
	}
	out := []Token{}
	for _, item := range DataList(data) {
		name := getString(item, "name")
		if name == prefix || strings.HasPrefix(name, prefix+"-") {
			out = append(out, FromRaw(item))
		}
	}
	return out, nil
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
