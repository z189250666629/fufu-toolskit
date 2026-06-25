package main

import (
	"fmt"
	activityapp "fufu-act"
	"net/http"
	"os"

	"fufu/webutil"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve working directory: %v\n", err)
		os.Exit(1)
	}
	if err := run(wd, os.Getenv("PORT")); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(wd, portValue string) error {
	port, err := webutil.ResolvePort(portValue, defaultPort)
	if err != nil {
		return fmt.Errorf("invalid PORT: %w", err)
	}
	if err := initRuntime(wd); err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", route)
	activityapp.StartWorkers()
	fmt.Printf("fufu tool site listening on :%s\n", port)
	if err := serve(port, mux); err != nil {
		return fmt.Errorf("server stopped: %w", err)
	}
	return nil
}
