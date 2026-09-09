# CR-WF-002 — Server Target Resolution (`project:`/`server:`/`fleet:tag:`) + AI Provider Resolution

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-002 |
| **Tên** | Resolver layer cho step target (multi-server) và AI provider (multi-provider) — 2 trụ cột "multi" trong tên feature F36 hiện chưa tồn tại |
| **Loại** | Feature |
| **Priority** | P0 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-WF-001](./CR-WF-001-fix-agent-step-executor-relay-method.md) (fix relay method trước khi thêm resolver lên trên) |
| **Tham chiếu** | `specs/backend-go/tdd/services/workflow-service.md` §2/§7 (resolver đã được sketch nhưng chưa build), `specs/backend-go/bugs/logic-v1/BUG-WF-02-workflow-execution-partial.md`, `specs/backend-go/bugs/task-v1/BUG-TASKV1-006-workflow-target-resolution-and-step-types-gap.md`, legacy `specs/backend/bugs/hld-v1/BUG-BE-HLD-008-workflow-provider-selection-not-implemented.md` (xác nhận gap kế thừa nguyên vẹn sang backend-go) |
| **Tác động** | `backend-go/services/workflow-service/internal/domain/step.go`, usecase mới (`internal/usecase/resolve_target.go`), gRPC client mới tới `infra-fleet-service`/`ai-provider-service` |

---

## 1. Vấn đề

`AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig`
(`step.go:64-87`) đều chỉ có `ConnectionID string` trần — caller phải **tự
biết trước** connection ID, không có cú pháp cascade nào. `step.go:58-63`'s
comment tự nhận đây "là field mới thêm ở lần sửa này" chính vì chưa từng có
khái niệm resolver. Grep xác nhận `fleet:tag`/`ServerResolver`/
`resolveServer` = 0 kết quả toàn `workflow-service`/`orchestration-service`/
`infra-fleet-service`.

