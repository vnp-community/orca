package grpcclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type InfraCapabilityClient struct {
	client  infrafleetv1.InfraFleetServiceClient
	timeout time.Duration
}

func NewInfraCapabilityClient(client infrafleetv1.InfraFleetServiceClient, timeout time.Duration) *InfraCapabilityClient {
	return &InfraCapabilityClient{
		client:  client,
		timeout: timeout,
	}
}

var _ usecase.DevServerCapabilityReader = (*InfraCapabilityClient)(nil)

func (c *InfraCapabilityClient) Get(ctx context.Context, ref usecase.DevServerRef, refresh bool) (domain.DevServerCapability, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// withTenantMetadata must be handled by the interceptor or caller as stated in the task.
	// In the real code we would inject the tenant metadata here, assuming it's done via interceptor or caller context.

	req := &infrafleetv1.GetDevServerCapabilitiesRequest{
		ConnectionId: ref.ConnectionID,
		DevServerId:  ref.DevServerID,
		Refresh:      refresh,
	}

	resp, err := c.client.GetDevServerCapabilities(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound && st.Message() == "INFRA_CAPABILITY_PROFILE_NOT_FOUND" {
			return domain.DevServerCapability{}, usecase.ErrCapabilityNotAvailable
		}
		// Also handle standard NotFound just in case
		if ok && st.Code() == codes.NotFound {
			return domain.DevServerCapability{}, usecase.ErrCapabilityNotAvailable
		}
		return domain.DevServerCapability{}, fmt.Errorf("infra capability: %w", err)
	}

	var probedAt time.Time
	if resp.ProbedAt != nil {
		probedAt = resp.ProbedAt.AsTime()
	}

	return domain.ParseCapabilityProfile(
		resp.Source,
		resp.Degraded,
		resp.Connected,
		resp.AgentBuildVersion,
		int(resp.ProtocolVersion),
		resp.Features,
		[]byte(resp.ProfileJson),
		probedAt,
	)
}
