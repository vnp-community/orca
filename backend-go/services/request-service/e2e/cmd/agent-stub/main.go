// Command agent-stub serves the stub agent boundary (infra-fleet Relay, project ListRepos, auth audit) on one
// gRPC port, so the dev stack can run request-service end to end with no AI and no dev server.
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/stablyai/orca-go/services/request-service/e2e/stubs"
)

func main() {
	addr := os.Getenv("AGENT_STUB_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-stub: listen %s: %v\n", addr, err)
		os.Exit(1)
	}
	slog.Info("agent-stub listening", slog.String("addr", addr))
	if err := stubs.NewServers().Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "agent-stub: %v\n", err)
		os.Exit(1)
	}
}
