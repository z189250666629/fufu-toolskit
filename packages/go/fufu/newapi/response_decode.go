package newapi

import (
	"bytes"

	"fufu/webutil"
)

func decodeResponsePayload(body []byte) map[string]any {
	decoded := map[string]any{}
	if len(bytes.TrimSpace(body)) == 0 {
		return decoded
	}
	if err := webutil.DecodeJSON(bytes.NewReader(body), &decoded, webutil.WithUseNumber()); err != nil {
		return map[string]any{}
	}
	return decoded
}
