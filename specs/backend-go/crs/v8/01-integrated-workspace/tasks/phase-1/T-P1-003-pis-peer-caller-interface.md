---
task_id: T-P1-003
phase: 1
cr_id: CR-ORCA-005
solution_ref: solutions/CR-ORCA-005-solution.md
service: orca-agent
target_file: orca-agent/internal/pis/client.go
priority: P1
status: done
depends_on: T-P1-001
estimated_effort: 30 phút
---

# T-P1-003 — Verify/Fix PeerCaller interface trong pis/client.go

## Context

`internal/pis/client.go` định nghĩa `PeerCaller` interface. Sau khi T-P1-001 implement `ws.Client.Call()`, cần đảm bảo signature của interface **khớp chính xác** với method đã implement để compile được.

## Target file

```
orca-agent/internal/pis/client.go
```

## Instructions

1. **Đọc file hiện tại** `orca-agent/internal/pis/client.go`

2. **Kiểm tra `PeerCaller` interface** — phải có đúng signature:
   ```go
   type PeerCaller interface {
       Call(ctx context.Context, targetAgentID, toolName string, args map[string]any) (json.RawMessage, error)
   }
   ```
   - Nếu chưa có → thêm vào
   - Nếu có nhưng sai signature → sửa cho đúng

3. **Kiểm tra `PISTool` struct và constructor** — phải có:
   ```go
   type PISTool struct {
       productsAgentID string
       peer            PeerCaller
   }

   func NewPISTool(productsAgentID string, peer PeerCaller) *PISTool {
       return &PISTool{productsAgentID: productsAgentID, peer: peer}
   }
   ```

4. **Kiểm tra `Dispatch()` method**:
   ```go
   func (t *PISTool) Dispatch(ctx context.Context, toolName string, params map[string]any) (json.RawMessage, error) {
       return t.peer.Call(ctx, t.productsAgentID, toolName, params)
   }
   ```

5. **Thêm `UpdateCRSStatus()` method** (cần cho CR-ORCA-003 Phase 5):
   ```go
   // UpdateCRSStatus peer-invokes products-agent để update CRS status trong PIKB KG.
   // CR-ORCA-003: gọi từ CompletionHook sau khi task done. Best-effort — caller không block nếu fail.
   func (t *PISTool) UpdateCRSStatus(ctx context.Context, crsID, newStatus string) error {
       _, err := t.peer.Call(ctx, t.productsAgentID, "pis_update_crs_status", map[string]any{
           "crs_id":     crsID,
           "status":     newStatus,
           "updated_by": "orca-agent",
       })
       return err
   }
   ```

6. **Đảm bảo imports**: `"context"`, `"encoding/json"`

## Acceptance criteria

- [ ] `PeerCaller` interface tồn tại với đúng signature
- [ ] `PISTool` struct có `productsAgentID` và `peer PeerCaller`
- [ ] `NewPISTool()` constructor tồn tại
- [ ] `Dispatch()` method tồn tại
- [ ] `UpdateCRSStatus()` method tồn tại
- [ ] `GOWORK=off go build ./internal/pis/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/pis/...
```

## Note

Tác vụ này chủ yếu là **read-verify-fix** — nếu file đã đúng thì không cần thay đổi gì lớn. Chỉ cần đảm bảo compile được sau khi T-P1-001 thêm `Call()` vào `ws.Client`.
