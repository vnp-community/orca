package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// EmulatorDeviceResult mirrors ListEmulatorDevicesResponse's per-device
// fields — a plain result struct (not a domain entity), matching
// SpawnPtyResult/AgentStatusResult's convention in ports.go: this service
// reflects whatever the agent reports, it doesn't own emulator state.
type EmulatorDeviceResult struct {
	ID       string
	Name     string
	Platform string
	State    string
}

// EmulatorSessionResult mirrors the EmulatorSession proto message's fields.
type EmulatorSessionResult struct {
	SessionID    string
	DeviceID     string
	ConnectionID string
	Platform     string
}

// EmulatorRelay implements every emulator.* usecase (TASK-048): mobile
// emulator/simulator control (ADB/xcrun simctl device driving), relayed to
// the Dev Server Agent's device.* JSON-RPC surface. There is deliberately
// no backend-host fallback branch — driving emulators on the shared
// backend-go host is explicitly excluded by
// 02-microservices-decomposition.md's "What's deliberately not a separate
// service" section (see wscompat's registerEmulatorChannels doc comment for
// the wire-level mirror of this rule): an empty/unresolved connectionId is
// a hard FailedPrecondition here, unlike ScanWorkspacePorts's "no
// connection = execute locally" convention.
//
// agent/ has no device.* method today (confirmed absent this pass by
// grepping agent/src/relay/agent-rpc-dispatch.ts and the whole agent/ tree
// for adb/simctl/xcrun/device.*) — every call below reaches a real dev
// server agent and gets back a real JSON-RPC "method not found", which
// callAgent translates into a typed, permanent
// apperrors.KindFailedPrecondition result. The moment agent/ adds a
// device.* handler, these calls start working with zero further
// backend-go changes.
type EmulatorRelay struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewEmulatorRelay(resolver ConnectionResolver, agent DevServerAgentClient) *EmulatorRelay {
	return &EmulatorRelay{resolver: resolver, agent: agent}
}

// resolveDevServer centralizes the "connectionId required, no local
// fallback" rule every emulator.* method other than GetAvailability shares.
func (uc *EmulatorRelay) resolveDevServer(ctx context.Context, connectionID string) (domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if connectionID == "" {
		return domain.DevServer{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_EMULATOR_NO_CONNECTION", "emulator control requires an active dev server connection — there is no local/backend-host fallback", nil)
	}
	connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, connectionID)
	if resolveErr != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
	}
	if !connected {
		return domain.DevServer{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_EMULATOR_NO_CONNECTION", "no dev server is bound to this connection", nil)
	}
	return devServer, nil
}

// callAgent relays method to devServer and translates a real agent-side
// "method not found" (domain.ErrAgentMethodNotFound — see its doc comment)
// into a typed, permanent apperrors result, mirroring git-gateway-service's
// domain.ErrForceDeleteBranchUnsupported translation in usecase.ForceDeleteBranch.
func (uc *EmulatorRelay) callAgent(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	result, err := uc.agent.Exec(ctx, devServer, method, params)
	if err != nil {
		if errors.Is(err, domain.ErrAgentMethodNotFound) {
			return nil, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_EMULATOR_UNSUPPORTED",
				"this dev server's agent build does not support emulator control ("+method+") — see specs/backend-go/bugs/missing-v1/tasks/TASK-048", err)
		}
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay "+method+" to dev server agent", err)
	}
	return result, nil
}

// ListDevices calls device.list.
func (uc *EmulatorRelay) ListDevices(ctx context.Context, connectionID string) ([]EmulatorDeviceResult, error) {
	devServer, err := uc.resolveDevServer(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	result, err := uc.callAgent(ctx, devServer, "device.list", nil)
	if err != nil {
		return nil, err
	}
	return decodeEmulatorDevices(result), nil
}

// GetAvailability calls device.availability. Unlike every other method
// here, an empty/unresolved connectionId is answered honestly as
// "unavailable" rather than an error — mirrors TASK-070's host.* stub
// posture: "is emulator control even possible right now" must always
// answer something, including from a settings pane with no active
// connection yet.
func (uc *EmulatorRelay) GetAvailability(ctx context.Context, connectionID string) (available bool, reason string, err error) {
	if connectionID == "" {
		return false, "no active dev server connection", nil
	}
	tenantID, tErr := tenant.RequireTenantID(ctx)
	if tErr != nil {
		return false, "", apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", tErr)
	}
	connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, connectionID)
	if resolveErr != nil {
		return false, "", apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
	}
	if !connected {
		return false, "no dev server is bound to this connection", nil
	}
	result, execErr := uc.agent.Exec(ctx, devServer, "device.availability", nil)
	if execErr != nil {
		if errors.Is(execErr, domain.ErrAgentMethodNotFound) {
			return false, "this dev server's agent build does not support emulator control", nil
		}
		return false, "", apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to check emulator availability", execErr)
	}
	avail, _ := result["available"].(bool)
	reasonStr, _ := result["reason"].(string)
	return avail, reasonStr, nil
}

// AttachSession calls device.attach.
func (uc *EmulatorRelay) AttachSession(ctx context.Context, connectionID, deviceID string) (EmulatorSessionResult, error) {
	devServer, err := uc.resolveDevServer(ctx, connectionID)
	if err != nil {
		return EmulatorSessionResult{}, err
	}
	result, err := uc.callAgent(ctx, devServer, "device.attach", map[string]any{"deviceId": deviceID})
	if err != nil {
		return EmulatorSessionResult{}, err
	}
	return decodeEmulatorSession(result, connectionID), nil
}

// SendCommand implements the 5 fire-and-forget emulator control operations
// (tap/gesture/button/rotate/shutdown) — identical shape (resolve, relay,
// translate unsupported), differing only in the agent method name and
// params, per TASK-048's wscompat wiring sketch note that they "follow the
// same decode -> ... -> translate shape".
func (uc *EmulatorRelay) SendCommand(ctx context.Context, connectionID, method string, params map[string]any) error {
	devServer, err := uc.resolveDevServer(ctx, connectionID)
	if err != nil {
		return err
	}
	_, err = uc.callAgent(ctx, devServer, method, params)
	return err
}

func decodeEmulatorDevices(result map[string]any) []EmulatorDeviceResult {
	raw, ok := result["devices"].([]any)
	if !ok {
		return []EmulatorDeviceResult{}
	}
	devices := make([]EmulatorDeviceResult, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		devices = append(devices, EmulatorDeviceResult{
			ID:       emulatorStringField(m, "id"),
			Name:     emulatorStringField(m, "name"),
			Platform: emulatorStringField(m, "platform"),
			State:    emulatorStringField(m, "state"),
		})
	}
	return devices
}

func decodeEmulatorSession(result map[string]any, connectionID string) EmulatorSessionResult {
	return EmulatorSessionResult{
		SessionID:    emulatorStringField(result, "sessionId"),
		DeviceID:     emulatorStringField(result, "deviceId"),
		ConnectionID: connectionID,
		Platform:     emulatorStringField(result, "platform"),
	}
}

func emulatorStringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
