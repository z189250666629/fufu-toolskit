package main

import (
	"fmt"
	activityapp "fufu-act"
	"os"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve working directory: %v\n", err)
		os.Exit(1)
	}
	if err := activityapp.Run(wd, os.Getenv("SLOT_PORT")); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}
