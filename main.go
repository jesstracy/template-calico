package main

import (
	"errors"
	"fmt"
	"os"
)

// Needs to take in release name, release namespace, values file, and calico-templates
func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if (len(args) < 4) {
		return errors.New(fmt.Sprintf("incorrect number of arguments.\n%s", usage()))
	}
	return nil
}

func usage() string {
	return "Usage:\n    calico-template <release-name> <namespace> <values-file> <calico-template-file>"
}
