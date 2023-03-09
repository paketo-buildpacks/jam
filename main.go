package main

import (
	"fmt"
	"os"

	"github.com/paketo-buildpacks/jam/v2/commands"
)

func main() {
	err := commands.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to execute: %s", err)
		os.Exit(1)
	}
}
