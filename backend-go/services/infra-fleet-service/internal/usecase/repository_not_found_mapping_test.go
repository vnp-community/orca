package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// Hồi quy TASK-E2E-017: repo trả sentinel miền (bọc %w) -> KindNotFound với mã
// hiện có; mọi lỗi hạ tầng khác -> KindInternal, không bị nuốt thành NotFound.

var errNFInfraDown = errors.New("connection refused")

func nfWrap(sentinel error) error { return fmt.Errorf("postgres: lookup: %w", sentinel) }

func nfCtx() context.Context {
	return tenant.WithRole(tenant.WithTenantID(context.Background(), "tenant-1"), "admin")
}

func nfAssert(t *testing.T, err error, kind apperrors.Kind, code string) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want *apperrors.AppError, got %T (%v)", err, err)
	}
	if ae.Kind != kind || ae.Code != code {
		t.Errorf("got kind=%v code=%q, want kind=%v code=%q", ae.Kind, ae.Code, kind, code)
	}
}

// nfCase runs run() once with the sentinel (wrapped) and once with an infra error.
func nfCase(t *testing.T, notFoundKind apperrors.Kind, notFoundCode, internalCode string, run func(repoErr error) error) {
	t.Helper()
	t.Run("not found", func(t *testing.T) { nfAssert(t, run(nil), notFoundKind, notFoundCode) })
	t.Run("infra error", func(t *testing.T) { nfAssert(t, run(errNFInfraDown), apperrors.KindInternal, internalCode) })
}

type nfDevServers struct {
	DevServerRepository
	getErr, approvalErr, assignErr error
}

func (f *nfDevServers) Get(context.Context, string, string) (domain.DevServer, error) {
	return domain.DevServer{}, f.getErr
}
func (f *nfDevServers) UpdateApprovalStatus(context.Context, string, string, domain.DevServerStatus) (domain.DevServer, error) {
	return domain.DevServer{}, f.approvalErr
}
func (f *nfDevServers) AssignGroup(context.Context, string, string, string) (domain.DevServer, error) {
	return domain.DevServer{}, f.assignErr
}

func nfErr(infra error, sentinel error) error {
	if infra != nil {
		return infra
	}
	return nfWrap(sentinel)
}

func TestDevServerLookup_NotFoundVsInfra(t *testing.T) {
	ctx := nfCtx()
	cases := map[string]func(repo DevServerRepository) error{
		"GetAgentCapabilities": func(r DevServerRepository) error {
			_, err := NewGetAgentCapabilities(r, nil).Execute(ctx, "ds-1")
			return err
		},
		"IsDevServerConnected": func(r DevServerRepository) error {
			_, err := NewIsDevServerConnected(r, nil).Execute(ctx, "ds-1")
			return err
		},
		"RelayByDevServer": func(r DevServerRepository) error {
			_, err := NewRelayByDevServer(r, nil).Execute(ctx, RelayByDevServerInput{DevServerID: "ds-1", Method: "m"})
			return err
		},
		"StreamCodeIntelEvents": func(r DevServerRepository) error {
			_, _, err := NewStreamCodeIntelEvents(r, nil, nil).Execute(ctx, "ds-1")
			return err
		},
		"StreamFileChanges": func(r DevServerRepository) error {
			_, err := NewStreamFileChanges(nil, r, nil).resolveDevServer(ctx, "tenant-1", StreamFileChangesInput{DevServerID: "ds-1"})
			return err
		},
		"CheckDevServerPreflight": func(r DevServerRepository) error {
			_, err := NewCheckDevServerPreflight(r, nil).Execute(ctx, "tenant-1", "ds-1", 3000)
			return err
		},
		"DetectDevServerAgents": func(r DevServerRepository) error {
			_, err := NewDetectDevServerAgents(r, nil).Execute(ctx, "tenant-1", "ds-1")
			return err
		},
		"GetDevServerCapabilities": func(r DevServerRepository) error {
			uc := &GetDevServerCapabilities{devServers: r}
			_, err := uc.Execute(ctx, GetCapabilitiesInput{DevServerID: "ds-1"})
			return err
		},
		"RefreshDevServerCapabilities": func(r DevServerRepository) error {
			uc := &RefreshDevServerCapabilities{devServers: r}
			_, err := uc.refresh(ctx, "tenant-1", "ds-1", false, time.Now())
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			nfCase(t, apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "INFRA_DEV_SERVER_LOOKUP_FAILED", func(infra error) error {
				return run(&nfDevServers{getErr: nfErr(infra, domain.ErrDevServerNotFound)})
			})
		})
	}
}

