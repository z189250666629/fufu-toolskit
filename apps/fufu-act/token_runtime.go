package activityapp

import (
	"sync"

	"fufu/tokens"
)

var tokenRuntimeMu sync.RWMutex

func setTokenRuntime(service *tokens.Service, configErr error) {
	tokenRuntimeMu.Lock()
	tokenSvc = service
	tokenConfigErr = configErr
	tokenRuntimeMu.Unlock()
}

func snapshotTokenRuntime() (*tokens.Service, error) {
	tokenRuntimeMu.RLock()
	defer tokenRuntimeMu.RUnlock()
	return tokenSvc, tokenConfigErr
}