Tương tự, `AgentStepConfig` không có field `provider`/`model`/`accountId` —
không nơi nào resolve AI provider cho step `agent`. TDD's §7 đã sketch thiết
kế ("Priority mirrors TS's `resolveAgentProvider`: explicit
`step.config.provider.accountId` pin (validated active) beats
`ai-provider-service`'s priority-chain resolution (user &gt; project &gt; server)")
nhưng **0% được code hoá**. Legacy TS's tương đương gap
(`BUG-BE-HLD-008`) xác nhận đây không phải regression mới — backend-go kế
thừa nguyên trạng gap từ đầu, chưa bao giờ được lấp.

## 2. Giải pháp đề xuất

### 2.1 Cú pháp target — 3 dạng, theo spec F36 §Multi-server execution

```go
// internal/domain/target_spec.go (mới)
type TargetKind int
const (
    TargetKindProject TargetKind = iota // "project:<id>"
    TargetKindServer                    // "server:<id>"
    TargetKindFleetTag                  // "fleet:tag:<tag>"
)

func ParseTargetSpec(raw string) (TargetSpec, error) { /* parse "project:"/"server:"/"fleet:tag:" prefix */ }
```

### 2.2 `ServerResolver` — usecase mới, gọi ra ngoài `workflow-service`

```go
// internal/usecase/resolve_target.go
type ServerResolver struct {
    projectClient    projectv1.ProjectServiceClient    // "project:<id>" → devServerId đã bind (F34)
    infraFleet       infrafleetv1.InfraFleetServiceClient // "fleet:tag:<tag>" → load-balanced pick
}

func (r *ServerResolver) Resolve(ctx context.Context, spec TargetSpec) (connectionID string, err error) {
    switch spec.Kind {
    case TargetKindProject:
        proj, err := r.projectClient.GetProject(ctx, &projectv1.GetProjectRequest{Id: spec.ID})
        if err != nil { return "", err }
        return proj.GetDevServerId(), nil
    case TargetKindServer:
        return spec.ID, nil // validate tồn tại + reachable trước khi trả về
    case TargetKindFleetTag:
        return r.infraFleet.PickByTag(ctx, &infrafleetv1.PickByTagRequest{Tag: spec.Tag}) // load-balance: round-robin hoặc least-busy
    }
    return "", ErrUnknownTargetKind
}
```

`PickByTag` là RPC mới cần thêm ở `infra-fleet-service` nếu chưa có tương
đương — xác nhận trước khi code (có thể đã có cơ chế pool-selection dùng cho
mục đích khác, tái sử dụng nếu vậy thay vì viết mới).

### 2.3 `ProviderResolver` — priority chain theo TDD

```go
// internal/usecase/resolve_provider.go
func (r *ProviderResolver) Resolve(ctx context.Context, step AgentStepConfig, projectID string) (ResolvedProvider, error) {
    if step.Provider.AccountID != "" { // pin tường minh trong step config
        return r.validateActive(ctx, step.Provider.AccountID)
    }
    return r.aiProvider.ResolveForContext(ctx, &aiproviderv1.ResolveForContextRequest{
        UserID: step.TriggeredBy, ProjectID: projectID, // user > project > server, theo priority-chain đã có ở ai-provider-service (F35)
    })
}
```

Tái sử dụng `ai-provider-service`'s priority-chain đã có cho F35 — không tự
viết lại logic ưu tiên user/project/server ở `workflow-service`.

### 2.4 Wire vào step executors

`agent_step_executor.go`/`shell_step_executor.go`/`notification_step_executor.go`
gọi `ServerResolver.Resolve` trước khi lấy `ConnectionID` để relay, thay vì
đọc `cfg.ConnectionID` trực tiếp. `agent_step_executor.go` bổ sung gọi
`ProviderResolver.Resolve` để điền `Model`/`AccountID` vào `agentExecParams`
(field CR-WF-001 vừa thêm).

## 3. Rủi ro / Không thuộc phạm vi

- `PickByTag`'s thuật toán load-balance (round-robin vs least-busy vs
  random) là quyết định cần review riêng — CR này không chốt thuật toán cụ
  thể, chỉ định nghĩa interface.
- Không thuộc phạm vi: UI cho người dùng nhập cú pháp `project:`/`server:`/
  `fleet:tag:` — `StepEditor.tsx`'s `serverSpec: string` field đã tồn tại ở
  frontend (free-form, chưa validate) — CR-WF-006 chịu trách nhiệm validate
  UI, CR này chỉ đảm bảo backend hiểu đúng cú pháp khi nhận được.
- Không giải quyết provider-mixing UI (chọn provider khác nhau cho từng step
  trong 1 workflow) — chỉ đảm bảo backend resolve đúng khi step config có
  pin, phần UI thuộc CR-WF-006.

## Acceptance Criteria

- [ ] `ParseTargetSpec` parse đúng cả 3 dạng, trả lỗi rõ ràng cho cú pháp
      không hợp lệ.
- [ ] `ServerResolver.Resolve("project:&lt;id&gt;")` trả đúng `devServerId` đã
      bind qua F34; test với project chưa bind trả lỗi rõ ràng (không panic).
- [ ] `ServerResolver.Resolve("fleet:tag:&lt;tag&gt;")` trả 1 trong các server có
      tag đó, test với tag không tồn tại trả lỗi rõ ràng.
- [ ] `ProviderResolver.Resolve` ưu tiên đúng thứ tự: pin tường minh (nếu có
      và active) &gt; priority-chain user&gt;project&gt;server.
- [ ] `agent_step_executor.go` gửi đúng `Model`/`AccountID` đã resolve, không
      còn để trống mặc định khi step config có pin.
- [ ] Test: 1 workflow với 2 step nhắm 2 dev server khác nhau (`project:A` +
      `server:B`) dispatch đúng tới đúng server tương ứng.
