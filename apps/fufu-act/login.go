package activityapp

import (
	"errors"
	"net/http"
	"time"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	key, ok, err := readLoginCardKey(r)
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	card, ok, lookupErr := lookupCard(key)
	if lookupErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	if ok {
		if err := requireCurrentTokenActive(r.Context(), key); err != nil {
			writeLoginError(w, err)
			return
		}
	} else {
		card, err = createLoginCard(r.Context(), key, loginClientIP(r), time.Now())
		if err != nil {
			writeLoginError(w, err)
			return
		}
	}
	respondCard(w, card)
}

func writeLoginError(w http.ResponseWriter, err error) {
	var rateLimited loginRateLimitError
	if errors.As(err, &rateLimited) {
		writeUnknownLoginRateLimited(w, rateLimited.Until)
		return
	}
	writeHTTPError(w, err)
}
