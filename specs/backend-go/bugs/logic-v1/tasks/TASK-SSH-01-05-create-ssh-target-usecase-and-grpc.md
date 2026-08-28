# TASK-SSH-01-05: Thread port/known-hosts/jump-host through `CreateSshTarget` usecase + gRPC + `ListSshTargets`/`GetSshState` mapping

**From Solution:** SOL-SSH-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go`
**Depends on:** TASK-SSH-01-01, TASK-SSH-01-02, TASK-SSH-01-04
**Status:** `[x] DONE — CreateSshTargetInput + gRPC CreateSshTarget/ListSshTargets map the 3 new fields, existing tests pass`

---

## Context

`domain.NewSshTarget`'s signature changed (TASK-SSH-01-02); `CreateSshTarget`
and the gRPC server's request/response mapping (which reads the proto fields
added in TASK-SSH-01-01) need updating in lockstep, or the service won't
build.

## Changes to make

`backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go`:

```go
type CreateSshTargetInput struct {
	Host                  string
	Port                  int
	UserName              string
	VaultSSHRole          string
	KnownHostsFingerprint string
	JumpHostTargetID      string
}

func (uc *CreateSshTarget) Execute(ctx context.Context, in CreateSshTargetInput) (domain.SshTarget, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.SshTarget{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	target, err := domain.NewSshTarget(uuid.NewString(), tenantID, in.Host, in.Port, in.UserName, in.VaultSSHRole, in.KnownHostsFingerprint, in.JumpHostTargetID)
	if err != nil {
		return domain.SshTarget{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_SSH_TARGET", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, target)
	if err != nil {
		return domain.SshTarget{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_SSH_TARGET_FAILED", "failed to create ssh target", err)
	}
	return saved, nil
}
```

`backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`,
`CreateSshTarget` (currently line 212):

```go
func (s *Server) CreateSshTarget(ctx context.Context, req *infrafleetv1.CreateSshTargetRequest) (*infrafleetv1.CreateSshTargetResponse, error) {
	target, err := s.createSshTarget.Execute(ctx, usecase.CreateSshTargetInput{
		Host:                  req.GetHost(),
		Port:                  int(req.GetPort()),
		UserName:              req.GetUser(),
		VaultSSHRole:          req.GetVaultSshRole(),
		KnownHostsFingerprint: req.GetKnownHostsFingerprint(),
		JumpHostTargetID:      req.GetJumpHostTargetId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.CreateSshTargetResponse{SshTargetId: target.ID}, nil
}
```

Also update the `SshTarget` proto-mapping literal in `server.go` (around
line 254, inside `ListSshTargets`'s handler — search for
`&infrafleetv1.SshTarget{`) to populate the 3 new fields from `domain.SshTarget`:

```go
out = append(out, &infrafleetv1.SshTarget{
	Id:                    t.ID,
	TenantId:              t.TenantID,
	Host:                  t.Host,
	Port:                  int32(t.Port),
	User:                  t.UserName,
	VaultSshRole:          t.VaultSSHRole,
	KnownHostsFingerprint: t.KnownHostsFingerprint,
	JumpHostTargetId:      t.JumpHostTargetID,
})
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestCreateSshTarget -v
go test ./services/infra-fleet-service/internal/adapter/grpc/... -v
```

Expected: existing `TestCreateSshTarget_*` tests updated for the new input
fields still pass (omitting the 3 new fields defaults exactly as before —
no breaking change for existing callers); gRPC server tests pass.
