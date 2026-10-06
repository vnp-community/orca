# CONTRACT: Hợp đồng kênh WS giữa backend-go và frontend cho luồng Request (v6)

> **Nguồn sự thật cho mọi thứ frontend và agent gọi hoặc nhận từ `api-gateway` trong feature Request.**
> Các `BE-REQ-SOL-*` (specs/backend-go/crs/v6) phải hiện thực đúng file này; các `FE-REQ-SOL-*` và `AG-REQ-SOL-*` chỉ dùng những gì có ở đây. Cần đổi thì sửa file này trước, rồi cập nhật cả hai phía.
> CR gốc: [CR-REQ-016](../../../../../docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md) và [README v6](../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3). Mẫu: [`../../v5/CONTRACT-mcp-ui-api.md`](../../v5/CONTRACT-mcp-ui-api.md).
> Trạng thái: 📋 Proposed. Chưa có dòng nào được hiện thực; chưa chạy trên hệ thống. Solution backend: [`solutions/BE-REQ-SOL-016-api-gateway-request-channels.md`](./solutions/BE-REQ-SOL-016-api-gateway-request-channels.md).

## 0. Nguyên tắc tương thích

| # | Nguyên tắc | Lý do (bằng chứng) |
|---|---|---|
| C1 | UI đi qua **kênh WS `request.*`, `solution.*`, `approval.*`, `backlog.*`** đăng ký trong `wscompat.Registry` (file mới `channels_request*.go`). Không thêm REST cho UI; 5 route HTTP ở mục 4 chỉ cho script, CLI, deep link thông báo | UI gọi `callRuntimeResult(method, params)` rồi `Registry.Dispatch`; mẫu v5 C1 |
| C2 | Danh tính chỉ lấy từ session (`Identity{TenantID, UserID, Role}`), **không bao giờ** từ tham số. Tham số `tenantId`, `userId`, `reporterId`, `sourceProvider` (khi ghi) bị bỏ qua hoặc từ chối | `registry.go` `Identity`; CR-REQ-016 G1 |
| C3 | Mỗi kênh nhận **đúng một object JSON ở `args[0]`**. Kênh không tham số nhận `{}` hoặc không có `args` | mẫu v5 C10; `decodeArg[T](args, 0)` |
| C4 | Lỗi trả dạng `"<CODE>: <thông điệp ngắn>"`, `CODE` là `UPPER_SNAKE` với tiền tố `REQUEST_`. Frontend tách bằng `^([A-Z0-9_]+): `. Backend **bọc** lỗi gRPC của kênh Request bằng `requestChannelError` (mới, mẫu `mcpChannelError`) để bỏ tiền tố `rpc error: code = ... desc = ` | `session_dialect.go` gửi `err.Error()` nguyên văn; v5 C4 |
| C5 | Push dùng `RegisterStream` (`request.subscribe`, mục 3), khung `request.event` | mẫu `task.activity.subscribe`, v5 C5 |
| C6 | JSON mọi view là **camelCase**; không trả message proto thô (snake_case). Slice rỗng là `[]`, không `null` | ghi chú BUG-023 ở `channels.go`; `normalizeNilSlices` |
| C7 | Thời gian là RFC 3339 UTC (`string`); id là UUID dạng `string`; phân trang bằng `pageSize` (mặc định 20, tối đa 100) và `pageToken` opaque; kết quả có `nextPageToken` (chuỗi rỗng khi hết) | CR-REQ-015 (mặc định 20, tối đa 100); proto `page_size`/`page_token` của CR-009, 007 |
| C8 | Thêm field mới vào kiểu hoặc kênh chỉ được **additive**; client bỏ qua field lạ | tương thích tiến hoá (v5 C9) |
| C9 | `body` của Request và nội dung Solution **không** nằm trong thông điệp lỗi, log, span, khung sự kiện | CR-REQ-016 mục 2.9; CR-REQ-024 mục 2.10 |
| C10 | Mọi RPC của `request-service` tự kiểm quyền; gateway không kiểm OPA trước định tuyến. Cột "Quyền" ở mục 2 là mức `request-service` thi hành | README api-gateway ("No OPA authorization check"); README v6 mục 8 dòng 13 |
| C11 | Cờ `request_flow_enabled` do `request-service` thi hành: lệnh "đi tiếp luồng" khi cờ tắt trả `REQUEST_FLOW_DISABLED`; đọc và thoát an toàn vẫn chạy. Frontend dùng `request.flowStatus` để hiện hay ẩn màn hình | CR-REQ-025 mục 2.2 |
| C12 | Ghi có khoá lạc quan: `expectedVersion` (Request, Approval, Solution). Lệch thì `REQUEST_VERSION_CONFLICT` hoặc `REQUEST_APPROVAL_VERSION_CONFLICT`; frontend tải lại rồi hỏi người dùng | CR-REQ-005, 006, 009 |

## 1. Kiểu dùng chung (TypeScript, frontend đặt trong `frontend/src/shared/request-types.ts` (mới); backend sinh JSON bằng struct view tag `json:"camelCase"`)

