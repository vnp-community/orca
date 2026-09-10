# CR-TG-002 — AI Decompose: Context Bundle, Structured Proposals, Dependency Edges &amp; Critical Path

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TG-002 |
| **Tên** | Nâng AI decompose từ "chỉ thấy title, chỉ tạo title" lên đúng spec F37 §AI decompose |
| **Loại** | Feature |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-TG-001](./CR-TG-001-orcatask-data-model-widening.md) (cần `description`/`aiContext`/`estimatedHours`/`promptTemplate` tồn tại trên `Task`) |
| **Áp dụng thiết kế** | [SOL-TG-02-ai-task-planning.md](../../../../specs/backend-go/bugs/logic-v1/solutions/SOL-TG-02-ai-task-planning.md) (377 dòng) |
| **Tác động** | `backend-go/services/task-service/internal/usecase/ai_decompose.go`, `ai_apply.go`, `internal/domain/subtask_proposal.go`, `proto/orca/task/v1/task.proto` |

---

## 1. Vấn đề

`AIDecompose.Execute` (`internal/usecase/ai_decompose.go:42-73`) là pipeline
decompose→apply **thật** (resolve AI provider qua gRPC, relay `ai.complete` tới
Dev Server Agent, commit transactional trong `AIApply` có test rollback giữa
chừng) — không phải stub. Nhưng chất lượng input/output quá nghèo so với spec:

- `buildDecomposePrompt` (`ai_decompose.go:79-90`) chỉ interpolate
  **`task.Title`** — không có description, `aiContext`, tech-stack, velocity,
  hay danh sách subtask đã tồn tại (để tránh AI đề xuất trùng).
- `parseSubtaskProposals` (`ai_decompose.go:98-118`) parse 1 danh sách text
  thuần `"&lt;n&gt;. &lt;title&gt;"` — không phải JSON có cấu trúc.
- `domain.SubtaskProposal` (`internal/domain/subtask_proposal.go:9-12`) chỉ có
  `{Title, Description}`, và `Description` **không bao giờ được set**.
- `AIApply` (`ai_apply.go:67`) **luôn luôn** tạo `EdgeKindParentChild` — không
  có `depends_on` edge nào được tạo từ output AI, dù spec (dòng 177-223) yêu
  cầu AI trả về `dependencies` giữa các subtask để hiển thị critical path.
- Không có `CalculateCriticalPath` — `grep -rn "CriticalPath"` = 0 kết quả
  toàn service.
- Không có flow "Generate Agent Prompt" riêng (tách biệt với decompose) — spec
  mô tả 2 flow khác nhau, code chỉ có 1.

## 2. Giải pháp đề xuất (theo SOL-TG-02)

### 2.1 Context bundle 5 nguồn khi build prompt

```go
// ai_decompose.go — buildDecomposePrompt() mở rộng
type DecomposeContext struct {
    Title            string
    Description      string
    AIContext        string   // task.AIContext — free-form ghi chú người dùng
    TechStack        []string // TechStackDetector qua git-gateway-service
    Velocity         *TeamVelocity // optional — throughput trung bình gần đây
    ExistingSubtasks []string // dedupe — không đề xuất lại subtask đã có
}
```

`TechStackDetector` (mới) gọi `git-gateway-service` đọc `package.json`/`go.mod`/
`requirements.txt` ở root worktree để suy ra ngôn ngữ/framework chính — chỉ
best-effort, không chặn decompose nếu detect thất bại.

### 2.2 Structured JSON proposal thay vì text list

```go
// SubtaskProposal mở rộng
type SubtaskProposal struct {
    Title           string
    Description     string
    Type            string   // "feature"|"bug"|"chore"...
    EstimatedHours  float64
    DependsOnIndex  []int    // chỉ số các proposal khác trong CÙNG batch mà proposal này phụ thuộc
    PromptTemplate  string
}
```

Prompt yêu cầu AI trả về JSON đúng shape này (`format: 'json'` đã có sẵn ở
`relay.call('ai.complete', ...)`); parser đổi từ text-list-parser sang
`json.Unmarshal` + validate field bắt buộc, fail rõ ràng nếu AI trả JSON không
hợp lệ (không âm thầm rơi về parse rỗng).

