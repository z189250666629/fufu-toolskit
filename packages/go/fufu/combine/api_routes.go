package combine

import (
	"net/http"
	"strings"
)

type apiRouteSpec struct {
	Method string
	Public bool
	Match  func(string) bool
}

var apiRoutes = []apiRouteSpec{
	{Method: http.MethodPost, Public: true, Match: exactAPIPath("/api/auth")},
	{Method: http.MethodGet, Match: exactAPIPath("/api/session")},
	{Method: http.MethodPost, Public: true, Match: exactAPIPath("/api/search-keys")},
	{Method: http.MethodPost, Match: exactAPIPath("/api/merge")},
	{Method: http.MethodPost, Public: true, Match: exactAPIPath("/api/public-merge")},
	{Method: http.MethodGet, Public: true, Match: prefixAPIPath("/api/merge-status/")},
	{Method: http.MethodPost, Match: exactAPIPath("/api/generate")},
	{Method: http.MethodDelete, Match: prefixAPIPath("/api/token/")},
}

func exactAPIPath(path string) func(string) bool {
	return func(got string) bool { return got == path }
}

func prefixAPIPath(prefix string) func(string) bool {
	return func(got string) bool { return strings.HasPrefix(got, prefix) }
}

func findAPIPath(path string) (apiRouteSpec, bool) {
	for _, route := range apiRoutes {
		if route.Match(path) {
			return route, true
		}
	}
	return apiRouteSpec{}, false
}

func IsAPIPath(path string) bool {
	_, ok := findAPIPath(path)
	return ok
}

func APIMethod(path string) (string, bool) {
	route, ok := findAPIPath(path)
	if !ok {
		return "", false
	}
	return route.Method, true
}
