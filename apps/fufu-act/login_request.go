package activityapp

import "net/http"

type loginCardKeyRequest struct {
	CardKey string `json:"cardKey"`
}

func readLoginCardKey(r *http.Request) (string, bool, error) {
	var body loginCardKeyRequest
	return readCardKeyRequest(r, &body, func() string { return body.CardKey })
}
