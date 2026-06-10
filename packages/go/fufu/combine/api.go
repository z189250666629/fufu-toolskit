package combine

import (
	"context"
	"fufu/tokens"
	"sync"
)

func (a *App) searchTokensConcurrent(ctx context.Context, keys []string) ([]SearchTokenResult, error) {
	results := make([]SearchTokenResult, len(keys))
	if len(keys) == 0 {
		return results, nil
	}
	jobs := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < min(searchConcurrency, len(keys)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				found, err := a.searchTokenByKey(ctx, keys[idx])
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}
				results[idx] = SearchTokenResult{Key: keys[idx], Found: found}
			}
		}()
	}
	for i := range keys {
		select {
		case jobs <- i:
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, err
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
		return results, nil
	}
}

func (a *App) searchTokenByKey(ctx context.Context, key string) (*ResolvedToken, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	found, err := a.tokenSvc.SearchTokenByKey(ctx, key)
	if err != nil || found == nil {
		return nil, err
	}
	resolved := resolvedFromToken(*found)
	if id, ok, err := a.generatedTokenIDByKey(ctx, key); err != nil {
		return nil, err
	} else if ok && resolved.ID == 0 {
		token, err := a.fetchVerifiedToken(ctx, id)
		if err == nil {
			return &token, nil
		}
	}
	return &resolved, nil
}

func (a *App) fetchVerifiedToken(ctx context.Context, id int) (ResolvedToken, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	token, err := a.tokenSvc.GetToken(ctx, id)
	if err != nil {
		return ResolvedToken{}, err
	}
	return resolvedFromToken(token), nil
}

func (a *App) deleteToken(ctx context.Context, id int) (bool, APIResponse, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.DeleteToken(ctx, id)
}

func (a *App) createToken(ctx context.Context, body map[string]any) (APIResponse, map[string]any, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.CreateToken(ctx, body)
}

func (a *App) updateTokenRaw(ctx context.Context, raw map[string]any) (APIResponse, map[string]any, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.UpdateTokenRaw(ctx, raw)
}

func (a *App) searchTokenByName(ctx context.Context, name string) (*tokens.Token, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.SearchTokenByName(ctx, name)
}