func TestApplyTerraformPlan_ControlHostLookup(t *testing.T) {
	nfCase(t, apperrors.KindInvalidArgument, "INFRA_UNKNOWN_CONTROL_HOST", "INFRA_DEV_SERVER_LOOKUP_FAILED", func(infra error) error {
		repo := &nfDevServers{getErr: nfErr(infra, domain.ErrDevServerNotFound)}
		_, err := NewApplyTerraformPlan(repo, nil).Execute(nfCtx(), ApplyTerraformPlanInput{ControlDevServerID: "ds-1"})
		return err
	})
}

func TestApproveRejectAssign_DevServerNotFoundVsInfra(t *testing.T) {
	ctx := nfCtx()
	t.Run("approve", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "INFRA_APPROVE_DEV_SERVER_FAILED", func(infra error) error {
			_, err := NewApproveDevServer(&nfDevServers{approvalErr: nfErr(infra, domain.ErrDevServerNotFound)}).Execute(ctx, "ds-1")
			return err
		})
	})
	t.Run("reject", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "INFRA_REJECT_DEV_SERVER_FAILED", func(infra error) error {
			_, err := NewRejectDevServer(&nfDevServers{approvalErr: nfErr(infra, domain.ErrDevServerNotFound)}).Execute(ctx, RejectDevServerInput{DevServerID: "ds-1"})
			return err
		})
	})
	t.Run("assign group", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_DEV_SERVER_NOT_FOUND", "INFRA_ASSIGN_DEV_SERVER_GROUP_FAILED", func(infra error) error {
			_, err := NewAssignDevServerGroup(&nfDevServers{assignErr: nfErr(infra, domain.ErrDevServerNotFound)}).Execute(ctx, AssignDevServerGroupInput{DevServerID: "ds-1", GroupID: "g-1"})
			return err
		})
	})
}

type nfSshTargets struct {
	SshTargetRepository
	getErr error
}

func (f *nfSshTargets) Get(context.Context, string, string) (domain.SshTarget, error) {
	return domain.SshTarget{}, f.getErr
}

func TestEstablishConnection_SshTargetNotFoundVsInfra(t *testing.T) {
	nfCase(t, apperrors.KindNotFound, "INFRA_SSH_TARGET_NOT_FOUND", "INFRA_SSH_TARGET_LOOKUP_FAILED", func(infra error) error {
		uc := NewEstablishConnection(&nfSshTargets{getErr: nfErr(infra, domain.ErrSshTargetNotFound)}, nil, nil, nil, nil)
		_, err := uc.Execute(nfCtx(), EstablishConnectionInput{SshTargetID: "t-1"})
		return err
	})
}

type nfAccessRequests struct {
	DevServerAccessRequestRepository
	getErr, updateErr error
}

func (f *nfAccessRequests) Get(context.Context, string, string) (domain.DevServerAccessRequest, error) {
	if f.getErr != nil {
		return domain.DevServerAccessRequest{}, f.getErr
	}
	return domain.DevServerAccessRequest{Status: domain.AccessRequestStatusPending}, nil
}
func (f *nfAccessRequests) UpdateStatus(context.Context, string, string, domain.AccessRequestStatus) (domain.DevServerAccessRequest, error) {
	return domain.DevServerAccessRequest{}, f.updateErr
}

func TestResolveAccessRequest_NotFoundVsInfra(t *testing.T) {
	in := ResolveAccessRequestInput{RequestID: "r-1", Approve: true}
	t.Run("get", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_ACCESS_REQUEST_NOT_FOUND", "INFRA_GET_ACCESS_REQUEST_FAILED", func(infra error) error {
			_, err := NewResolveAccessRequest(&nfAccessRequests{getErr: nfErr(infra, domain.ErrDevServerAccessRequestNotFound)}, nil).Execute(nfCtx(), in)
			return err
		})
	})
	t.Run("update status", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_ACCESS_REQUEST_NOT_FOUND", "INFRA_RESOLVE_ACCESS_REQUEST_FAILED", func(infra error) error {
			_, err := NewResolveAccessRequest(&nfAccessRequests{updateErr: nfErr(infra, domain.ErrDevServerAccessRequestNotFound)}, nil).Execute(nfCtx(), in)
			return err
		})
	})
}

type nfTerminalSessions struct {
	TerminalSessionRepository
	touchErr, closeErr error
}

