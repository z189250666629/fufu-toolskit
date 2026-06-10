package webutil

import (
	"encoding/json"
	"errors"
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
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}
