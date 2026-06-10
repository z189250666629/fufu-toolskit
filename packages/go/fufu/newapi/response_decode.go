package newapi

import (
	"bytes"
	"fmt"

	"fufu/webutil"
)

func decodeResponsePayload(body []byte) (map[string]any, error) {
	decoded := map[string]any{}
	if len(bytes.TrimSpace(body)) == 0 {
		return decoded, nil
	}
	if err := webutil.DecodeJSON(bytes.NewReader(body), &decoded, webutil.WithUseNumber()); err != nil {
		return nil, fmt.Errorf("decode NewAPI response JSON: %w", err)
	}
	return decoded, nil
}
