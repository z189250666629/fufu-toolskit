package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func AdminToken(configured, fallback string) string {
	return FirstNonEmpty(configured, fallback)
}

func CheckAdminToken(got, configured, fallback string) bool {
	expected := AdminToken(configured, fallback)
	got = strings.TrimSpace(got)
	if expected == "" || got == "" {
		return false
	}
	gotSum := sha256.Sum256([]byte(got))
	expectedSum := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(gotSum[:], expectedSum[:]) == 1
}
