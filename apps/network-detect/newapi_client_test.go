package main

import (
	"encoding/json"
	"testing"
)

func TestToInt64ParsesDecimalJSONNumber(t *testing.T) {
	if got := toInt64(json.Number("42.0")); got != 42 {
		t.Fatalf("decimal json.Number = %d", got)
	}
}