```ts
type RequestType = 'change_request' | 'bug' | 'hotfix' | 'task' | 'spike' | 'question'
                 | 'refactor' | 'security' | 'performance' | 'docs' | 'ops_request'
type RequestSize = 'S' | 'M' | 'L'
type RequestUrgency = 'normal' | 'urgent'
type RequestStatus = 'new' | 'classifying' | 'awaiting_type_confirmation' | 'analyzing'
  | 'awaiting_analysis_approval' | 'planning' | 'awaiting_plan_approval' | 'executing'
  | 'completed' | 'request_backlog' | 'cancelled'
type RequestSourceProvider = 'jira' | 'github' | 'gitlab' | 'linear' | 'mcp' | 'manual' | 'webhook'
type ReturnStage = 'classification' | 'analysis' | 'plan' | 'phase' | 'task'
type LinkReason = 'spawned_by_spike' | 'spawned_by_question' | 'followup_hotfix' | 'escalation'

interface RequestView {
  id: string; projectId: string
  number: number                       // số hiển thị theo project (request_counters, CR-REQ-002)
  title: string
  body?: string                        // chỉ có ở request.get; request.list không trả
  sourceProvider: RequestSourceProvider
  sourceRef?: string; sourceUrl?: string; sourceSite?: string
  type: RequestType | null             // null tới khi có đề xuất đầu tiên (README v6 mục 8 dòng 3)
  typeSource: 'ai' | 'human' | null
  size: RequestSize | null; urgency: RequestUrgency | null
  confidence: number | null            // [0,1], chỉ khi typeSource='ai'
  classificationReason?: string
  status: RequestStatus
  returnedFromStage?: ReturnStage; returnCategory?: string; returnReason?: string
  planTaskId?: string                  // id Task type='plan'; cây đọc bằng task.getSubtree
  reporterId: string
  createdAt: string; updatedAt: string
  version: number                      // gửi lại ở expectedVersion
}

interface TypeHistoryEntryView {       // request.typeHistory
  fromType: RequestType | null; toType: RequestType
  actorId?: string; actorKind: 'ai' | 'user'
  reason?: string; at: string
}

interface RequestLinkView {            // request.links (mục 2.5, đề xuất bổ sung)
  parentRequestId: string; childRequestId: string; reason: LinkReason
}

type SolutionKind = 'solution' | 'diagnosis' | 'findings' | 'answer'
type SolutionStatus = 'draft' | 'proposed' | 'approved' | 'rejected' | 'superseded'

interface SolutionView {
  id: string; requestId: string; kind: SolutionKind; status: SolutionStatus
  options: unknown | null              // object theo schema CR-REQ-007 mục 2.3 (kind=solution: {options[], recommendation, assumptions, openQuestions}); kind khác: tài liệu của CR-REQ-008. Khoá JSON bên trong **giữ snake_case của CR** vì là nội dung AI sinh (xem C13)
  chosenOption: number                 // chỉ số 0-based; -1 = chưa chọn
  chosenOptionId?: string              // 'opt-N', gateway suy từ chosenOption
  generationRunId?: string
  createdAt: string
  version: number
}

interface AnalysisRunView {
  id: string; kind: SolutionKind
  status: 'running' | 'succeeded' | 'failed'
  errorCode?: string; errorMessage?: string
  startedAt: string; finishedAt?: string
}

type ApprovalSubjectType = 'request_type' | 'solution' | 'findings' | 'answer'
  | 'plan' | 'phase' | 'task_list' | 'pre_deploy'
type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'cancelled' | 'expired'

interface ApprovalView {
  id: string; requestId: string
  subjectType: ApprovalSubjectType; subjectId: string; stage: string
  status: ApprovalStatus
  requestedBy: string; decidedBy?: string; decidedAt?: string; comment?: string
  dueAt?: string
  version: number
  subjectDigest: string                // gửi lại ở expectedDigest (CR-REQ-009)
  createdAt: string
}

interface BacklogRequestRowView {      // backlog.requests
  requestId: string; number: number; title: string; type: RequestType | null
  sourceProvider: RequestSourceProvider; sourceRef?: string; sourceUrl?: string
  returnedFromStage: ReturnStage; returnedCategory?: string; returnReason?: string
  returnedBy?: string; returnedAt: string; parentRequestIds: string[]
}
interface BacklogTaskRowView {
  taskId: string; title: string; status: string; estimatedHours?: number
  assigneeId?: string; blockedByTaskIds: string[]
  lastEngine?: string; lastLinkStatus?: string; failedAttempts: number; lastError?: string
}
interface BacklogGroupView {           // backlog.tasks, backlog.execute
  requestId: string; planTaskId?: string; planTitle?: string
  phaseTaskId?: string; phaseTitle?: string; gateStatus?: string
  tasks: BacklogTaskRowView[]
}

interface RequestEventFrame {          // mục 3
  requestId: string
  eventType: RequestEventType
  status?: RequestStatus; type?: RequestType | null
  occurredAt: string
  trigger?: string                     // chỉ request.status_changed
  approvalId?: string; subjectType?: ApprovalSubjectType   // approval.*
  solutionId?: string                  // solution.*
  phaseTaskId?: string; planTaskId?: string                 // phase.*, plan.generated
}
```

