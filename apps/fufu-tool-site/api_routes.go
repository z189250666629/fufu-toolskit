package main

import "net/http"

type toolAPIRouteSpec struct {
	Method string
	Match  func(string) bool
}

var toolAPIRoutes = []toolAPIRouteSpec{
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/health")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/client")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/connectivity/targets")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/newapi/sites")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/nav/lines")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/nav/tools")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/newapi/model-status")},
	{Method: http.MethodGet, Match: exactToolAPIPath("/api/newapi/overview")},
	{Method: http.MethodPost, Match: exactToolAPIPath("/api/newapi/model-status/test")},
}

func exactToolAPIPath(path string) func(string) bool {
	return func(got string) bool { return got == path }
}

func findToolAPIPath(path string) (toolAPIRouteSpec, bool) {
	for _, route := range toolAPIRoutes {
		if route.Match(path) {
			return route, true
		}
	}
	return toolAPIRouteSpec{}, false
}