func (f *nfTerminalSessions) Get(_ context.Context, tenantID, ptyID string) (bool, domain.TerminalSession, error) {
	return true, domain.TerminalSession{PtyID: ptyID, TenantID: tenantID, ConnectionID: "conn-1"}, nil
}
func (f *nfTerminalSessions) Touch(context.Context, string, string, time.Time) error {
	return f.touchErr
}
func (f *nfTerminalSessions) Close(context.Context, string, string, time.Time) error {
	return f.closeErr
}

type nfResolver struct{ ConnectionResolver }

func (nfResolver) ResolveConnection(context.Context, string, string) (bool, domain.DevServer, domain.Connection, error) {
	return true, domain.DevServer{ID: "ds-1"}, domain.Connection{ID: "conn-1"}, nil
}

type nfAgent struct{ DevServerAgentClient }

func (nfAgent) ResizePty(context.Context, domain.DevServer, string, int32, int32) error { return nil }
func (nfAgent) KillPty(context.Context, domain.DevServer, string, bool) error           { return nil }

func TestTerminalSessionMutations_NotFoundVsInfra(t *testing.T) {
	t.Run("focus touch", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_TERMINAL_NOT_FOUND", "INFRA_TOUCH_TERMINAL_SESSION_FAILED", func(infra error) error {
			return NewFocusTerminalSession(&nfTerminalSessions{touchErr: nfErr(infra, domain.ErrTerminalSessionNotFound)}).Execute(nfCtx(), "pty-1")
		})
	})
	t.Run("resize touch", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_TERMINAL_NOT_FOUND", "INFRA_TOUCH_TERMINAL_SESSION_FAILED", func(infra error) error {
			sessions := &nfTerminalSessions{touchErr: nfErr(infra, domain.ErrTerminalSessionNotFound)}
			return NewResizeTerminalSession(sessions, nfResolver{}, nil, nfAgent{}).Execute(nfCtx(), ResizeTerminalSessionInput{PtyID: "pty-1", Cols: 80, Rows: 24})
		})
	})
	t.Run("kill close", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_TERMINAL_NOT_FOUND", "INFRA_CLOSE_TERMINAL_SESSION_FAILED", func(infra error) error {
			sessions := &nfTerminalSessions{closeErr: nfErr(infra, domain.ErrTerminalSessionNotFound)}
			return NewKillTerminalSession(sessions, nfResolver{}, nil, nfAgent{}).Execute(nfCtx(), "pty-1")
		})
	})
}

type nfCloser struct{ err error }

func (f nfCloser) CloseWithEvent(context.Context, string, string, time.Time, domain.OutboxEvent) (bool, error) {
	return false, f.err
}

func TestKillTerminalSession_CloseWithEventNotFoundVsInfra(t *testing.T) {
	nfCase(t, apperrors.KindNotFound, "INFRA_TERMINAL_NOT_FOUND", "INFRA_CLOSE_TERMINAL_SESSION_FAILED", func(infra error) error {
		uc := NewKillTerminalSession(&nfTerminalSessions{}, nfResolver{}, nil, nfAgent{}).
			WithClosedEvents(nfCloser{err: nfErr(infra, domain.ErrTerminalSessionNotFound)})
		return uc.Execute(nfCtx(), "pty-1")
	})
}

