package activityapp

import (
	"errors"
	"net/http"
	"time"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	ident, err := readLoginIdentity(r)
	if err != nil {
		writeLoginRequestError(w, err)
		return
	}

	if ident.CardKey != "" {
		handleCardKeyLogin(w, r, ident.CardKey)
		return
	}
	if ident.CardKeyProvided && !ident.UserIDProvided && !ident.UsernameProvided {
		writeMissingCardKey(w)
		return
	}
	if !ident.UserIDProvided || ident.Username == "" {
		writeMissingLoginIdentity(w)
		return
	}

	card, err := loginWithSubscriptionIdentity(r, ident.UserID, ident.Username)
	if err != nil {
		writeLoginError(w, err)
		return
	}
	respondCard(w, card)
}

func handleCardKeyLogin(w http.ResponseWriter, r *http.Request, key string) {
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
		var err error
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
