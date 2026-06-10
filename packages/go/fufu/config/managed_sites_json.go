package config

import (
	"fufu/webutil"
	"strings"
)

func decodeManagedSitesJSON(raw string) (any, error) {
	var data any
	if err := webutil.DecodeJSON(strings.NewReader(raw), &data, webutil.WithUseNumber()); err != nil {
		return nil, err
	}
	return data, nil
}
