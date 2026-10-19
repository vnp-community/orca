package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// completeRelayClient gives ai.complete what relay() gives every git and fs call, because agentrelay sees only the
// transport: a dev server whose agent dropped after dispatch is a retryable DEV_SERVER_UNREACHABLE, and a JSON-RPC
// error the agent returns inside an "ok" relay answer is an error, not an empty completion.
type completeRelayClient struct {
	client infrafleetv1.InfraFleetServiceClient
}

func (c completeRelayClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	resp, err := c.client.Relay(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	if rejected := agentRejection(in.GetMethod(), resp.GetResultJson()); rejected != nil {
		return nil, rejected
	}
	return resp, nil
}

func (c completeRelayClient) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	resp, err := c.client.RelayByDevServer(ctx, in, opts...)
	if err != nil {
		// Fleet-health samples lag up to 30s, so the agent can drop after dispatch.
		if status.Code(err) == codes.FailedPrecondition && strings.Contains(status.Convert(err).Message(), "INFRA_DEV_SERVER_NOT_CONNECTED") {
			return nil, usecase.ErrDevServerUnreachable(in.GetDevServerId())
		}
		return nil, err
	}
	if rejected := agentRejection(in.GetMethod(), resp.GetResultJson()); rejected != nil {
		return nil, rejected
	}
	return resp, nil
}

// agentRejection reads the {error:{code,message}} envelope the agent answers with when it refuses a call. The code is a
// json.Number because the agent's codes are numeric JSON-RPC codes (-32000, ...).
func agentRejection(method, resultJSON string) error {
	if resultJSON == "" {
		return nil
	}
	var envelope struct {
		Error *struct {
			Code    json.Number `json:"code"`
			Message string      `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(resultJSON), &envelope) != nil || envelope.Error == nil {
		return nil
	}
	return fmt.Errorf("grpcclient: agent rejected %s: %s (%s)", method, envelope.Error.Message, envelope.Error.Code)
}
