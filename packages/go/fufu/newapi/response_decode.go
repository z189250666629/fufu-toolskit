package newapi

import (
	"bytes"
	"encoding/json"
)

func decodeResponsePayload(body []byte) map[string]any {
	decoded := map[string]any{}
	if len(bytes.TrimSpace(body)) == 0 {
		return decoded
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	_ = dec.Decode(&decoded)
	return decoded
}
