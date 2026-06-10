package main

import (
	"errors"
	"strings"
	"testing"
)

func TestTestModelSurfacesManagedSiteConfigErrors(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `not-json`)

	_, err := testModel("missing-site", "gpt-test", "")
	var httpErr *httpError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected httpError, got %T %v", err, err)
	}
	if httpErr.Status != 500 {
		t.Fatalf("status = %d, message = %q", httpErr.Status, httpErr.Message)
	}
	if strings.Contains(httpErr.Message, "站点不存在") || !strings.Contains(httpErr.Message, "不是有效 JSON") {
		t.Fatalf("expected config JSON error, got %q", httpErr.Message)
	}
}

func clearManagedSiteEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"NEWAPI_MANAGED_API_SITES",
		"NEWAPI_MANAGED_API_CONFIG",
		"NEWAPI_API_SITE_URL",
		"NEWAPI_API_SITE_TOKEN",
		"NEWAPI_API_SITE_ACCESS_TOKEN",
		"NEWAPI_TOKEN_SITE_URL",
		"NEWAPI_TOKEN_SITE_TOKEN",
		"NEWAPI_TOKEN_SITE_ACCESS_TOKEN",
	} {
		t.Setenv(name, "")
	}
	for i := 1; i <= 10; i++ {
		prefix := "NEWAPI_MANAGED_SITE_" + string(rune('0'+i))
		if i == 10 {
			prefix = "NEWAPI_MANAGED_SITE_10"
		}
		t.Setenv(prefix+"_URL", "")
		t.Setenv(prefix+"_TOKEN", "")
		t.Setenv(prefix+"_ACCESS_TOKEN", "")
	}
}
