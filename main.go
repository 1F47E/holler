package main

import (
	"os"

	"github.com/1F47E/holler/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
