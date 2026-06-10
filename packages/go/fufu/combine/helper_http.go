package combine

import (
	"encoding/json"
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

func writeBadJSONRequest(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
}

func decodeJSON(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(out)
}
