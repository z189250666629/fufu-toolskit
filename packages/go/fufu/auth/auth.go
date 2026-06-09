package auth

import "strings"

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
	return expected != "" && strings.TrimSpace(got) == expected
}