**C13.** `SolutionView.options` là JSON do AI sinh, validate ở backend (CR-REQ-007 mục 2.3); gateway chuyển nguyên văn, không đổi khoá. Frontend không giả định khoá camelCase bên trong `options`. Việc đổi khoá bên trong là thay đổi hợp đồng cần sửa file này.

## 2. Kênh

Cột "Quyền" là mức tối thiểu `request-service` thi hành. "Timeout" là deadline context của gateway cho RPC (bảng thật ở BE-REQ-SOL-016 mục 2.4). Kết quả danh sách luôn là object có `nextPageToken`.

### 2.1 `request.*` (vòng đời)

| Kênh | Tham số (`args[0]`) | Kết quả | Quyền | Timeout | RPC |
|---|---|---|---|---|---|
| `request.flowStatus` | `{}` | `{enabled: boolean}` | thành viên tenant | 8s | `GetRequestFlowSettings` |
| `request.flowSet` | `{enabled}` | `{enabled}` | `Identity.Role=admin` | 8s | `SetRequestFlowSettings` |
| `request.create` | `{projectId, title (1..500), body? (≤100000), source?: {provider, ref, url, site}, hints?: {issueType, labels[], priority}, clientRequestId? (≤128)}` | `{request: RequestView, created: boolean}` | ghi trên project | 8s | `CreateRequest` |
| `request.get` | `{id}` | `{request: RequestView}` (có `body`) | đọc | 8s | `GetRequest` |
| `request.list` | `{projectId?, status?: RequestStatus[], type?: RequestType[], sourceProvider?, sourceSite?, sourceRef?, pageSize?, pageToken?}` | `{requests: RequestView[], nextPageToken}` | đọc trên project | 8s | `ListRequests` |
| `request.typeHistory` | `{id}` | `{changes: TypeHistoryEntryView[]}` | đọc | 8s | `ListRequestTypeHistory` |
| `request.classify` | `{id}` | `{request: RequestView}` | ghi | 24s (xem ghi chú) | `ClassifyRequest` |
| `request.confirmType` | `{id, type, size?, urgency?, reason?, expectedVersion}` | `{request: RequestView}` | người duyệt `request_type` | 8s | `ConfirmRequestType` |
| `request.changeType` | `{id, toType, size?, urgency?, reason, expectedVersion}` | `{request: RequestView}` | ghi | 8s | `ChangeRequestType` (`new_type`) |
| `request.returnToBacklog` | `{id, stage: ReturnStage, category?, reason, expectedVersion?}` | `{request: RequestView}` | ghi | 8s | `ReturnToBacklog` |
| `request.reopen` | `{id, note?, expectedVersion?}` | `{request: RequestView}` | ghi | 8s | `ReopenRequest` |
| `request.cancel` | `{id, reason, expectedVersion?}` | `{request: RequestView}` | ghi, chủ Request hoặc admin project | 8s | `CancelRequest` |
| `request.spawnChild` | `{id, linkReason: LinkReason, title, body?, typeHint?: RequestType, clientRequestId?}` | `{child: RequestView, created: boolean}` | ghi | 8s | `SpawnChildRequest` |
| `request.generatePlan` | `{id, mode?: 'propose'\|'commit' (mặc định 'propose'), proposal?, rawAiResponse?}` | `{proposal, planTaskId?, phaseTaskIds[], taskIds[], alreadyExists}` | ghi | 24s (propose, xem ghi chú), 15s (commit) | `GeneratePlan` |
| `request.startPhase` | `{id, phaseTaskId?}` | `{phaseTaskId, alreadyStarted, dispatchedTaskIds[]}` | exec trên project | 15s | `StartPhase` |
| `request.subscribe` | `{id?}` | stream `request.event` (mục 3) | đọc | n/a | (NATS) |

Ghi chú:
- `request.create`: `source.provider` ngoài tập `jira|github|gitlab|linear` bị từ chối `REQUEST_SOURCE_FORBIDDEN` (quy tắc ở mục 6.1). `created=false` khi gọi lặp cùng khoá idempotent.
- `request.confirmType` bắt buộc người dùng thật; tool MCP không có kênh này (CR-REQ-017 mục 2.2).
- `request.generatePlan` hai pha theo CR-REQ-012: `propose` không ghi gì, trả `proposal`; `commit` bắt buộc `proposal` (có thể đã sửa tay) và nên gửi `rawAiResponse` từ lần `propose`. Bỏ `mode` thì là `propose`.
- **Trần 25 giây của WS.** `wscompat/handler.go` đặt `invokeTimeout = 25s` cho mọi dispatch (khớp `INVOKE_TIMEOUT_MS` 30s của `rpc-client.ts`). `ClassifyRequest` (AI 60 giây, CR-REQ-005) và `GeneratePlan` propose (`ai.complete` đồng bộ, CR-REQ-012) vượt trần này; gateway đặt deadline 24 giây để lỗi là `REQUEST_AI_COMPLETE_TIMEOUT` của ta. **Yêu cầu với CR-REQ-005 và 012:** RPC phải trả sớm (enqueue và trả `Request` ở `classifying`; kết quả qua sự kiện `request.classified`), hoặc propose bất đồng bộ trả `runId` rồi báo qua sự kiện. Cho tới khi chốt (mục 9 Q2), frontend coi hai kênh này có thể trả `REQUEST_AI_COMPLETE_TIMEOUT` và phải chờ sự kiện hoặc polling `request.get`.
- `request.startPhase`: `phaseTaskId` rỗng chỉ hợp lệ khi Plan không có Phase (CR-REQ-013 mục 2.3).
- Cây Plan, Phase, Task đọc bằng kênh `task.getSubtree` hiện có với `rootTaskId = request.planTaskId`; không có kênh `plan.*`, `phase.*` (CR-REQ-016 D1).