### 2.3 Dependency edges từ `DependsOnIndex`

```go
// ai_apply.go — AIApply.Execute(), trong cùng transaction tạo subtask:
for i, proposal := range proposals {
    subtaskID := createdIDs[i]
    repo.AddEdgeTx(tx, subtaskID, parentID, EdgeKindParentChild)
    for _, depIdx := range proposal.DependsOnIndex {
        repo.AddEdgeTx(tx, subtaskID, createdIDs[depIdx], EdgeKindDependsOn)
    }
}
```

Tái sử dụng `AddEdge`'s atomic cycle-check từ CR-TG-001 — không tự viết lại
logic cycle-detection ở đây.

### 2.4 `CalculateCriticalPath` — pure domain function

```go
// internal/domain/critical_path.go (mới)
// Longest-path theo estimated_hours trên DAG depends_on — thuật toán chuẩn
// (topological sort + DP), không gọi AI, chạy in-process trên dữ liệu đã có.
func CalculateCriticalPath(tasks []Task, edges []Edge) []string // ordered task IDs
```

### 2.5 `GenerateAgentPrompt` — flow tách biệt

```protobuf
// task.proto
rpc GenerateAgentPrompt(GenerateAgentPromptRequest) returns (GenerateAgentPromptResponse);
```

Nhận `taskId`, trả `promptTemplate` được AI soạn riêng cho việc **chạy agent**
(khác mục đích với decompose prompt) — ghi vào `Task.PromptTemplate`, người
dùng có thể sửa tay trước khi Run (xem CR-TG-007 §C4 và CR-TG-005's context
preamble tiêu thụ field này).

### 2.6 Lưu `ai_plan_json` raw response

Ghi nguyên văn response AI (trước parse) vào `Task.AIPlanJSON` để debug khi
parse lỗi hoặc audit lại quyết định AI sau này.

## 3. Rủi ro / Không thuộc phạm vi

- `TechStackDetector` là best-effort — không thiết kế lại git-gateway-service,
  chỉ gọi RPC đọc file đã có sẵn (`git.readFile` tương đương).
- Không tự làm UI hiển thị critical path — đó là CR-TG-007 (tô màu path trên
  Board/DAG view dựa vào field `criticalPath` RPC này trả về).
- `Velocity` (throughput trung bình) là optional field — nếu chưa có nguồn dữ
  liệu lịch sử đủ tin cậy, có thể bỏ qua ở lần triển khai đầu, không chặn CR.
- Không đổi cơ chế relay `ai.complete` hiện có (đã đúng, xem D-gaps trong báo
  cáo nghiên cứu) — chỉ đổi nội dung prompt gửi đi và cách parse response.

## Acceptance Criteria

- [ ] `buildDecomposePrompt` interpolate đủ 5 nguồn context (title, description,
      aiContext, tech-stack tối thiểu 1 nguồn detect được, existing subtasks
      dedupe).
- [ ] AI response được parse dưới dạng JSON có cấu trúc; proposal có đủ
      `type/estimatedHours/dependsOnIndex/promptTemplate`; lỗi parse JSON trả
      về error rõ ràng (không âm thầm tạo 0 subtask).
- [ ] `AIApply` tạo đúng `depends_on` edge theo `DependsOnIndex`, tái sử dụng
      atomic `AddEdge` từ CR-TG-001 (test: request `AIApply` đồng thời với 1
      `AddEdge` thủ công trên cùng subtree không tạo cycle).
- [ ] `CalculateCriticalPath` trả đúng path dài nhất theo `estimated_hours`
      trên ít nhất 1 test case DAG ≥ 5 node có nhánh rẽ.
- [ ] `GenerateAgentPrompt` là RPC độc lập với `AIDecompose`, ghi kết quả vào
      `Task.PromptTemplate`.
- [ ] `Task.AIPlanJSON` lưu đúng raw response cho mọi lần decompose (kể cả lần
      parse lỗi, để debug).
