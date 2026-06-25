package main

import (
	"errors"
	"net/http"
	"strings"
)

func handleModelTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteName     string `json:"siteName"`
		Model        string `json:"model"`
		Group        string `json:"group"`
		PreferredURL string `json:"preferredUrl"`
	}
	if err := readJSON(r, &body); err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
		writeJSONError(w, 400, "请求体无效")
		return
	}
	body.SiteName = strings.TrimSpace(body.SiteName)
	body.Model = strings.TrimSpace(body.Model)
	body.Group = strings.TrimSpace(body.Group)
	body.PreferredURL = strings.TrimSpace(body.PreferredURL)
	if body.SiteName == "" || body.Model == "" {
		writeJSONError(w, 400, "siteName 和 model 必填")
		return
	}
	ctx := r.Context()
	ctx = contextWithModelTestPreferredURL(ctx, body.PreferredURL)
	result, err := runModelTest(ctx, body.SiteName, body.Model, body.Group)
	if err != nil {
		var e *httpError
		if errors.As(err, &e) {
			writeJSON(w, e.Status, map[string]any{"error": e.Message, "nextAllowedAt": e.NextAllowedAt})
		} else {
			writeJSONError(w, 500, "模型测试失败，请稍后重试")
		}
		return
	}
	writeJSON(w, 200, result)
}