### 2.2 `solution.*`

| Kênh | Tham số | Kết quả | Quyền | Timeout | RPC |
|---|---|---|---|---|---|
| `solution.list` | `{requestId, kind?, status?, pageSize?, pageToken?}` | `{solutions: SolutionView[], runs: AnalysisRunView[], nextPageToken}` | đọc | 8s | `ListSolutions` |
| `solution.generate` | `{requestId, idempotencyKey?, feedback? (≤2000), analysisMode?: 'complete'\|'agent_readonly'}` | `{solutionId, runId}` | ghi | 8s | `GenerateSolution` |
| `solution.choose` | `{requestId, solutionId, optionId, comment?}` | `{solution: SolutionView, approvalDigest}` | người duyệt `solution` | 8s | `ChooseSolutionOption` |

`solution.generate` **bất đồng bộ** (CR-REQ-007 mục 2.5): trả ngay `runId`; hoàn tất qua sự kiện `solution.proposed`, hoặc lỗi nằm ở `runs[].errorCode` (`REQUEST_SOLUTION_INVALID_OUTPUT`, `REQUEST_SOLUTION_RUN_INTERRUPTED`). Frontend không giữ yêu cầu chờ 25 giây. `solution.choose` trả `approvalDigest` mới; gửi lại ở `approval.approve.expectedDigest`.

### 2.3 `approval.*`

| Kênh | Tham số | Kết quả | Quyền | Timeout | RPC |
|---|---|---|---|---|---|
| `approval.get` | `{id}` | `{approval: ApprovalView}` | đọc Request chứa nó | 8s | `GetApproval` |
| `approval.list` | `{requestId, subjectType?, status?, pageSize?, pageToken?}` | `{approvals: ApprovalView[], nextPageToken}` | đọc | 8s | `ListApprovals` |
| `approval.listPending` | `{subjectType?, pageSize?, pageToken?}` | `{approvals: ApprovalView[], nextPageToken}` | người dùng hiện tại | 8s | `ListPendingForUser` |
| `approval.approve` | `{id, expectedVersion, expectedDigest, comment?}` | `{approval: ApprovalView, requestStatus: RequestStatus}` | người duyệt (CR-REQ-009, 010) | 8s | `Approve` |
| `approval.reject` | `{id, expectedVersion, expectedDigest, comment (bắt buộc)}` | `{approval, requestStatus}` | như trên | 8s | `Reject` |
| `approval.cancel` | `{id, reason}` | `{approval}` | người yêu cầu hoặc admin | 8s | `Cancel` |

Không có kênh `approval.request`: Approval do máy trạng thái sinh (CR-REQ-016 D4). `approval.listPending` chỉ chứa Approval mà người dùng duyệt được (CR-REQ-010 mục 2.8). Hàng chờ cần tiêu đề Request: frontend gọi `request.get` theo `requestId` (đề xuất gắn `requestTitle`, `requestType`, `requestNumber` vào `approval.listPending` là việc additive, chờ chốt ở mục 9 câu Q3).

### 2.4 `backlog.*`

Cả ba kênh gọi **một RPC** `ListBacklog` với `view` (CR-REQ-015 mục 2.2); không có RPC riêng cho Task và Execute.

| Kênh | Tham số | Kết quả | `view` |
|---|---|---|---|
| `backlog.requests` | `{projectId?, requestTypes?: RequestType[], categories?: string[], pageSize?, pageToken?}` | `{requestRows: BacklogRequestRowView[], nextPageToken}` | `REQUEST` |
| `backlog.tasks` | `{projectId?, requestId?, planTaskId?, assigneeId?, pageSize?, pageToken?}` | `{groups: BacklogGroupView[], nextPageToken}` | `TASK` |
| `backlog.execute` | `{projectId?, requestId?, phaseTaskId?, assigneeId?, pageSize?, pageToken?}` | `{groups: BacklogGroupView[], nextPageToken}` | `EXECUTE` |

Quyền: đọc. `pageSize` đếm theo Request (CR-REQ-015 mục 2.5). Lỗi riêng: `REQUEST_BACKLOG_BAD_PAGE_TOKEN`, `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE`.

### 2.5 Kênh bổ sung đề xuất (cần chốt, không nằm trong 28 kênh của CR-REQ-016)

README v6 mục 8 dòng 12 thêm RPC mà CR-016 chưa có kênh. Backend dự trù, frontend **chưa được dùng** tới khi mục 9 câu Q1 chốt:

