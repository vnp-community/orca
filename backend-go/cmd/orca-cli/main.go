// Command orca-cli is a thin REST client of api-gateway for CLI/CI callers
// that can't drive the browser-only WS JSON-RPC surface (BUG-CLI-01).
package main

import (
	"os"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/command"
)

func main() {
	os.Exit(command.Run())
}
