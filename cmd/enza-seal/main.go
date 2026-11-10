package main

import (
	"fmt"
	"os"

	"github.com/kirollos-rafik/enza-seal/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=1.0.0".
var version = "dev"

func main() {
	if err := cli.NewRoot(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
