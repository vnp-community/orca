# TASK-AWS-03-07: Add `CreateAgentToken`/`ListAgentTokens`/`RevokeAgentToken` RPCs

**From Solution:** SOL-AWS-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`, `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-AWS-03-05
**Status:** `[ ]` TODO

---

## Context

Exposes the three new usecases over gRPC so `api-gateway`'s `wscompat`
layer (TASK-AWS-03-08) can reach them. Additive-only proto change,
following this file's existing RPC/message style (see e.g.
`CreateSshTarget`/`CreateSshTargetResponse`).

## Changes to make

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, add to the
`InfraFleetService` service block (after `KillWorkspacePort`):

```protobuf
  // CreateAgentToken/ListAgentTokens/RevokeAgentToken back BL-AWS-03's
  // persistent, named, per-DevServer agent token admin surface — see
  // specs/backend-go/bugs/logic-v1/solutions/SOL-AWS-03-agent-token-management.md.
  rpc CreateAgentToken(CreateAgentTokenRequest) returns (CreateAgentTokenResponse);
  rpc ListAgentTokens(ListAgentTokensRequest) returns (ListAgentTokensResponse);
  rpc RevokeAgentToken(RevokeAgentTokenRequest) returns (google.protobuf.Empty);
```

Add messages (append near `GetFleetHealthResponse`):

```protobuf
message CreateAgentTokenRequest  { string dev_server_id = 1; string name = 2; }
message CreateAgentTokenResponse {
  string id = 1;
  string token = 2; // plaintext — shown once, never returned again
  string name = 3;
  int64 created_at_unix_ms = 4;
}

message AgentTokenSummary {
  string id = 1;
  string name = 2;
  int64 created_at_unix_ms = 3;
  optional int64 last_used_at_unix_ms = 4;
}
message ListAgentTokensRequest  { string dev_server_id = 1; }
message ListAgentTokensResponse { repeated AgentTokenSummary tokens = 1; }

message RevokeAgentTokenRequest { string dev_server_id = 1; string id = 2; }
```

Regenerate stubs, then add the three handlers to
`backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`,
following `CreateSshTarget`'s existing shape (`server.go:212`) — tenantID
is pulled from ctx *inside* the usecase (`tenant.RequireTenantID`), not
extracted in this handler, matching every other handler in this file:

```go
func (s *Server) CreateAgentToken(ctx context.Context, req *infrafleetv1.CreateAgentTokenRequest) (*infrafleetv1.CreateAgentTokenResponse, error) {
	plaintext, tok, err := s.createAgentToken.Execute(ctx, req.GetDevServerId(), req.GetName())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateAgentTokenResponse{
		Id: tok.ID, Token: plaintext, Name: tok.Name, CreatedAtUnixMs: tok.CreatedAt.UnixMilli(),
	}, nil
}

func (s *Server) ListAgentTokens(ctx context.Context, req *infrafleetv1.ListAgentTokensRequest) (*infrafleetv1.ListAgentTokensResponse, error) {
	summaries, err := s.listAgentTokens.Execute(ctx, req.GetDevServerId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.AgentTokenSummary, 0, len(summaries))
	for _, sum := range summaries {
		pb := &infrafleetv1.AgentTokenSummary{Id: sum.ID, Name: sum.Name, CreatedAtUnixMs: sum.CreatedAt.UnixMilli()}
		if sum.LastUsedAt != nil {
			ms := sum.LastUsedAt.UnixMilli()
			pb.LastUsedAtUnixMs = &ms
		}
		out = append(out, pb)
	}
	return &infrafleetv1.ListAgentTokensResponse{Tokens: out}, nil
}

func (s *Server) RevokeAgentToken(ctx context.Context, req *infrafleetv1.RevokeAgentTokenRequest) (*emptypb.Empty, error) {
	if err := s.revokeAgentToken.Execute(ctx, req.GetDevServerId(), req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}
```

Add the three new usecase fields (`createAgentToken`, `listAgentTokens`,
`revokeAgentToken`) to
`Server`'s constructor (`infragrpc.New(...)`) and wire them from
`cmd/server/main.go` alongside the existing `New*` usecase constructions,
passing `agentTokenStore` (TASK-AWS-03-04), `repo` (`DevServerRepository`),
`credentialBrokerClient` (TASK-AWS-01-02), and `agentClient`
(`LiveSessionCloser`).

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./...
```

Expected: clean build; `buf breaking` reports only additions.
