package activityapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

var errInvalidUserID = errors.New("invalid user id")

type loginRequest struct {
	CardKey  *string `json:"cardKey"`
	UserID   any     `json:"userId"`
	Username *string `json:"username"`
}

type loginIdentity struct {
	CardKey          string
	CardKeyProvided  bool
	UserID           int64
	UserIDProvided   bool
	Username         string
	UsernameProvided bool
}

func readLoginIdentity(r *http.Request) (loginIdentity, error) {
	var body loginRequest
	if err := readBody(r, &body); err != nil {
		return loginIdentity{}, err
	}

	var ident loginIdentity
	if body.CardKey != nil {
		ident.CardKeyProvided = true
		ident.CardKey = strings.TrimSpace(*body.CardKey)
		if ident.CardKey != "" && !isValidCardKey(ident.CardKey) {
			return loginIdentity{}, errInvalidCardKey
		}
	}
	userID, provided, err := parseLoginUserID(body.UserID)
	if err != nil {
		return loginIdentity{}, err
	}
	ident.UserID = userID
	ident.UserIDProvided = provided
	if body.Username != nil {
		ident.UsernameProvided = true
		ident.Username = strings.TrimSpace(*body.Username)
	}
	return ident, nil
}

func parseLoginUserID(value any) (int64, bool, error) {
	switch v := value.(type) {
	case nil:
		return 0, false, nil
	case float64:
		if v == 0 {
			return 0, false, nil
		}
		if !isFinitePositiveInteger(v) {
			return 0, true, errInvalidUserID
		}
		return int64(v), true, nil
	case string:
		return parseLoginUserIDString(v)
	case json.Number:
		return parseLoginUserIDString(v.String())
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "" || text == "<nil>" {
			return 0, false, nil
		}
		return parseLoginUserIDString(text)
	}
}

func parseLoginUserIDString(value string) (int64, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, nil
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, true, errInvalidUserID
	}
	return id, true, nil
}

func isFinitePositiveInteger(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0 && math.Trunc(value) == value
}

func writeLoginRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInvalidUserID):
		writeJSONError(w, http.StatusBadRequest, "用户ID格式错误")
	default:
		writeCardKeyRequestError(w, err)
	}
}

func writeMissingLoginIdentity(w http.ResponseWriter) {
	writeJSONError(w, http.StatusBadRequest, "请输入用户ID和用户名")
}
