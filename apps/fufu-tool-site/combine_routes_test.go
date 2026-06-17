package main

import (
	"fufu/combine"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeCombineHandler struct {
	normalCalled  bool
	trustedCalled bool
	trustedRole   combine.Role
}

func (h *fakeCombineHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.normalCalled = true
	writeJSON(w, http.StatusUnauthorized, map[string]any{"mode": "normal"})
}

func (h *fakeCombineHandler) ServeHTTPAsRole(w http.ResponseWriter, r *http.Request, role combine.Role) {
	h.trustedCalled = true
	h.trustedRole = role
	writeJSON(w, http.StatusOK, map[string]any{"mode": "trusted", "role": role})
}

func TestServeCombineAPIUsesUnifiedAdminSessionAsCombineAdmin(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", temporaryAdminLoginPassword)
	previous := combineApp
	fake := &fakeCombineHandler{}
	combineApp = fake
	t.Cleanup(func() { combineApp = previous })

	req := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	req.AddCookie(newUnifiedAdminSessionCookie(time.Now()))
	rec := httptest.NewRecorder()

	serveCombineAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !fake.trustedCalled || fake.normalCalled || fake.trustedRole != combine.RoleAdmin {
		t.Fatalf("trusted=%v normal=%v role=%q", fake.trustedCalled, fake.normalCalled, fake.trustedRole)
	}
}

func TestServeCombineAPIKeepsOriginalCombineAuthWithoutUnifiedSession(t *testing.T) {
	previous := combineApp
	fake := &fakeCombineHandler{}
	combineApp = fake
	t.Cleanup(func() { combineApp = previous })

	req := httptest.NewRequest(http.MethodPost, "/api/generate", nil)
	rec := httptest.NewRecorder()

	serveCombineAPI(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fake.trustedCalled || !fake.normalCalled {
		t.Fatalf("trusted=%v normal=%v", fake.trustedCalled, fake.normalCalled)
	}
}
