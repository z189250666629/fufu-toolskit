package activityapp

import (
	"errors"
	"net/http"
)

func handleSpin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	key, ok, err := readCardKeyRequest(r, &body, func() string { return body.CardKey })
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	res, err := runSpinForCard(r, key)
	if err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}
