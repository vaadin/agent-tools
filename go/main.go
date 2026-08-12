// Command vaadin-agent-tools is a self-contained native CLI of Vaadin tools for
// AI agents (and humans). It is a Go port of the JavaScript implementation in
// ../src; the CLI surface (tool names, args, --json shape, exit codes) is the
// stable contract and is identical across both implementations.
package main

import (
	"os"

	"github.com/vaadin/agent-tools/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
