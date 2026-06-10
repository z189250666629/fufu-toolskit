package webutil

import (
	"encoding/json"
	"io"
)

type DecodeJSONOption func(*json.Decoder)

func WithUseNumber() DecodeJSONOption {
	return func(dec *json.Decoder) {
		dec.UseNumber()
	}
}

func DecodeJSON(r io.Reader, out any, opts ...DecodeJSONOption) error {
	dec := json.NewDecoder(r)
	for _, opt := range opts {
		opt(dec)
	}
	return dec.Decode(out)
}
