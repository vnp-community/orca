package main

import (
	"google.golang.org/grpc"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// newMCPRegistryClient returns the external server registry client over the
// mcp-service connection, or a true nil interface when MCP is not configured
// (a typed-nil would defeat the channels' "not configured" check).
func newMCPRegistryClient(conn *grpc.ClientConn) mcpv1.McpRegistryServiceClient {
	if conn == nil {
		return nil
	}
	return mcpv1.NewMcpRegistryServiceClient(conn)
}
