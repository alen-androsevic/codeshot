// Command codeshot renders a command and its output as a picture of a terminal
// window. This file exists to choose the adapters and get out of the way.
package main

import (
	"os"

	"codeshot/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
