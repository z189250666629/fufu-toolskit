package activityapp

import "testing"

func TestMCYRuntimeConfigOverridesEnv(t *testing.T) {
	t.Setenv("MCY_BASE_URL", "https://env.example.test")
	t.Setenv("MCY_USERNAME", "envuser")
	t.Setenv("MCY_PASSWORD", "envpass")
	t.Cleanup(func() { SetMCYRuntimeConfig(MCYRuntimeConfig{}) })

	// No admin config → fall back to env.
	SetMCYRuntimeConfig(MCYRuntimeConfig{})
	base, user, pass, login := mcyConfig()
	if base != "https://env.example.test" || user != "envuser" || pass != "envpass" || login != "/admin/login" {
		t.Fatalf("env fallback wrong: base=%q user=%q pass=%q login=%q", base, user, pass, login)
	}

	// Admin-supplied config overrides env (and trims a trailing slash).
	SetMCYRuntimeConfig(MCYRuntimeConfig{BaseURL: "https://admin.example.test/", Username: "adminuser", Password: "adminpass"})
	base, user, pass, _ = mcyConfig()
	if base != "https://admin.example.test" || user != "adminuser" || pass != "adminpass" {
		t.Fatalf("admin override wrong: base=%q user=%q pass=%q", base, user, pass)
	}
	if !mcyConfigured() {
		t.Fatal("base url is set, should be configured")
	}
}

func TestMCYConfiguredFalseWithoutBaseURL(t *testing.T) {
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")
	SetMCYRuntimeConfig(MCYRuntimeConfig{})
	t.Cleanup(func() { SetMCYRuntimeConfig(MCYRuntimeConfig{}) })
	if mcyConfigured() {
		t.Fatal("no base url anywhere → not configured")
	}
}

func TestSetMCYRuntimeConfigDropsCookieOnCredentialChange(t *testing.T) {
	setMCYCookieForTest(t, "stale-cookie")
	t.Cleanup(func() { SetMCYRuntimeConfig(MCYRuntimeConfig{}) })

	SetMCYRuntimeConfig(MCYRuntimeConfig{BaseURL: "https://a.example.test", Username: "u", Password: "p"})
	if getMCYCookie() != "" {
		t.Fatal("changing credentials should drop the cached login cookie to force re-auth")
	}
}

func TestSetMCYRuntimeConfigDropsCachedCookieWhenExplicitCookieCleared(t *testing.T) {
	t.Cleanup(func() { SetMCYRuntimeConfig(MCYRuntimeConfig{}) })

	SetMCYRuntimeConfig(MCYRuntimeConfig{BaseURL: "https://a.example.test", Username: "u", Password: "p", Cookie: "manual-cookie"})
	if getMCYCookie() != "manual-cookie" {
		t.Fatalf("explicit cookie should be applied, got %q", getMCYCookie())
	}

	SetMCYRuntimeConfig(MCYRuntimeConfig{BaseURL: "https://a.example.test", Username: "u", Password: "p"})
	if getMCYCookie() != "" {
		t.Fatalf("clearing explicit cookie should drop cached cookie, got %q", getMCYCookie())
	}
}

func TestMCYUploadEndpointPrefersRuntimeThenEnvThenDefault(t *testing.T) {
	t.Setenv("MCY_UPLOAD_ENDPOINT", "")
	SetMCYRuntimeConfig(MCYRuntimeConfig{})
	t.Cleanup(func() { SetMCYRuntimeConfig(MCYRuntimeConfig{}) })
	if got := mcyUploadEndpoint(); got != "/plugin/virtual-card-ship/card/add" {
		t.Fatalf("default upload endpoint = %q", got)
	}
	SetMCYRuntimeConfig(MCYRuntimeConfig{UploadEndpoint: "/custom/add"})
	if got := mcyUploadEndpoint(); got != "/custom/add" {
		t.Fatalf("runtime upload endpoint = %q", got)
	}
}
