// Package tenantclient reads the two things ResolveAgentMcpConfig needs from
// tenant-service: the merged profile's MCP server NAMES and the user's teams.
// tenant-service itself (including mergeMCPServers) is not modified.
package tenantclient

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

const DefaultTimeout = 3 * time.Second

func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

type Client struct {
	api     tenantv1.TenantServiceClient
	timeout time.Duration
}

var _ usecase.ProfileMcpReader = (*Client)(nil)

func New(api tenantv1.TenantServiceClient, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{api: api, timeout: timeout}
}

func (c *Client) call(ctx context.Context, userID string) (context.Context, context.CancelFunc) {
	if t, ok := tenant.TenantID(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, t)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataUserID, userID)
	return context.WithTimeout(ctx, c.timeout)
}

// ServerNames returns mcp.servers[].name from the resolved profile and nothing
// else: inline command/args/env in profile entries are discarded by design.
func (c *Client) ServerNames(ctx context.Context, userID string) ([]string, error) {
	ctx, cancel := c.call(ctx, userID)
	defer cancel()
	resp, err := c.api.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return ParseServerNames(resp.GetResolvedSettingsJson()), nil
}

// ParseServerNames extracts names from {"mcp":{"servers":[{"name":...}]}}; any
// other shape yields no names (fail closed).
func ParseServerNames(settingsJSON string) []string {
	var doc struct {
		MCP struct {
			Servers []struct {
				Name string `json:"name"`
			} `json:"servers"`
		} `json:"mcp"`
	}
	if json.Unmarshal([]byte(settingsJSON), &doc) != nil {
		return nil
	}
	var out []string
	for _, s := range doc.MCP.Servers {
		if s.Name != "" {
			out = append(out, s.Name)
		}
	}
	return out
}

func (c *Client) TeamIDs(ctx context.Context, userID string) ([]string, error) {
	ctx, cancel := c.call(ctx, userID)
	defer cancel()
	resp, err := c.api.ListTeamsForUser(ctx, &tenantv1.ListTeamsForUserRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return resp.GetTeamIds(), nil
}
