package main

import "net/http"

type networkAPIRouteSpec struct {
	Method string
	Match  func(string) bool
}

var networkAPIRoutes = []networkAPIRouteSpec{
	{Method: http.MethodGet, Match: exactNetworkAPIPath("/api/health")},
	{Method: http.MethodGet, Match: exactNetworkAPIPath("/api/client")},
	{Method: http.MethodGet, Match: exactNetworkAPIPath("/api/connectivity/targets")},
	{Method: http.MethodGet, Match: exactNetworkAPIPath("/api/newapi/sites")},
	{Method: http.MethodGet, Match: exactNetworkAPIPath("/api/newapi/model-status")},
	{Method: http.MethodGet, Match: exactNetworkAPIPath("/api/newapi/overview")},
	{Method: http.MethodPost, Match: exactNetworkAPIPath("/api/newapi/model-status/test")},
}

func exactNetworkAPIPath(path string) func(string) bool {
	return func(got string) bool { return got == path }
}

func findNetworkAPIPath(path string) (networkAPIRouteSpec, bool) {
	for _, route := range networkAPIRoutes {
		if route.Match(path) {
			return route, true
		}
	}
	return networkAPIRouteSpec{}, false
}
