package combine

import (
	"fmt"
	"fufu/webutil"
	"io"
	"net/http"
)

func upstreamStatusMessage(r APIResponse, fallback string) string {
	if r.StatusCode > 0 {
		return fmt.Sprintf("%s（上游状态 %d）", fallback, r.StatusCode)
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	webutil.WriteJSONError(w, status, message)
}

func writeBadJSONRequest(w http.ResponseWriter) {
	writeJSONError(w, http.StatusBadRequest, "请求格式错误")
}

func decodeJSON(r io.Reader, out any) error {
	return webutil.DecodeJSON(r, out, webutil.WithUseNumber())
}
