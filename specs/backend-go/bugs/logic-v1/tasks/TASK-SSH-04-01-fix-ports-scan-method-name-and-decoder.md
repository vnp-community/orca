# TASK-SSH-04-01: Fix `ports.scan` → `ports.detect` + decode the real `{ports, platform}` response shape

**From Solution:** SOL-SSH-04
**Priority:** P0 — every relayed scan fails at the transport level until this lands
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go`
**Depends on:** none
**Status:** `[x] DONE — ScanWorkspacePorts relays to ports.detect and decodes {port,host,pid,processName}; TestScanWorkspacePorts_* updated and pass`

---

## Context

`ScanWorkspacePorts.Execute` relays to `"ports.scan"`
(`scan_workspace_ports.go:50`), but the agent's actual registered RPC is
`"ports.detect"` (`agent/src/relay/port-scan-handler.ts:20`) — there is no
`ports.scan` handler anywhere in `agent/src/relay/`. `decodeOpenPorts` also
only extracts a flat `openPorts []int32` field, but the real response shape
is `{ports: DetectedPort[], platform}`, each entry carrying
`port/host/pid/processName` (`port-scan-handler.ts:7-12`). This is exactly
the "two independently-implemented RPC surfaces diverge in method names"
failure mode `infra-fleet-service.md` §10 warns about — fix both before
building anything on top of this relay.

## Changes to make

Replace `scan_workspace_ports.go` in full:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type ScanWorkspacePortsInput struct {
	ConnectionID string
	WorktreeID   string
}

// DetectedPort mirrors agent/src/relay/port-scan-handler.ts's DetectedPort —
// the agent's real ports.detect response shape, not a flat port-number list.
type DetectedPort struct {
	Port        int32
	Host        string
	PID         int32
	ProcessName string
}

type ScanWorkspacePorts struct {
	resolver ConnectionResolver
	agent    DevServerAgentClient
}

func NewScanWorkspacePorts(resolver ConnectionResolver, agent DevServerAgentClient) *ScanWorkspacePorts {
	return &ScanWorkspacePorts{resolver: resolver, agent: agent}
}

func (uc *ScanWorkspacePorts) Execute(ctx context.Context, in ScanWorkspacePortsInput) ([]DetectedPort, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	if in.ConnectionID != "" {
		connected, devServer, _, resolveErr := uc.resolver.ResolveConnection(ctx, tenantID, in.ConnectionID)
		if resolveErr != nil {
			return nil, apperrors.New(apperrors.KindInternal, "INFRA_RESOLVE_FAILED", "failed to resolve connection", resolveErr)
		}
		if connected {
			result, execErr := uc.agent.Exec(ctx, devServer, "ports.detect", map[string]any{"worktreeId": in.WorktreeID})
			if execErr != nil {
				return nil, apperrors.New(apperrors.KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay workspace port scan to dev server agent", execErr)
			}
			return decodeDetectedPorts(result), nil
		}
	}

	return []DetectedPort{}, nil
}

// decodeDetectedPorts extracts the "ports" field ports.detect's real
// response carries — see agent/src/relay/port-scan-handler.ts's
// DetectedPort type. Defensive against absent/malformed fields.
func decodeDetectedPorts(result map[string]any) []DetectedPort {
	raw, ok := result["ports"].([]any)
	if !ok {
		return []DetectedPort{}
	}
	out := make([]DetectedPort, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		port, ok := toInt32(m["port"])
		if !ok {
			continue
		}
		host, _ := m["host"].(string)
		pid, _ := toInt32(m["pid"])
		processName, _ := m["processName"].(string)
		out = append(out, DetectedPort{Port: port, Host: host, PID: pid, ProcessName: processName})
	}
	return out
}

func toInt32(v any) (int32, bool) {
	switch n := v.(type) {
	case float64:
		return int32(n), true
	case int32:
		return n, true
	case int:
		return int32(n), true
	default:
		return 0, false
	}
}
```

The gRPC server's `ScanWorkspacePorts` handler and
`ScanWorkspacePortsResponse` proto message need updating to carry the richer
shape — that's TASK-SSH-04-07 (bundled with the new port-forward RPCs to
avoid two separate proto-regeneration passes). This task's own return-type
change (`[]int32` -> `[]DetectedPort`) is a breaking change to the usecase
layer only; the gRPC/proto boundary keeps compiling against the old
`[]int32`-shaped `ScanWorkspacePortsResponse` until TASK-SSH-04-07 lands (the
gRPC server's existing mapping will need a one-line adjustment to keep
building in between — acceptable transient breakage within this task
sequence, not a public API change).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/internal/usecase/... 2>&1 | head -30
go test ./services/infra-fleet-service/internal/usecase/... -run TestScanWorkspacePorts -v
```

Expected: relays to `"ports.detect"`, not `"ports.scan"` (regression guard);
decodes `{port, host, pid, processName}` entries, not a flat int array.
`internal/adapter/grpc` will fail to build until TASK-SSH-04-07 — expected
at this point in the sequence.