// A non-NotFound devServers.Get error in the connectionId->devServerId fallback
// must surface as Internal, not be treated as "not a dev server".
func TestResolveTerminalSession_FallbackInfraErrorIsInternal(t *testing.T) {
	notBound := nfNotConnectedResolver{}
	_, _, err := resolveTerminalSession(nfCtx(), "tenant-1", "pty-1", &nfTerminalSessions{}, notBound, &nfDevServers{getErr: errNFInfraDown})
	nfAssert(t, err, apperrors.KindInternal, "INFRA_DEV_SERVER_LOOKUP_FAILED")
	_, _, err = resolveTerminalSession(nfCtx(), "tenant-1", "pty-1", &nfTerminalSessions{}, notBound, &nfDevServers{getErr: nfWrap(domain.ErrDevServerNotFound)})
	nfAssert(t, err, apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND")
}

type nfNotConnectedResolver struct{ ConnectionResolver }

func (nfNotConnectedResolver) ResolveConnection(context.Context, string, string) (bool, domain.DevServer, domain.Connection, error) {
	return false, domain.DevServer{}, domain.Connection{}, nil
}

func TestSpawnTerminalSession_DevServerFallbackInfraErrorIsInternal(t *testing.T) {
	uc := NewSpawnTerminalSession(nfNotConnectedResolver{}, &nfDevServers{getErr: errNFInfraDown}, nil, nil, nil, false)
	_, err := uc.Execute(nfCtx(), SpawnTerminalSessionInput{ConnectionID: "ds-1"})
	nfAssert(t, err, apperrors.KindInternal, "INFRA_DEV_SERVER_LOOKUP_FAILED")
}

type nfConnections struct {
	ConnectionRepository
	updateErr error
}

func (f *nfConnections) UpdateStatus(context.Context, string, domain.Connection) error {
	return f.updateErr
}

func TestTeardownConnection_UpdateStatusNotFoundVsInfra(t *testing.T) {
	nfCase(t, apperrors.KindNotFound, "INFRA_CONNECTION_NOT_FOUND", "INFRA_UPDATE_CONNECTION_STATUS_FAILED", func(infra error) error {
		conns := &nfConnections{updateErr: nfErr(infra, domain.ErrConnectionNotFound)}
		return NewTeardownConnection(nfResolver{}, conns, nil, nil).Execute(nfCtx(), "conn-1")
	})
}

type nfBrowserProfiles struct {
	BrowserProfileRepository
	err error
}

func (f *nfBrowserProfiles) Delete(context.Context, string, string) error { return f.err }

type nfAgentTokens struct {
	AgentTokenRepository
	err error
}

func (f *nfAgentTokens) Revoke(context.Context, string, string) (domain.AgentToken, error) {
	return domain.AgentToken{}, f.err
}

type nfGrants struct {
	DevServerGroupGrantRepository
	err error
}

func (f *nfGrants) Delete(context.Context, string, string) error { return f.err }

func TestDeleteLikeUsecases_NotFoundVsInfra(t *testing.T) {
	t.Run("delete browser profile", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_BROWSER_PROFILE_NOT_FOUND", "INFRA_DELETE_BROWSER_PROFILE_FAILED", func(infra error) error {
			return NewDeleteBrowserProfile(&nfBrowserProfiles{err: nfErr(infra, domain.ErrBrowserProfileNotFound)}).Execute(nfCtx(), "bp-1")
		})
	})
	t.Run("revoke agent token", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_AGENT_TOKEN_NOT_FOUND", "INFRA_REVOKE_AGENT_TOKEN_FAILED", func(infra error) error {
			return NewRevokeAgentToken(&nfAgentTokens{err: nfErr(infra, domain.ErrAgentTokenNotFound)}, nil).Execute(nfCtx(), "ds-1", "tok-1")
		})
	})
	t.Run("revoke group access", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, "INFRA_GRANT_NOT_FOUND", "INFRA_REVOKE_GRANT_FAILED", func(infra error) error {
			return NewRevokeDevServerGroupAccess(&nfGrants{err: nfErr(infra, domain.ErrDevServerGroupGrantNotFound)}).Execute(nfCtx(), "grant-1")
		})
	})
}

type nfRuntimes struct {
	EphemeralVmRuntimeRepository
	err error
}

func (f *nfRuntimes) Get(context.Context, string, string) (domain.EphemeralVmRuntime, error) {
	return domain.EphemeralVmRuntime{}, f.err
}
func (f *nfRuntimes) GetByWorkspaceID(context.Context, string, string) (domain.EphemeralVmRuntime, error) {
	return domain.EphemeralVmRuntime{}, f.err
}
func (f *nfRuntimes) UpdateStatus(context.Context, string, string, string, string, string) (domain.EphemeralVmRuntime, error) {
	return domain.EphemeralVmRuntime{}, f.err
}

func TestEphemeralVmRelay_RuntimeNotFoundVsInfra(t *testing.T) {
	const code, internal = "INFRA_EPHEMERAL_VM_RUNTIME_NOT_FOUND", "INFRA_EPHEMERAL_VM_RUNTIME_STORE_FAILED"
	newRelay := func(infra error) *EphemeralVmRelay {
		return NewEphemeralVmRelay(nfResolver{}, nil, &nfRuntimes{err: nfErr(infra, domain.ErrEphemeralVmRuntimeNotFound)})
	}
	t.Run("attach", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := newRelay(infra).AttachWorkspace(nfCtx(), "rt-1", "ws-1")
			return err
		})
	})
	t.Run("suspend", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := newRelay(infra).SuspendWorkspace(nfCtx(), "conn-1", "ws-1", "cmd")
			return err
		})
	})
	t.Run("resume", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := newRelay(infra).ResumeWorkspace(nfCtx(), "conn-1", "ws-1", "cmd")
			return err
		})
	})
	t.Run("cleanup", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := newRelay(infra).CleanupWorkspace(nfCtx(), "conn-1", "rt-1", "cmd")
			return err
		})
	})
}

