package rawconv

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Int converts raw API values into an int using the same truncation semantics as Int64.
func Int(v any) int { return int(Int64(v)) }

// Int64 converts raw API numeric values into int64.
//
// JSON numbers and strings may contain either integer or decimal representations;
// decimal values are truncated toward zero to match Go's numeric casts.
func Int64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint:
		if uint64(x) <= math.MaxInt64 {
			return int64(x)
		}
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		if x <= math.MaxInt64 {
			return int64(x)
		}
	case float32:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		return numberStringInt64(x.String())
	case string:
		return numberStringInt64(x)
	default:
		return numberStringInt64(fmt.Sprint(x))
	}
	return 0
}

// IntDefault keeps the existing raw-default semantics: nil means absent and
// uses fallback; any present value, including an explicit zero or invalid value,
// is converted normally.
func IntDefault(v any, fallback int) int {
	if v == nil {
		return fallback
	}
	return Int(v)
}

func numberStringInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}
