// Command codeshot renders a command and its output as a picture of a terminal
// window. Everything happens in internal/cli, which parses the arguments and
// chooses the adapters; this file only hands over argv and passes the exit
// code back, so that the whole of a run stays reachable from a test.
package main

import (
	"os"

	"codeshot/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
