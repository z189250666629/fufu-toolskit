package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetModelStatusInvalidatesWhenManagedSitesConfigChanges(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	t.Cleanup(func() {
		rootDir = oldRootDir
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.Unlock()
	clearManagedSiteEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
	}))
	defer server.Close()

	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON("site-a", server.URL))
	first := getModelStatus(false)
	if len(first.Sites) != 1 || first.Sites[0].Site.Name != "site-a" {
		t.Fatalf("first status sites = %#v", first.Sites)
	}

	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON("site-b", server.URL))
	second := getModelStatus(false)
	if len(second.Sites) != 1 || second.Sites[0].Site.Name != "site-b" {
		t.Fatalf("model status cache should not survive managed-site config changes: %#v", second.Sites)
	}
}

func managedSiteConfigJSON(name, url string) string {
	return fmt.Sprintf(`[{"name":%q,"url":%q,"token":"token"}]`, name, strings.TrimRight(url, "/"))
}