| Kênh | Tham số | Kết quả | RPC | Lý do cần |
|---|---|---|---|---|
| `request.links` | `{id}` | `{parents: RequestLinkView[], children: RequestLinkView[]}` | `ListRequestLinks` (CR-REQ-006) | hiện Request cha, con ở trang chi tiết (CR-REQ-019 hỏi `request.get` có `links` không) |
| `request.flow` | `{type, size?}` | `{steps: [...], phases: boolean, gates: ApprovalSubjectType[]}` (hình dạng chốt ở CR-REQ-003 Q2) | `GetRequestFlow` | frontend không sao chép registry luồng |
| `request.checks` | `{id}` | `{checks: [...]}` | `ListRequestChecks` (CR-REQ-014) | số đo `performance`, `refactor` |

`RecordRequestCheck` không có kênh WS ở v1; chỉ đường MCP hoặc nội bộ (CR-REQ-014 Q1, CR-REQ-017 Q).

## 3. Sự kiện thời gian thực: `request.subscribe`

- Mở: `request.subscribe {id?}`. Có `id`: chỉ sự kiện của Request đó; không `id`: mọi sự kiện Request trong tenant của người dùng, lọc thêm theo quyền đọc. Gateway gọi `GetRequest` một lần để kiểm quyền khi có `id`.
- Đóng: frontend gọi hàm teardown (mẫu `client.subscribe`); gateway huỷ `ctx`.
- Khung: kênh `request.event`, `args: [RequestEventFrame]`. Không kèm `body`, `title`, nội dung Solution.
- Kênh chỉ đăng ký khi NATS kết nối (cùng cách `TaskActivityEnabled`). Frontend phát hiện bằng lỗi `REQUEST_UNAVAILABLE` hoặc kênh không có, rồi **polling**: Request đang mở 15 giây, `approval.listPending {pageSize:1}` 60 giây, chỉ khi cửa sổ hiển thị (CR-REQ-018 mục 2).
- **Không bảo đảm giao đủ** (ephemeral consumer không có con trỏ bền, `common/eventbus` `SubscribeEphemeral`): sau mỗi lần kết nối lại hoặc mở màn hình, frontend phải tải lại bằng kênh đọc, không dựa vào lịch sử sự kiện.
- Gateway bỏ sự kiện có `occurredAt` trước thời điểm đăng ký (consumer mới có thể phát lại lịch sử stream; xem BE-REQ-SOL-016 mục 6).

`RequestEventType` (đã ánh xạ từ subject `orca.request.<entity>.<event>`, README v6 mục 8 dòng 4):

| `eventType` | Subject NATS | Khi nào frontend làm gì |
|---|---|---|
| `request.created` | `orca.request.request.created` | thêm vào danh sách |
| `request.classified` | `orca.request.request.classified` | làm mới Request; `failed=true` nghĩa người dùng chọn loại tay |
| `request.type_confirmed` | `orca.request.request.type_confirmed` | làm mới |
| `request.type_changed` | `orca.request.request.type_changed` | làm mới Request và lịch sử loại |
| `request.status_changed` | `orca.request.request.status_changed` | làm mới Request; kèm `trigger` (`reopen`, `cancel`... thay cho sự kiện `reopened`, `cancelled` không tồn tại) |
| `request.returned` | suy từ `status_changed` với `to=request_backlog`, hoặc subject riêng nếu CR-006 phát | làm mới Request và backlog |
| `request.completed` | `orca.request.request.completed` | làm mới |
| `solution.proposed` | `orca.request.solution.proposed` | tải `solution.list`; dừng trạng thái "đang sinh" |
| `solution.approved` | `orca.request.solution.approved` | làm mới |
| `approval.requested` | `orca.request.approval.requested` | tăng chấm số hộp duyệt; tải `approval.list` |
| `approval.decided` | `orca.request.approval.decided` | làm mới Approval và Request |
| `plan.generated` | `orca.request.plan.generated` | tải `task.getSubtree` |
| `phase.started` | `orca.request.phase.started` | tải cây, backlog execute |
| `phase.completed` | `orca.request.phase.completed` | tải cây, Approval Phase kế |

## 4. Route HTTP (chỉ năm)

| Method và đường dẫn | Tương đương kênh | Body, trả về |
|---|---|---|
| `POST /v1/requests` | `request.create` | cùng tham số, `source` theo mục 6.1 |
| `GET /v1/requests`, `GET /v1/requests/{id}` | `request.list`, `request.get` | query: `projectId`, `status`, `type`, `pageSize`, `pageToken` |
| `GET /v1/approvals/pending` | `approval.listPending` | query: `subjectType`, `pageSize`, `pageToken`; dùng cho deep link thông báo |
| `POST /v1/approvals/{id}/approve`, `.../reject` | `approval.approve`, `approval.reject` | body `{version, digest, comment}` (CR-016 chỉ ghi `{version, comment}`, thiếu `digest`; xem mục 8) |

Lỗi JSON: 400 `INVALID_ARGUMENT`; chưa đăng nhập: 401; lỗi gRPC qua `writeGRPCError` (mẫu `task_routes.go`). `tenant_id` không nằm trong body.

## 5. Mã lỗi

