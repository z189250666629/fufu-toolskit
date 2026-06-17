package newapi

import (
	"fmt"
	"strings"
)

func messageFromPayload(data map[string]any) (string, bool) {
	for _, key := range []string{"message", "error"} {
		if value := cleanPayloadMessage(data[key]); value != "" {
			return value, true
		}
	}
	if nested, ok := data["data"].(map[string]any); ok {
		for _, key := range []string{"message", "error"} {
			if value := cleanPayloadMessage(nested[key]); value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func cleanPayloadMessage(value any) string {
	if obj, ok := value.(map[string]any); ok {
		for _, key := range []string{"message", "error"} {
			if text := cleanPayloadMessage(obj[key]); text != "" {
				return text
			}
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}
