package combine

import (
	"errors"
	"fmt"
	"fufu/webutil"
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

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	writeBadJSONRequest(w)
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, out any) error {
	return webutil.DecodeJSON(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes), out, webutil.WithUseNumber())
}
