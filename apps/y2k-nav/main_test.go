package main

import "testing"

func TestDefaultPortConstant(t *testing.T) {
	if defaultPort == "" {
		t.Fatal("default port should be set")
	}
}