Tiền tố `REQUEST_` cho **mọi** mã (README v6 mục 8 dòng 5; CR-REQ-007 và 009 đã dùng). CR-REQ-016 mục 2.8 liệt kê dạng không tiền tố (`APPROVAL_NOT_FOUND`, `SOLUTION_NOT_FOUND`, `APPROVAL_VERSION_CONFLICT`...): đó là tên viết tắt, **hợp đồng chốt tên có tiền tố**; bảng ánh xạ ở mục 8.

| Mã | gRPC | Khi nào | Frontend |
|---|---|---|---|
| `REQUEST_NOT_FOUND`, `REQUEST_APPROVAL_NOT_FOUND`, `REQUEST_SOLUTION_NOT_FOUND` | NotFound | id sai hoặc khác tenant | làm mới, về danh sách |
| `REQUEST_TRANSITION_NOT_ALLOWED`, `REQUEST_STATE_STALE` | FailedPrecondition | thao tác không hợp lệ ở trạng thái hiện tại | tải lại Request |
| `REQUEST_VERSION_CONFLICT`, `REQUEST_APPROVAL_VERSION_CONFLICT`, `REQUEST_SOLUTION_VERSION_CONFLICT` (xem Q4) | FailedPrecondition | `expectedVersion` cũ | tải lại, hỏi người dùng |
| `REQUEST_TYPE_REQUIRED`, `REQUEST_SIZE_REQUIRED`, `REQUEST_HOTFIX_REQUIRES_URGENT`, `REQUEST_INVALID_TYPE`, `REQUEST_TYPE_UNCHANGED` | InvalidArgument hoặc FailedPrecondition | dữ liệu xác nhận loại sai | hiện lỗi cạnh trường |
| `REQUEST_TYPE_CHANGE_NOT_ALLOWED`, `REQUEST_TYPE_CHANGE_USE_CHILD`, `REQUEST_TYPE_CHANGE_BLOCKED_ACTIVE_EXECUTION` | FailedPrecondition | đường đổi loại không cho phép | gợi ý tạo Request con hoặc chờ task dừng |
| `REQUEST_CLASSIFICATION_LIMIT` | FailedPrecondition | quá 5 lần phân loại AI | ẩn nút phân loại lại |
| `REQUEST_REASON_REQUIRED`, `REQUEST_APPROVAL_COMMENT_REQUIRED` | InvalidArgument | thiếu `reason` hoặc `comment` | hiện lỗi cạnh trường |
| `REQUEST_RETURN_STAGE_INVALID`, `REQUEST_RETURN_CATEGORY_INVALID`, `REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION`, `REQUEST_REOPEN_NOT_ALLOWED`, `REQUEST_CANCEL_NOT_ALLOWED`, `REQUEST_CANCEL_BLOCKED_ACTIVE_EXECUTION` | FailedPrecondition | CR-REQ-006 | toast |
| `REQUEST_CHILD_NOT_ALLOWED`, `REQUEST_CHILD_LIMIT`, `REQUEST_CHILD_DEPTH_EXCEEDED`, `REQUEST_LINK_SELF`, `REQUEST_PARENT_NOT_FOUND` | FailedPrecondition | `request.spawnChild` | toast |
| `REQUEST_SOURCE_PROVIDER_INVALID`, `REQUEST_SOURCE_REF_REQUIRED`, `REQUEST_TITLE_REQUIRED`, `REQUEST_BODY_TOO_LARGE`, `REQUEST_PROJECT_REQUIRED` | InvalidArgument | `request.create` | lỗi cạnh trường |
| `REQUEST_SOURCE_FORBIDDEN` | PermissionDenied | client WS khai nguồn `mcp`, `webhook`, `manual` (do **gateway** trả) | không xảy ra với UI đúng |
| `REQUEST_SOURCE_NOT_FOUND`, `REQUEST_SOURCE_FETCH_FAILED` | NotFound, Unavailable | không lấy được issue nguồn | toast, cho nhập tay |
| `REQUEST_SOLUTION_WRONG_STATE`, `REQUEST_SOLUTION_KIND_NOT_ALLOWED`, `REQUEST_SOLUTION_NOT_PROPOSED`, `REQUEST_SOLUTION_OPTION_NOT_FOUND`, `REQUEST_SOLUTION_OPTION_NOT_CHOSEN` | FailedPrecondition, InvalidArgument | CR-REQ-007 | toast; tải lại Solution |
| `REQUEST_SOLUTION_NO_CONNECTION`, `REQUEST_AI_NO_PROVIDER`, `REQUEST_ANALYSIS_NO_CONNECTION`, `REQUEST_ANALYSIS_BUSY` | FailedPrecondition | project chưa có dev server, chưa có AI | hướng dẫn kết nối dev server |
| `REQUEST_SOLUTION_INVALID_OUTPUT`, `REQUEST_SOLUTION_RUN_INTERRUPTED`, `REQUEST_ANALYSIS_*` | (nằm ở `runs[].errorCode`, không ném) | run sinh Solution lỗi | nút "Sinh lại" |
| `REQUEST_PLAN_NOT_APPLICABLE`, `REQUEST_PLAN_SOLUTION_NOT_APPROVED`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_PLAN_*` (kiểm đề xuất, CR-REQ-012 mục 2.4), `REQUEST_AI_COMPLETE_TIMEOUT` | FailedPrecondition, InvalidArgument | `request.generatePlan` | hiện lỗi cụ thể; `REQUEST_PLAN_AI_INVALID_JSON` kèm `raw` để người xem |
| `REQUEST_PHASE_NOT_APPROVED`, `REQUEST_PHASE_PREDECESSOR_NOT_DONE`, `REQUEST_PHASE_NOT_IN_PLAN`, `REQUEST_NOT_EXECUTING`, `REQUEST_EXECUTE_FORBIDDEN` | FailedPrecondition, PermissionDenied | `request.startPhase` | toast |
| `REQUEST_APPROVAL_ALREADY_DECIDED`, `REQUEST_APPROVAL_EXPIRED`, `REQUEST_APPROVAL_DIGEST_MISMATCH`, `REQUEST_APPROVAL_STAGE_MISMATCH` | FailedPrecondition | duyệt lặp, quá hạn, nội dung đã đổi | tải lại cổng |
| `REQUEST_APPROVAL_NOT_APPROVER`, `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`, `REQUEST_APPROVAL_AGENT_FORBIDDEN`, `REQUEST_APPROVAL_FORBIDDEN`, `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` | PermissionDenied, FailedPrecondition | quyền duyệt (CR-REQ-010) | toast, ẩn nút, tải lại |
| `REQUEST_BACKLOG_INVALID_VIEW`, `REQUEST_BACKLOG_BAD_PAGE_TOKEN`, `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` | InvalidArgument, Unavailable | `backlog.*` | thử lại |
| `REQUEST_FLOW_DISABLED` | FailedPrecondition | cờ tắt | ẩn màn hình Request |
| `REQUEST_RATE_LIMITED`, `REQUEST_PENDING_LIMIT` | ResourceExhausted | chống spam (chủ yếu nguồn MCP) | toast kèm thời gian chờ |
| `REQUEST_UNAVAILABLE` | Unavailable | chưa cấu hình `REQUEST_SERVICE_ADDR` hoặc service không trả lời (do **gateway** trả) | trạng thái "không khả dụng" |

Danh sách này gộp từ CR-REQ-003 đến 016; mã mới của CR sau (026 đến 036) thêm additive.

## 6. Quy tắc chéo

### 6.1 Nguồn Request

| Tình huống | `sourceProvider` gửi xuống `request-service` |
|---|---|
| Gọi qua MCP (`ToolOrigin` trong `ctx`) | `mcp`; `sourceSite = ClientName`; client không ghi đè |
| WS gửi `source.provider` ∈ `jira|github|gitlab|linear` kèm `ref` | giữ nguyên (nút "Tạo Request" ở trang Tasks) |
| WS không gửi `source` | `manual` |
| WS gửi `mcp`, `webhook`, `manual` tường minh | `REQUEST_SOURCE_FORBIDDEN` |

### 6.2 Quyền theo vai trò (frontend)

Frontend không tự suy quyền duyệt: hiện nút Duyệt, Từ chối khi Approval nằm trong `approval.listPending` của người dùng, hoặc `approval.get` cho phép (lỗi `REQUEST_APPROVAL_NOT_APPROVER` khi ghi là nguồn chân lý). `request.flowSet` chỉ hiện cho `Role=admin`.

## 7. Chỗ mở rộng cho CR 026 đến 036

Các CR bổ sung (`clarification.*`, `decision.*`, `impact.*`, `readiness.*`, an ninh CR-REQ-035) soạn song song, **chưa phụ thuộc**. Quy tắc để thêm kênh mà không phá hợp đồng:

| Quy tắc | Nội dung |
|---|---|
| Tiền tố | mỗi nhóm một tiền tố kênh riêng (`clarification.*`, `decision.*`, `impact.*`, `readiness.*`); không thêm vào `request.*` trừ thao tác trên chính Request |
| File | mỗi nhóm một `channels_<nhóm>.go` trong `wscompat`, đăng ký trong `registerRequestChannels` cùng `ChannelDeps.Request` hoặc client mới cùng service |
| Khuôn | một object ở `args[0]`, view camelCase, phân trang `pageSize`/`pageToken`, lỗi `REQUEST_<NHÓM>_*`, `expectedVersion` ở ghi |
| Sự kiện | thêm giá trị vào `RequestEventType` (additive); subject `orca.request.<entity>.<event>`; gateway chỉ thêm vào bảng ánh xạ whitelist |
| MCP | mỗi kênh mới phải có `ToolSpec` hoặc dòng trong `excluded_channels.yaml` (parity test); kênh quyết định của người (`decision.*` duyệt) mặc định loại trừ vĩnh viễn |
| Cờ | đi qua `request_flow_enabled`; nếu cần cờ con, đặt trong `GetRequestFlowSettings` dạng additive (`{enabled, features?: {...}}`) |
| Bảo mật | CR-REQ-035 có thể đổi quyền ghi theo kênh: hợp đồng chỉ cam kết "request-service quyết quyền", nên đổi chính sách không đổi tên kênh |

Gateway giữ một bảng ánh xạ duy nhất `requestEventRegistry` (BE-REQ-SOL-016 mục 2.5) để thêm sự kiện là thêm một dòng.

## 8. Lệch giữa CR và chốt trong hợp đồng này

| # | Nơi lệch | Chốt ở đây |
|---|---|---|
| 1 | CR-016 mã lỗi không tiền tố vs CR-007, 009 và README v6 mục 8 dòng 5 (`REQUEST_APPROVAL_*`, `REQUEST_SOLUTION_*`) | dùng tên có tiền tố; `APPROVAL_NOT_FOUND`→`REQUEST_APPROVAL_NOT_FOUND`, `APPROVAL_ALREADY_DECIDED`→`REQUEST_APPROVAL_ALREADY_DECIDED`, `APPROVAL_EXPIRED`→`REQUEST_APPROVAL_EXPIRED`, `APPROVAL_NOT_APPROVER`→`REQUEST_APPROVAL_NOT_APPROVER`, `APPROVAL_COMMENT_REQUIRED`→`REQUEST_APPROVAL_COMMENT_REQUIRED`, `APPROVAL_VERSION_CONFLICT`→`REQUEST_APPROVAL_VERSION_CONFLICT`, `SOLUTION_NOT_FOUND`→`REQUEST_SOLUTION_NOT_FOUND` |
| 2 | Frontend CR dùng `solution.chooseOption`, `backlog.list`, `request.listHistory`, `request.events.subscribe`, `request.getPlan` | chốt tên CR-016: `solution.choose`, `backlog.requests|tasks|execute`, `request.typeHistory`, `request.subscribe`; không có `request.getPlan` (dùng `task.getSubtree`) |
| 3 | CR-016 `request.spawnChild` dùng `reason`, `type?` vs proto CR-006 `link_reason`, `type_hint` | WS dùng `linkReason`, `typeHint` (tránh nhầm `reason` với lý do trả backlog) |
| 4 | CR-016 `request.generatePlan {id}` vs CR-012 có `mode`, `proposal`, `raw_ai_response` | thêm `mode`, `proposal`, `rawAiResponse` |
| 5 | CR-016 `request.startPhase` bắt buộc `phaseTaskId` vs CR-013 cho rỗng | tuỳ chọn |
| 6 | CR-016 thiếu `expectedVersion` ở `confirmType`, `changeType`, `returnToBacklog`, `reopen`, `cancel` | thêm theo CR-005, 006 |
| 7 | CR-016 đặt 25s cho `request.classify`, `request.generatePlan`; CR-005 gọi AI 60s, CR-012 gọi `ai.complete` đồng bộ; `invokeTimeout = 25s` trong `wscompat/handler.go` chặn mọi kênh | deadline gateway 24s; hai CR phải làm RPC trả sớm (mục 2.1 ghi chú, Q2) |
| 8 | CR-016 `solution.generate` timeout 25s vs CR-007 bất đồng bộ | 8s, kết quả qua sự kiện |
| 9 | CR-016 `backlog.tasks`/`backlog.execute` "RPC của CR-015 (tên chốt ở đó)" | một RPC `ListBacklog` với `view` |
| 10 | CR-016 HTTP approve body `{version, comment}` | thêm `digest` (CR-009 `expected_digest` bắt buộc) |

## 9. Câu hỏi mở

1. **Q1.** Chấp nhận ba kênh bổ sung ở mục 2.5 (`request.links`, `request.flow`, `request.checks`)? Hình dạng `request.flow` phụ thuộc CR-REQ-003 Q2.
2. **Q2 (chặn).** `ClassifyRequest` và `GeneratePlan` propose cần quá 25 giây, vượt `invokeTimeout`. Chọn: (a) RPC trả sớm, kết quả qua `request.classified` và một sự kiện mới `plan.proposed` kèm cách đọc đề xuất (ví dụ lưu `plan_proposals` hoặc dùng `analysis_runs`), như `solution.generate`; (b) nâng `invokeTimeout` riêng cho hai kênh (chạm `handler.go` và `rpc-client.ts`). Cần chốt ở CR-REQ-005 và 012.
3. **Q3.** `approval.listPending` có kèm `requestTitle`, `requestType`, `requestNumber` (một lời gọi, ít N+1) hay frontend gọi thêm `request.get`? Cần CR-REQ-009 hoặc 022 chốt.
4. **Q4.** `SOLUTION_VERSION_CONFLICT` có trong CR-016 nhưng CR-REQ-007 không khai mã đó (`ChooseSolutionOption` dùng so sánh-và-ghi); giữ hay bỏ.
5. **Q5.** `request.returned`: có subject riêng hay suy từ `status_changed` (`to=request_backlog`)? README v6 mục 8 dòng 4 chỉ nói bỏ `reopened`, `cancelled`.
6. **Q6.** Tên stream NATS (`REQUEST`?) và việc `SubscribeEphemeral` có hỗ trợ wildcard `orca.request.>` chưa kiểm chứng.
7. **Q7.** `request.list` trả `total`? Frontend CR-REQ-023 hỏi cho backlog; hợp đồng hiện không có `total` (đếm tốn kém trên hai DB).
