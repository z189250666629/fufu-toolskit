package config

import (
	"encoding/json"
	"strings"
)

func decodeManagedSitesJSON(raw string) (any, error) {
	var data any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}