type nfFleetDefinitions struct {
	FleetDefinitionRepository
	getErr, updateErr error
}

func (f *nfFleetDefinitions) Get(context.Context, string, string) (domain.FleetDefinition, error) {
	if f.getErr != nil {
		return domain.FleetDefinition{}, f.getErr
	}
	return domain.FleetDefinition{Version: 1}, nil
}
func (f *nfFleetDefinitions) Update(context.Context, domain.FleetDefinition) (domain.FleetDefinition, error) {
	return domain.FleetDefinition{}, f.updateErr
}

// Repos now wrap the sentinel with context, so a bare == comparison would
// misreport the wrapped not-found as Internal.
func TestFleetDefinition_WrappedSentinelIsNotFound(t *testing.T) {
	const code, internal = "INFRA_FLEET_DEFINITION_NOT_FOUND", "INFRA_GET_FLEET_DEFINITION_FAILED"
	get := func(infra error) *nfFleetDefinitions {
		return &nfFleetDefinitions{getErr: nfErr(infra, domain.ErrFleetDefinitionNotFound)}
	}
	t.Run("get", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := NewGetFleetDefinition(get(infra)).Execute(nfCtx(), "fd-1")
			return err
		})
	})
	t.Run("export", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := NewExportFleetDefinitionYaml(get(infra)).Execute(nfCtx(), "fd-1")
			return err
		})
	})
	t.Run("update", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := NewUpdateFleetDefinition(get(infra)).Execute(nfCtx(), UpdateFleetDefinitionInput{ID: "fd-1"})
			return err
		})
	})
	t.Run("deploy", func(t *testing.T) {
		nfCase(t, apperrors.KindNotFound, code, internal, func(infra error) error {
			_, err := NewDeployFleetDefinition(get(infra), nil, nil, nil).Execute(nfCtx(), DeployFleetDefinitionInput{FleetDefinitionID: "fd-1"})
			return err
		})
	})
	t.Run("update conflict wrapped", func(t *testing.T) {
		repo := &nfFleetDefinitions{updateErr: fmt.Errorf("postgres: %w", domain.ErrFleetDefinitionVersionConflict)}
		_, err := NewUpdateFleetDefinition(repo).Execute(nfCtx(), UpdateFleetDefinitionInput{ID: "fd-1", Servers: []domain.FleetSpecServer{{}}})
		nfAssert(t, err, apperrors.KindFailedPrecondition, "INFRA_FLEET_DEFINITION_VERSION_CONFLICT")
	})
}

type nfErrResolver struct{ ConnectionResolver }

func (nfErrResolver) ResolveConnection(context.Context, string, string) (bool, domain.DevServer, domain.Connection, error) {
	return false, domain.DevServer{}, domain.Connection{}, errNFInfraDown
}

// An infra error from ResolveConnection used to be reported as NotFound.
func TestStartAgentSession_ResolveInfraErrorIsInternal(t *testing.T) {
	uc := NewStartAgentSession(nfErrResolver{}, nil, nil, nil, nil)
	_, err := uc.Execute(nfCtx(), StartAgentSessionInput{ConnectionID: "conn-1"})
	nfAssert(t, err, apperrors.KindInternal, "INFRA_RESOLVE_FAILED")
}

type nfAgentSessions struct {
	AgentSessionRepository
	err error
}

func (f nfAgentSessions) LatestForWorktree(context.Context, string, string) (bool, domain.AgentSession, error) {
	return false, domain.AgentSession{}, f.err
}

func TestSwitchAgentAccount_LookupInfraErrorIsInternal(t *testing.T) {
	uc := NewSwitchAgentAccount(nfAgentSessions{err: errNFInfraDown}, nil, nil, nil, nil)
	_, err := uc.Execute(nfCtx(), SwitchAgentAccountInput{WorktreeID: "wt-1"})
	nfAssert(t, err, apperrors.KindInternal, "INFRA_AGENT_SESSION_LOOKUP_FAILED")
	uc = NewSwitchAgentAccount(nfAgentSessions{}, nil, nil, nil, nil)
	_, err = uc.Execute(nfCtx(), SwitchAgentAccountInput{WorktreeID: "wt-1"})
	nfAssert(t, err, apperrors.KindNotFound, "INFRA_AGENT_SESSION_NOT_FOUND")
}
