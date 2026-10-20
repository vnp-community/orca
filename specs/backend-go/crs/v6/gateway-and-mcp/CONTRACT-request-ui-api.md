# CONTRACT: Hợp đồng kênh WS giữa backend-go và frontend cho luồng Request (v6)

> **Nguồn sự thật cho mọi thứ frontend và agent gọi hoặc nhận từ `api-gateway` trong feature Request.**
> Các `BE-REQ-SOL-*` (specs/backend-go/crs/v6) phải hiện thực đúng file này; các `FE-REQ-SOL-*` và `AG-REQ-SOL-*` chỉ dùng những gì có ở đây. Cần đổi thì sửa file này trước, rồi cập nhật cả hai phía.
> CR gốc: [CR-REQ-016](../../../../../docs/crs/v6/gateway-and-mcp/CR-REQ-016-api-gateway-request-channels.md) và [README v6](../../../../../docs/crs/v6/README.md) (mục 8 thắng mục 3). Mẫu: [`../../v5/CONTRACT-mcp-ui-api.md`](../../v5/CONTRACT-mcp-ui-api.md).
> Trạng thái: ✅ **28 kênh WS, stream `request.subscribe` (14 sự kiện), 5 route HTTP và 4 kênh bổ sung (2.5 và `request.planProposal`) đã thật trong api-gateway** (2026-10-08, test đơn vị và bufconn; chưa chạy với request-service và NATS thật). Phía `request-service` một số RPC còn `Unimplemented` và trả `REQUEST_NOT_IMPLEMENTED`. Solution backend: [`solutions/BE-REQ-SOL-016-api-gateway-request-channels.md`](./solutions/BE-REQ-SOL-016-api-gateway-request-channels.md).

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
  baseRef: string                      // CR-REQ-039: nhánh hoặc tag gốc của worktree; '' = nhánh mặc định. LUÔN có mặt (mục 2.11)
  baseRefLocked?: boolean              // chỉ có ở request.get, list, create, setBaseRef (mục 2.11)
  baseRefLockReason?: '' | 'proposal' | 'task' | 'execution' | 'terminal'
  baseRefUnverified?: boolean          // chỉ có ở request.create, setBaseRef
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
  seq?: number                          // số phiên bản trong một Request (1, 2, ...); vắng với bản nháp chưa được lập chỉ mục (CR-REQ-041)
  revisionFeedback?: string            // người dùng đã yêu cầu gì để sinh ra phiên bản này; ai đọc được Request thì đọc được; vắng với bản đầu (CR-REQ-041)
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
| `request.flowStatus` | `{}` | `{enabled: boolean, features: string[]}` (mục 2.11) | thành viên tenant | 8s | `GetRequestFlowSettings` |
| `request.flowSet` | `{enabled}` | `{enabled}` | `Identity.Role=admin` | 8s | `SetRequestFlowSettings` |
| `request.create` | `{projectId, title (1..500), body? (≤100000), source?: {provider, ref, url, site}, hints?: {issueType, labels[], priority}, clientRequestId? (≤128), baseRef? (≤255, mục 2.11)}` | `{request: RequestView, created: boolean}` | ghi trên project | 8s | `CreateRequest` |
| `request.get` | `{id}` | `{request: RequestView}` (có `body`) | đọc | 8s | `GetRequest` |
| `request.list` | `{projectId?, status?: RequestStatus[], type?: RequestType[], sourceProvider?, sourceSite?, sourceRef?, pageSize?, pageToken?}` | `{requests: RequestView[], nextPageToken}` | đọc trên project | 8s | `ListRequests` |
| `request.typeHistory` | `{id}` | `{changes: TypeHistoryEntryView[]}` | đọc | 8s | `ListRequestTypeHistory` |
| `request.classify` | `{id}` | `{request: RequestView}` | ghi | 24s (xem ghi chú) | `ClassifyRequest` |
| `request.confirmType` | `{id, type, size?, urgency?, reason?, expectedVersion}` | `{request: RequestView}` | người duyệt `request_type` | 8s | `ConfirmRequestType` |
| `request.changeType` | `{id, toType, size?, urgency?, reason, expectedVersion}` | `{request: RequestView}` | ghi | 8s | `ChangeRequestType` (`new_type`) |
| `request.setBaseRef` | `{id, baseRef, expectedVersion?}` (mục 2.11) | `{request: RequestView}` | chủ Request hoặc người báo (nhóm triage), không phải agent | 8s | `SetRequestBaseRef` |
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
- `request.planProposal {id, runId}` (bổ sung, RPC `GetPlanProposal`) trả `{runId, status, errorCode?, errorMessage?, proposal|null, rawAiResponse}`; `request.generatePlan` mode `propose` trả `{runId, proposal:null, rawAiResponse}` (chạy nền, quyết định D3), `request.classify` trả thêm `runId`. `request.flow` trả các trường của `GetRequestFlowResponse` (camelCase). Mã `REQUEST_FORBIDDEN`, `REQUEST_NOT_IMPLEMENTED`, `REQUEST_INTERNAL` do gateway thêm (additive). `request.generatePlan` hai pha theo CR-REQ-012: `propose` không ghi gì, trả `proposal`; `commit` bắt buộc `proposal` (có thể đã sửa tay) và nên gửi `rawAiResponse` từ lần `propose`. Bỏ `mode` thì là `propose`.
- **Trần 25 giây của WS.** `wscompat/handler.go` đặt `invokeTimeout = 25s` cho mọi dispatch (khớp `INVOKE_TIMEOUT_MS` 30s của `rpc-client.ts`). `ClassifyRequest` (AI 60 giây, CR-REQ-005) và `GeneratePlan` propose (`ai.complete` đồng bộ, CR-REQ-012) vượt trần này; gateway đặt deadline 24 giây để lỗi là `REQUEST_AI_COMPLETE_TIMEOUT` của ta. **Yêu cầu với CR-REQ-005 và 012:** RPC phải trả sớm (enqueue và trả `Request` ở `classifying`; kết quả qua sự kiện `request.classified`), hoặc propose bất đồng bộ trả `runId` rồi báo qua sự kiện. Cho tới khi chốt (mục 9 Q2), frontend coi hai kênh này có thể trả `REQUEST_AI_COMPLETE_TIMEOUT` và phải chờ sự kiện hoặc polling `request.get`.
- `request.startPhase`: `phaseTaskId` rỗng chỉ hợp lệ khi Plan không có Phase (CR-REQ-013 mục 2.3).
- Cây Plan, Phase, Task đọc bằng kênh `task.getSubtree` hiện có với `rootTaskId = request.planTaskId`; không có kênh `plan.*`, `phase.*` (CR-REQ-016 D1).

### 2.2 `solution.*`

| Kênh | Tham số | Kết quả | Quyền | Timeout | RPC |
|---|---|---|---|---|---|
| `solution.list` | `{requestId, kind?, status?, pageSize?, pageToken?}` | `{solutions: SolutionView[], runs: AnalysisRunView[], nextPageToken}` | đọc | 8s | `ListSolutions` |
| `solution.generate` | `{requestId, idempotencyKey?, feedback? (≤2000), analysisMode?: 'complete'\|'agent_readonly'}` | `{solutionId, runId}` | ghi; chỉnh bằng `feedback`: xem dưới | 8s | `GenerateSolution` |
| `solution.choose` | `{requestId, solutionId, optionId, comment?}` | `{solution: SolutionView, approvalDigest}` | người duyệt `solution` | 8s | `ChooseSolutionOption` |

`solution.generate` **bất đồng bộ** (CR-REQ-007 mục 2.5): trả ngay `runId`; hoàn tất qua sự kiện `solution.proposed`, hoặc lỗi nằm ở `runs[].errorCode` (`REQUEST_SOLUTION_INVALID_OUTPUT`, `REQUEST_SOLUTION_RUN_INTERRUPTED`). Frontend không giữ yêu cầu chờ 25 giây. `solution.choose` trả `approvalDigest` mới; gửi lại ở `approval.approve.expectedDigest`.

**Chỉnh phân tích bằng phản hồi (CR-REQ-041).** `solution.generate` với `feedback` khác rỗng trên Request đang `awaiting_analysis_approval` yêu cầu một phiên bản mới của Solution, Diagnosis, Findings hoặc Answer (cổng `solution`, `findings`, `answer` tuỳ loại Request). Quyền do request-service quyết định, gateway và MCP không lọc thêm: người báo cáo, `admin|lead|owner`, và người duyệt của Approval `solution|findings|answer` đang chờ (theo user, team hoặc vai trò) đều gọi được; người khác nhận `PERMISSION_DENIED` (`REQUEST_SOLUTION_FORBIDDEN`). Người duyệt chỉ được chỉnh, không được khởi động phân tích đầu. Hệ quả cho client:
- Phiên bản đang chờ chuyển `superseded` và Approval của nó bị huỷ ngay khi lượt mới bắt đầu, nên nếu lượt mới thất bại thì Request ở `analyzing` và không còn đề xuất để duyệt. Lỗi nằm ở `runs[0].errorCode`. `solution.generate` không kèm `feedback` chạy lại lượt đó và **mang theo phản hồi của lượt vừa thất bại** (phản hồi không mất); gửi `feedback` mới thì phản hồi mới thay phản hồi cũ. Phiên bản `superseded` luôn nằm trong ngữ cảnh của lượt kế.
- Hai người chỉnh cùng lúc: người đến sau nhận `REQUEST_SOLUTION_WRONG_STATE` (lượt chỉnh khác đang chạy), không có lượt thứ hai; UI tải lại `solution.list`. Cùng `idempotencyKey` (bấm đúp, thử lại mạng) trả lại đúng `runId` đầu và không sinh lượt mới.
- `hotfix` không có cổng duyệt Analysis: Request không dừng ở `awaiting_analysis_approval` (Diagnosis tự duyệt), nên sau khi phân tích xong, `feedback` bị từ chối `REQUEST_SOLUTION_WRONG_STATE`.
- `revisionFeedback` và `seq` của `SolutionView` do `solution.list` trả; MCP `solution_list` trả cùng trường (`solution_generate` nhận `feedback` và `idempotency_key`). Nội dung phản hồi không có trong activity, log, span hay sự kiện (C9).

### 2.3 `approval.*`

| Kênh | Tham số | Kết quả | Quyền | Timeout | RPC |
|---|---|---|---|---|---|
| `approval.get` | `{id}` | `{approval: ApprovalView}` | đọc Request chứa nó | 8s | `GetApproval` |
| `approval.list` | `{requestId, subjectType?, status?, pageSize?, pageToken?}` | `{approvals: ApprovalView[], nextPageToken}` | đọc | 8s | `ListApprovals` |
| `approval.listPending` | `{subjectType?, pageSize?, pageToken?}` | `{approvals: ApprovalView[], nextPageToken}` | người dùng hiện tại | 8s | `ListPendingForUser` |
| `approval.approve` | `{id, expectedVersion, expectedDigest, comment?, executionMode?}` (mục 2.11) | `{approval: ApprovalView, requestStatus: RequestStatus, executionMode?}` | người duyệt (CR-REQ-009, 010) | 8s | `Approve` |
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

### 2.5 Kênh bổ sung (đã thật ở gateway, không nằm trong 28 kênh của CR-REQ-016)

README v6 mục 8 dòng 12 thêm RPC mà CR-016 chưa có kênh. Backend dự trù, frontend **chưa được dùng** tới khi mục 9 câu Q1 chốt:

| Kênh | Tham số | Kết quả | RPC | Lý do cần |
|---|---|---|---|---|
| `request.links` | `{id}` | `{parents: RequestLinkView[], children: RequestLinkView[]}` | `ListRequestLinks` (CR-REQ-006) | hiện Request cha, con ở trang chi tiết (CR-REQ-019 hỏi `request.get` có `links` không) |
| `request.flow` | `{type, size?}` | `{steps: [...], phases: boolean, gates: ApprovalSubjectType[]}` (hình dạng chốt ở CR-REQ-003 Q2) | `GetRequestFlow` | frontend không sao chép registry luồng |
| `request.checks` | `{id}` | `{checks: [...]}` | `ListRequestChecks` (CR-REQ-014) | số đo `performance`, `refactor` |

`RecordRequestCheck` không có kênh WS ở v1; chỉ đường MCP hoặc nội bộ (CR-REQ-014 Q1, CR-REQ-017 Q).

### 2.6 `clarification.*` (CR-REQ-028)

Sau khi xác nhận loại, request-service đưa Request vào `awaiting_information` nếu thiếu thông tin. Không có các kênh này thì không client nào trả lời được và Request đứng yên, nên chúng là một phần bắt buộc của luồng (BUG-E2E-001). Quyền: người được giao câu hỏi hoặc người có quyền trên Request; gateway chỉ ánh xạ hình dạng.

| Kênh | Tham số | Kết quả | RPC |
|---|---|---|---|
| `clarification.list` | `{requestId, status?, pageSize?, pageToken?}`; `status` ∈ `open`, `answered`, `expired`, `cancelled` | `{clarifications: ClarificationView[], nextPageToken}` | `ListClarifications` |
| `clarification.get` | `{clarificationId}` | `{clarification}` | `GetClarification` |
| `clarification.answer` | `{clarificationId, answers: [{questionId, valueJson, acceptDefault?}], complete?, expectedVersion?}` | `{clarification, requestStatus, requestRevision, stillMissing}` | `AnswerClarification` |
| `clarification.cancel` | `{clarificationId, reason?, expectedVersion?}` | `{clarification}` | `CancelClarification` |
| `clarification.request` | `{requestId, source, sourceRef?, questions: [{questionKey, kind, prompt, reason, options?, suggestedDefault?, required?, targetPath?}]}`; `source` ∈ `solution_open_question`, `plan_assumption`, `manual`; `kind` ∈ `text`, `single_choice`, `multi_choice`, `file`, `boolean` | `{clarification}` | `RequestClarification` |

`ClarificationView`: `{id, displayId, requestId, source, sourceRef?, status, resumeStatus?, round, questions: ClarificationQuestionView[], dueAt?, version, createdAt?}`. `ClarificationQuestionView`: `{id, seq, questionKey, kind, prompt, reason?, options?, suggestedDefault?, required, targetPath?, valueType?, answer?}`. `valueType` ∈ `string`, `number`, `boolean`, `list` suy từ cùng luật kiểm tra trường đích, để UI chọn ô nhập và cách mã hoá `valueJson` (ví dụ `time_box_hours` là `number`, `data_exposed` là `boolean`, `repro_steps` là `list`); vắng thì coi là `string`. `options`, `suggestedDefault` và `answer` là giá trị JSON đã giải mã, không phải chuỗi.

`valueJson` là **chuỗi JSON** (ví dụ `"\"yes\""`, `42`, `["a","b"]`). Khi `complete=true` mà còn câu bắt buộc chưa trả lời, kết quả có `stillMissing=true` và Request chưa rời `awaiting_information`. Lỗi riêng: `REQUEST_CLARIFICATION_*` (mục 5). Không thuộc công cụ MCP: đây là việc của người (`excluded_channels.yaml`, mẫu `clarification.*`).

#### Câu hỏi chặn của Solution (TASK-V2-023, CR-REQ-028 mục 2.5)

Solution có `open_questions[{id, text, blocking}]` (trong `optionsJson` của `solution.list`). Mục `blocking=true` chặn duyệt: `approval.approve` trả `REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1, Q-2` (FailedPrecondition; danh sách id sau dấu hai chấm cuối, cách nhau `, `; đây là hợp đồng, có test giữ nguyên). Đường trả lời, theo thứ tự:

1. Người báo cáo hoặc admin gọi `clarification.request {requestId, source:"solution_open_question", sourceRef:<solution.id>, questions:[{questionKey:"open_question:<Q-id>", kind:"text", prompt:<text, tối đa 1000 ký tự>, reason:<lý do hỏi, bắt buộc, tối đa 500>, required:true}]}`, mỗi câu hỏi của Solution một câu (`questionKey` đúng `open_question:` + `id` của câu; `boolean` cho câu có/không). Hiệu ứng: approval đang chờ bị huỷ (`information_required`), Request sang `awaiting_information`, một Clarification `open` với `resumeStatus:"analyzing"`. Một Request chỉ có một Clarification mở; gọi lặp cùng `source`, `sourceRef` và bộ `questionKey` trả lại chính Clarification đó.
2. `clarification.answer {clarificationId, answers:[{questionId, valueJson}], complete:true, expectedVersion}` (cùng kênh với Clarification sẵn sàng).
3. Trả lời xong, Request về `analyzing`, **Solution cũ bị `superseded`** (Decision cũng vậy) và backend tự sinh lại (`REQUEST_CLARIFICATION_AUTO_REGENERATE`, mặc định bật) với câu trả lời đã nằm trong đầu vào của AI. Solution mới có `id` mới; UI phải chọn lại phương án rồi duyệt. Câu hỏi chặn của Solution mới (nếu AI vẫn hỏi) cần Clarification mới với `sourceRef` là id mới. Chưa tự khởi động được thì UI gọi `solution.generate`.

Lỗi của `clarification.request`: `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED` (nguồn của nền tảng như `readiness`, `task_blocked`; Request ở trạng thái không hỏi được; đã có Clarification mở khác), `REQUEST_CLARIFICATION_INVALID_ANSWER` (câu hỏi sai: thiếu `reason`, `prompt` quá dài, trùng `questionKey`), `REQUEST_FORBIDDEN` (không phải người báo cáo hoặc admin).

### 2.7 `task.list` cho tab Plan (TASK-V2-022)

`task.list` nhận thêm các tham số tuỳ chọn, gateway chuyển nguyên xuống `TaskService.ListTasks` (proto đã có `task_types`, `request_ids`, `parent_id`; không đổi proto):

| Tham số | Kiểu | Ý nghĩa |
|---|---|---|
| `taskTypes` | `string[]` | lọc theo `task`, `bug`, `feature`, `epic`, `plan`, `phase`; **vắng thì task-service ẩn `plan`/`phase`** (tương thích client cũ), nên tab Plan phải gửi `["plan","phase", ...]` |
| `requestId` | `string` | một Request; gộp với `requestIds` nếu cả hai có (tối đa 100 id) |
| `requestIds` | `string[]` | nhiều Request |
| `parentId` | `string` | chỉ con trực tiếp của task đó |

Kết quả giữ `{tasks, nextPageToken}`; mỗi task thêm `requestId` (vắng nếu task không thuộc Request). Phạm vi vẫn là tenant của người gọi (task-service không lọc theo grant ở `ListTasks`); Plan và Phase do `CreatePlanTree` cấp quyền owner (`ApplyTree`) cho người tạo Request.

### 2.8 `execution.get` (TASK-V2-025)

Tab Result của sheet task (`useExecutionResult`). Gateway gọi `ListExecutionRecords(task_ids=[taskId], latest_only)` rồi, nếu task không có bản ghi, `GetTaskExecutionOutput(task_id)`; task-service kiểm quyền đọc task ở cả hai lời gọi.

| Kênh | Tham số | Kết quả |
|---|---|---|
| `execution.get` | `{taskId, latestOnly?}` | `ExecutionResultView` |

```
interface ExecutionResultView {          // camelCase; parseExecutionResult của frontend đọc đúng hình này
  taskId: string
  attempt?: number                       // vắng khi source = "last_execution_output"
  parseStatus: 'ok' | 'missing' | 'invalid'
  status?: 'done' | 'blocked' | 'failed' | 'needs_info'
  summary?: string                       // đã che bí mật
  filesChanged: string[]                 // file agent thấy đổi trong worktree trước, rồi file agent tự khai
  checksRun: { id: string; exit: number }[]
  failureClass?: string
  stdoutTail?: string                    // đã che bí mật, tối đa 8 KiB cuối
  outputs?: Record<string, unknown>      // đã che bí mật trong mọi chuỗi
  source: 'execution_record' | 'last_execution_output'
  redacted?: boolean; truncated?: boolean
}
```

- `source = "execution_record"`: lượt chạy hợp đồng (CR-REQ-029) có khối kết quả. `verdict` không bao giờ có ở kênh này (là việc của bước kiểm của request-service).
- `source = "last_execution_output"`: lượt chạy thành công không có khối kết quả (mọi task của Request hiện nay): `parseStatus = "missing"`, `stdoutTail` là stdout của lượt chạy cuối thành công.
- Chưa có gì để hiện: lỗi `TASK_NOT_FOUND: ...` (UI đọc là `legacy`). Không đủ quyền đọc task: `TASK_FORBIDDEN: ...`. Lỗi khác: `TASK_UNAVAILABLE: ...` (không lộ nội dung lỗi của service).
- Lượt chạy lỗi không để lại output ở đây (task-service chỉ lưu stdout của lượt thành công); lỗi gần nhất của task nằm ở `backlog.tasks` (`lastError`).

### 2.9 `devServer.list` thêm trạng thái bận, và `devServer.subscribeStatus` (TASK-V2-025)

`devServer.list` và `devServer.listForUser` thêm `activeExecutions: number` và `busy: boolean` vào mỗi dev server (nguồn: số lượt `agent.execPrompt` và `ai.complete` đang chờ trả lời trên phiên sống của dev server; luôn `0`/`false` khi `status != "connected"`). `platform`, `arch`, `nodeVersion` nay lấy từ handshake của agent (null khi chưa biết), `healthStatus` thành `healthy` sau lần handshake đầu.

`devServer.subscribeStatus` (kênh push, không có tham số) đẩy `devServer.statusChanged` với `{devServerId, connected, activeExecutions, busy}` khi dev server nối, rớt, hoặc bắt đầu/kết thúc một lượt chạy. Là ảnh chụp trạng thái (không phải thay đổi) và tạm thời: client nên gọi `devServer.list` một lần khi mở, rồi để sự kiện ghi đè.

### 2.10 Nhật ký hoạt động, tóm tắt thực thi, engine và chế độ thực thi (CR-REQ-038)

Tên tham số của kênh là `id` (không phải `requestId`), giống các kênh `request.*` khác. Cả hai kênh đọc, `GroupRead`, agent được phép gọi (OPA `agent_rpcs`), có tương đương MCP (`request_activity`, `request_executionSummary`).

| Kênh | Tham số | Kết quả | RPC |
|---|---|---|---|
| `request.activity` | `{id, pageSize?, pageToken?, kinds?: string[], minSeverity?: info\|attention\|error, actorKind?: user\|ai\|agent\|system, subjectId?, since?}` | `{entries: ActivityEntryView[], nextPageToken?}`; mới nhất trước, khoá phân trang `(occurred_at, id)` | `ListRequestActivity` |
| `request.executionSummary` | `{id}` | `{summary: ExecutionSummaryView}` | `GetExecutionSummary` |

`ActivityEntryView = {id, kind, occurredAt, actorKind, actorId?, subjectType?, subjectId?, summary, attempt?, severity, details}`. `kind` là chuỗi mở (`request.created`, `request.type_changed`, `analysis.generated`, `analysis.revision_requested`, `clarification.asked|answered`, `approval.requested|decided`, `phase.started`, `task.started|succeeded|failed|accepted|cancelled`, `request.returned|reopened|cancelled`…); frontend phải chịu được `kind` lạ. `attempt` là số lần chạy của task (cửa sổ `SUM` trên `task_run_outcomes`).

`analysis.revision_requested` (CR-REQ-041): mỗi lượt phân tích có phản hồi của người duyệt để lại đúng một dòng, `id = <runId>:revision`, `occurredAt = started_at` của lượt, `actorKind = user`, `actorId` là người chỉnh (rỗng nếu lượt chưa ghi), `subjectType = solution`, `subjectId` là Solution của lượt (rỗng nếu chưa có), `severity = info`, `summary = "Requested changes to the analysis"`, `details = {solutionId}`. Dòng **không bao giờ chứa nội dung phản hồi** (C9): lấy nội dung qua `solution.list` (`revisionFeedback`), không qua activity. `kinds` nhận giá trị mới như các kind khác; hai dialect cho cùng kết quả.

`ExecutionSummaryView = {requestId, status, engine?, executionMode?, progress{phasesTotal, phasesDone, tasksTotal, tasksDone, tasksFailed, tasksRunning}, currentPhaseId?, nextPhaseId?, running[{taskId, phaseId?, title, attempt, maxAttempts, startedAt?}], failed[{taskId, phaseId?, title, attempts, maxAttempts, lastCause, lastError?, lastAt?}], needsYou[{kind: approval|clarification|failed_task|pre_deploy|drift, refId, subjectType?, title?, phaseId?, since?, dueAt?}], handover{state: none|archiving|ready|failed, archiveAttempts, summaryRef?}, maxAttempts, startedAt?}`. `needsYou` chỉ liệt kê việc người gọi có quyền làm. `lastCause` ∈ `timed_out|model_error|verify_failed|env_defect|spec_defect|agent_error|needs_info|cancelled|unknown`.

Trường thêm vào kênh có sẵn (cộng, không đổi nghĩa):

- `request.get`/`request.list`: `engine` (`native|openspec`, rỗng khi không xác định được) và `executionMode` (`step_by_step|continuous`: giá trị đã ghim khi vào `executing`, trước đó là chế độ sẽ áp dụng theo cài đặt dự án).
- `request.flow`: `openSpecProfile` (`full|light|none`).
- `request.engineSettingsGet|Set`: `executionMode` (rỗng khi `Set` giữ giá trị đã lưu). Biến môi trường mặc định của triển khai: `REQUEST_DEFAULT_EXECUTION_MODE`.
- Lỗi mới: `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` (duyệt Plan bị từ chối vì người duyệt không có quyền chạy thực thi).

### 2.11 Nhánh gốc của worktree (CR-REQ-039)

Một Request có thể chọn nhánh hoặc tag mà mọi worktree của nó (worktree đề xuất OpenSpec và worktree của Task) bắt đầu từ đó. Rỗng nghĩa là nhánh mặc định của repo. Tên tham số của kênh là `id`.

**Kênh mới `request.setBaseRef {id, baseRef, expectedVersion?}`** (RPC `SetRequestBaseRef`, nhóm `triage`, tức chủ Request hoặc người báo; `expectedVersion` 0 hoặc bỏ qua thì không kiểm phiên bản). Trả `{request: RequestView}` với `baseRef` mới, `baseRefLocked`, `baseRefLockReason`, `baseRefUnverified`.
- `baseRef` là **bắt buộc có khoá**: thiếu khoá, `null` hay không phải chuỗi thì gateway trả `INVALID_ARGUMENT` mà không gọi service (gõ nhầm tên khoá không được lặng lẽ đặt lại về mặc định). Gửi `""` để quay về nhánh mặc định.
- Chỉ được đổi khi Request chưa có worktree. Sau đó trả `REQUEST_BASE_REF_LOCKED`.
- **Không có tool MCP.** Agent không đổi được nền của worktree mình sẽ chạy: RPC có `AgentAllowed=false` và không nằm trong `agent_rpcs` của `request.rego`, kênh nằm trong `excluded_channels.yaml` (`human_gate`). Agent chỉ đặt được `base_ref` một lần, ở `request_create`.

**`request.create`** nhận thêm `baseRef?` (≤255 byte; rỗng hoặc bỏ qua giữ hành vi cũ, nên frontend cũ chạy như trước) và trả `request.baseRef`, `baseRefLocked` (luôn `false`), `baseRefLockReason` (`""`), `baseRefUnverified`.

**Trường trên `RequestView`** (cộng, không đổi nghĩa):
- `baseRef: string` có mặt ở **mọi** `RequestView`, kể cả `""` (không `omitempty`): frontend phân biệt gateway cũ (thiếu khoá) với gateway mới (khoá có, giá trị rỗng) bằng sự có mặt của khoá.
- `baseRefLocked: boolean` và `baseRefLockReason: "" | "proposal" | "task" | "execution" | "terminal"` có mặt ở `request.get`, `request.list`, `request.create` và `request.setBaseRef` (luôn có, `false` và `""` khi chưa khoá). Các kênh khác (`changeType`, `cancel`, `classify`…) **bỏ hai khoá này**, vì chúng không tra cứu worktree: `false` ở đó có thể sai. Frontend giữ giá trị lần đọc trước hoặc gọi `request.get`.
- `baseRefUnverified: boolean` chỉ có ở `request.create` và `request.setBaseRef`.
- Lý do khoá: `proposal` (có worktree đề xuất OpenSpec, kể cả đang `preparing`), `task` (một Task của Request có worktree), `execution` (một lần thực thi đã bắt đầu dùng ref; mã do request-service thêm cùng đợt, gateway chuyển nguyên), `terminal` (Request `completed` hoặc `cancelled`). Gateway không kiểm tập giá trị này: frontend phải chịu được lý do lạ (coi là đã khoá).

**Mã lỗi** (đều do `request-service` cấp, gateway chuyển nguyên; thông báo là câu cho người đọc). Với hai mã sau, gateway thêm đuôi JSON sau thông báo, cùng dạng `{"retryAfterMs":N}` của `REQUEST_RATE_LIMITED`:

| Mã | gRPC | Khi nào | Đuôi JSON (`details`) |
|---|---|---|---|
| `REQUEST_BASE_REF_INVALID` | InvalidArgument | tên sai cú pháp git (khoảng trắng, ký tự cấm, `..`, `@{`, kết thúc `.lock`, quá 255 byte, chuỗi chỉ gồm khoảng trắng…); thông báo nêu quy tắc, không lặp lại giá trị | không có |
| `REQUEST_BASE_REF_NOT_FOUND` | NotFound | git trả lời rằng ref không tồn tại trong repo | `{"suggestions":["main",…]}`, tối đa 5, có thể vắng; gateway không lặp lại `ref` vì client đã gửi |
| `REQUEST_BASE_REF_LOCKED` | FailedPrecondition | Request đã có worktree hoặc đã ở trạng thái cuối | `{"reason":"proposal\|task\|execution\|terminal"}` |

Lỗi chung của `setBaseRef`: `REQUEST_VERSION_CONFLICT` (phiên bản cũ, tải lại), `REQUEST_NOT_FOUND`, `REQUEST_FORBIDDEN` (không thuộc nhóm triage) hoặc `REQUEST_ACTOR_NOT_ALLOWED` (agent), `REQUEST_BASE_REF_CHECK_NOT_CONFIGURED` (triển khai thiếu bộ kiểm ref; đây là lỗi cấu hình, ref không được nhận mà chưa kiểm), `INVALID_ARGUMENT` (do gateway).

**Kiểm ref fail-open.** Khi `baseRef` không rỗng, `request-service` hỏi git trên dev server xem ref có tồn tại không. Chỉ câu trả lời "không có" mới bị từ chối (`REQUEST_BASE_REF_NOT_FOUND`). Nếu dev server không với tới được hoặc git báo lỗi chung thì ref **vẫn được lưu** và kết quả mang `baseRefUnverified: true`: tạo Request không bị chặn chỉ vì agent offline. Worktree sẽ được tạo từ ref đó sau; nếu khi đó ref vẫn không có thì lỗi xuất hiện lúc tạo worktree. UI nên hiện cảnh báo nhẹ khi `baseRefUnverified`. Không có kiểm tra thì `baseRefUnverified` là `false` (ref rỗng).

**Cờ tính năng `request.flowStatus.features`.** `request.flowStatus` trả thêm `features: string[]`, do **gateway** công bố (không phải `request-service`), hiện là `["request.baseRef","approval.runAll","request.planAuto","request.planRevisions"]`. Frontend chỉ hiện ô nhánh gốc khi mảng có `request.baseRef` và chỉ mời "Approve and run all steps" khi có `approval.runAll`; gateway cũ không trả `features` (và `RequestView` thiếu khoá `baseRef`) nên ô ẩn. Tên chỉ được thêm, không bỏ. `features` có mặt kể cả khi `enabled=false`.

**`approval.approve` nhận `executionMode` (CR-REQ-043).** Rỗng hoặc vắng = duyệt như cũ; giá trị duy nhất khác rỗng là `continuous` (`step_by_step` bị từ chối). Chỉ hợp lệ khi duyệt `plan`, `task_list` hoặc `pre_deploy` ở `awaiting_plan_approval`. Gateway chuyển nguyên văn, `request-service` kiểm. `request-service` ghim chế độ trong cùng transaction với quyết định, nên quyết định lỗi thì chế độ không được ghim; kết quả `executionMode` là chế độ đang ghim trên Request sau quyết định (cũng trả khi gọi lại một quyết định đã xong), vắng khi chưa ghim. Request đã ghim `step_by_step` không đổi được: duyệt kèm `continuous` trả `REQUEST_EXECUTION_MODE_ALREADY_PINNED` và không ghi quyết định. `approval.reject` kèm `executionMode` khác rỗng trả `REQUEST_EXECUTION_MODE_INVALID: executionMode applies to approve only` ngay ở gateway (REST 400). Gateway cũ bỏ qua khoá lạ nên không được gửi khi `features` thiếu `approval.runAll`. Payload `approval.decided` có `execution_mode` (chỉ khi người duyệt đã chọn) và audit `approval.approve` có `metadata.execution_mode`; khung `request.event` không chuyển khoá này. MCP không có công cụ này (agent không duyệt).

### 2.12 Danh sách nhánh của repo cho ô "Start from" (CR-REQ-039, CR-REQ-040 sửa đổi 2026-10-20)

Ô "Start from" mở ra **danh sách nhánh** của repo ngay khi bấm, không bắt người dùng gõ trước. Kênh chỉ đọc, theo repo (không theo worktree):

**`repo.listBranches {repoId}`** (RPC git-gateway `ListRepoBranches`) trả `{branches: [{name, remote}], truncated}`.
- `name` là tên ngắn của ref: nhánh local là `main`, `release/1.x`; nhánh remote là `origin/hotfix/x` (đủ tên remote, dùng được ngay làm `baseRef`).
- `remote` là `true` chỉ cho nhánh remote **không có bản local cùng tên** (sau khi bỏ tiền tố remote); nhánh remote có bản local bị ẩn để không lặp. `<remote>/HEAD` không bao giờ có mặt.
- Thứ tự: nhánh local theo tên (so byte), rồi nhánh remote theo tên. Tối đa 500 mục; vượt thì cắt và `truncated: true` (frontend vẫn cho gõ tìm bằng `repo.searchRefs`).
- Không chứa tag. Tag vẫn chọn được bằng tìm kiếm hoặc nhập tay.
- Lỗi: repo không có `WORKTREE_REPO_NOT_FOUND`; host không tới được `Unavailable` (cùng mã với `repo.searchRefs`). Kênh không có ở gateway cũ: frontend rơi về picker tìm kiếm, không báo lỗi.
- Không có tool MCP (chỉ phục vụ màn hình); chạy được qua SSH vì dùng `git branch --list --all --format` (git 2.13 trở lên, nằm trong allowlist `git.exec`).

**Lưu ý dạng JSON:** gateway bỏ khoá `remote` khi giá trị là `false` (không `omitempty` ở phía client), nên vắng mặt nghĩa là nhánh local; và `repo.baseRefDefault` trả `{ref}` (không phải `defaultBaseRef`), frontend web ánh xạ hai tên này trong `web/base-ref-default-wire.ts`.

**Nhánh mặc định** vẫn lấy từ `repo.baseRefDefault` (đã sửa ở BE-REQ-TASK-039-04); frontend ghim nó lên đầu danh sách.

### 2.13 Tự sinh Plan sau khi duyệt Analysis (mới, CR-REQ-044)

Khi một Request vào `planning` sau khi Analysis được duyệt (hoặc sau khi xác nhận loại, với loại không có Analysis), `request-service` tự bắt đầu run sinh Plan, tự commit proposal khi run thành công và tự thử lại có giới hạn lỗi tạm thời. Gateway chỉ chuyển ba thứ ra giao diện. Cờ `request.planAuto` trong `request.flowStatus.features` báo gateway có đủ; vắng thì giao diện giữ nút Generate thủ công.

| Kênh | Tham số | Kết quả | RPC |
|---|---|---|---|
| `request.retryPlan` (mới) | `{id}` | `{autoPlan: AutoPlanView}` | `RetryPlanGeneration` |
| `request.planProposal` | không đổi | thêm khoá `autoPlan: AutoPlanView` khi service gửi khối này | `GetPlanProposal` |
| `request.engineSettings`, `request.engineSettingsSet` | `engineSettingsSet` nhận thêm `planAutoGenerate?: boolean` | `planAutoGenerate: boolean` trong kết quả | `Get/SetProjectEngineSettings` |

`AutoPlanView = {enabled: boolean, state: AutoPlanState, origin: "event"|"retry"|"revision"|"", attempt: number, maxAttempts: number, reasonCode: string, nextAttemptAt: string}` (camelCase, luôn đủ khoá; `nextAttemptAt` là RFC 3339, rỗng khi không áp dụng; `origin = "revision"` là job của lượt chỉnh Plan, CR-REQ-045).

| `state` | Nghĩa | `reasonCode` |
|---|---|---|
| `""` | không có job (hoặc job đã xong của vòng trước, nay Request lại `planning` với Plan cũ) | rỗng |
| `pending` | chờ bắt đầu hoặc chờ lượt thử lại; `nextAttemptAt` điền khi chờ sau lỗi hoặc hoãn vì bận | mã lỗi của lượt trước, hoặc `REQUEST_ANALYSIS_BUSY` |
| `running` | run đang chạy | rỗng |
| `committing` | run xong, đang ghi Plan | rỗng |
| `done` | Plan đã được ghi (cũng đọc được sau khi Request sang `awaiting_plan_approval`) | rỗng |
| `gave_up` | hết lượt hoặc lỗi không thể tự thử lại; giao diện hiện thẻ lỗi và nút Retry | mã lỗi (`REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_PLAN_COMMIT_FAILED`...) |
| `skipped` | không bắt đầu được tự động (không có dev server, thiếu đường dẫn repo, engine không có, bị tắt giữa chừng); nút Generate thủ công vẫn dùng | mã lý do |
| `superseded` | người đã làm trước (Request rời `planning`, đã có Plan, hoặc có run mới hơn) | `REQUEST_PLAN_REQUEST_MOVED`, `REQUEST_PLAN_EXISTS`, `REQUEST_PLAN_NEWER_RUN` |

- `enabled` luôn được điền (cả khi chưa có job): công tắc triển khai `REQUEST_PLAN_AUTO_GENERATE` **và** công tắc dự án `planAutoGenerate`. `false` nghĩa là giao diện dùng luồng thủ công.
- `request.planProposal` khi Request chưa có run Plan nào và server có tính năng này: trả view rỗng (`runId:""`, `proposal:null`) **kèm `autoPlan`** thay vì `REQUEST_PLAN_RUN_NOT_FOUND`. Server không có tính năng giữ lỗi cũ.
- `request.retryPlan` yêu cầu Request `planning`; có job hoạt động thì trả chính nó (bấm hai lần không sinh thêm); run Plan mới nhất đã thành công mà chưa commit thì tiếp tục từ bước commit, không tốn AI lần nữa. Quyền như `request.generatePlan` (người báo cáo, owner, admin). Lỗi: `REQUEST_PLAN_AUTO_DISABLED` (công tắc triển khai tắt hoặc không có `CommitPlan`; giao diện rơi về luồng thủ công), `REQUEST_STATE_STALE`, `REQUEST_PLAN_FORBIDDEN`. Không có tool MCP (agent đi qua `request_generatePlan`).
- `planAutoGenerate` trong `engineSettingsSet`: thiếu khoá = giữ giá trị đã lưu (`true` nếu dự án chưa có dòng); sai kiểu bị từ chối `INVALID_ARGUMENT`. Chỉ admin. Giao diện cho công tắc này chưa có (chỉ kênh này và `orca engine set --plan-auto-generate=<true|false>`).
- Mã `REQUEST_PLAN_COMMIT_FAILED` chỉ xuất hiện làm `reasonCode`, không là lỗi RPC. Mã lỗi run mới `REQUEST_PLAN_AUTO_DISABLED` (FailedPrecondition) chỉ do `request.retryPlan` trả.

### 2.14 Phiên bản của Plan và chỉnh Plan bằng prompt (mới, CR-REQ-045)

Mỗi run Plan thành công là một phiên bản, đánh số từ 1 theo thứ tự (cả engine `native` lẫn `openspec`). Phiên bản mới nhất là bản hiện hành (`status: "current"`), các bản còn lại `superseded`. Chỉ bản hiện hành mới có `checks` và `knowledge`. Cờ `request.planRevisions` trong `request.flowStatus.features` báo gateway hiểu tham số `revision`; gateway cũ bỏ qua khoá lạ nên giao diện không được gửi `revision` khi thiếu cờ. Tài liệu chỉ để xem; chỉnh Plan là gửi phản hồi bằng chữ qua `request.generatePlan`.

| Kênh | Tham số | Kết quả | RPC |
|---|---|---|---|
| `request.planRevisions` (mới, nhóm đọc, 8s) | `{id, pageSize?, pageToken?}` (mặc định 20, tối đa 100) | `{revisions: PlanRevision[], nextPageToken}` (mới nhất trước) | `ListPlanRevisions` |
| `request.planProposal` | thêm `revision?: number >= 0` | `bundle.files[]` thêm `sha256`, `change` | `GetPlanProposal` |
| `request.planBundleFile` | thêm `revision?: number >= 0` | không đổi `{path, content, omitted}` | `GetPlanBundleFile` |
| `request.generatePlan` (propose) | thêm `idempotencyKey?: string` (tối đa 128 ký tự) | không đổi | `GeneratePlan` |

`PlanRevision = {revision, runId, status: "current"|"superseded", engine: "native"|"openspec", revisionFeedback, actorId, generatedAt, commit, filesChanged, fileCount, textKept, stats}` (camelCase, luôn đủ khoá; `stats` là `RunStats` hoặc `null`).

- `revision` vắng hoặc 0: hành vi cũ (theo `runId`, hoặc bản mới nhất). `revision > 0` thắng `runId`. `revision < 0` hoặc không phải số: `INVALID_ARGUMENT` ngay ở gateway.
- `revisionFeedback` là chữ người yêu cầu chỉnh; rỗng với bản đầu, với lần thử lại không chữ và với câu trả lời một Clarification (`clarification:`).
- `filesChanged` đếm file `added`, `changed`, `removed` so với bản liền trước (bản 1 và bản không có chỉ mục bản trước: bằng `fileCount`). `fileCount` và `commit` chỉ có với engine `openspec`.
- `textKept=false` (luôn được gửi tường minh): chữ của bản đã bị lược, chỉ còn danh sách file. Chỉ 10 bản mới nhất giữ chữ. Request có Plan từ trước CR này chỉ có tài liệu cho bản hiện hành; các bản cũ hơn là `textKept=false`, `fileCount=0`.
- `bundle.files[].change` là `added|changed|unchanged|removed` so với bản liền trước, rỗng khi không có gì để so (bản 1); file `removed` có trong danh sách (kèm kích thước cũ) nhưng không đọc được ở bản này. `sha256` là hash của chữ đã lưu (bí mật đã che); với file `omitted` là hash của chính file.
- `idempotencyKey`: gửi lại cùng khoá trả đúng run đã tạo, không tạo run thứ hai. Giao diện sinh khoá mới mỗi lần bấm: khoá gắn mãi với run đầu tiên dùng nó, kể cả khi run đó đã thất bại.
- Người chỉnh Plan đang chờ duyệt là người báo cáo, owner/admin hoặc approver của cổng `plan`, `task_list`, `pre_deploy` (kể cả khi họ chỉ là member của dự án). Chỉ khi `request-service` chạy worker tự sinh Plan (mục 2.13) thì approver mới chỉnh được, vì worker commit Plan thay họ; tắt worker thì approver nhận `REQUEST_PLAN_FORBIDDEN` và người báo cáo tự commit như cũ. Lượt chỉnh tạo một job `autoPlan.origin = "revision"` (`maxAttempts = 1`).
- REST (cùng kênh): `GET /v1/requests/{id}/plan/revisions?pageSize&pageToken`, và tham số query `revision` cho `GET /v1/requests/{id}/plan`, `.../plan/bundle`, `.../plan/bundle/file`. CLI: `orca plan revisions <id>`, `orca plan bundle <id> --revision N [--file ...]`.
- Không có tool MCP đọc hay ghi (`request.planRevisions` có dòng loại trừ trong `excluded_channels.yaml`); `request.planProposal` và `request.planBundleFile` cũng chưa có.

| Mã lỗi | Kiểu | Khi nào |
|---|---|---|
| `REQUEST_PLAN_REVISION_IN_PROGRESS` | FailedPrecondition | đã có yêu cầu chỉnh khác của Plan này đang chạy (khoá khác); cũng trả trong vài giây sau khi lượt chỉnh trước thất bại, tới khi worker ghi nhận |
| `REQUEST_PLAN_REVISION_NOT_APPLICABLE` | FailedPrecondition | chỉnh Plan của loại `hotfix` (chỉ có một task sửa từ chẩn đoán) |
| `REQUEST_PLAN_REVISION_NO_CHANGE` | (nằm ở `errorCode` của run, không ném) | agent không đổi tài liệu nào; Request ở `planning`, người dùng sửa chữ rồi gửi lại |
| `REQUEST_PLAN_REVISION_NOT_FOUND` | NotFound | `revision` lớn hơn số bản |
| `REQUEST_PLAN_REVISION_TEXT_PRUNED` | FailedPrecondition | đọc file của bản đã bị lược chữ |
| `REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID` | InvalidArgument | `pageToken` không do danh sách này trả |
| `REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID` | InvalidArgument | `idempotencyKey` dài hơn 128 ký tự |

REST ánh xạ: `*_NOT_FOUND` 404; `REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID` và `REQUEST_PLAN_IDEMPOTENCY_KEY_INVALID` 400; các mã còn lại 412.

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
| `request.base_ref_changed` | `orca.request.request.base_ref_changed` | làm mới Request (nhánh bắt đầu, khóa); khung không kèm tên nhánh (CR-REQ-039) |
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
| `POST /v1/approvals/{id}/approve`, `.../reject` | `approval.approve`, `approval.reject` | body `{version, digest, comment, executionMode?}` (CR-016 chỉ ghi `{version, comment}`, thiếu `digest`; xem mục 8). `executionMode` chỉ hợp lệ ở `approve` (CR-REQ-043) |

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
| `REQUEST_SOLUTION_BLOCKING_QUESTIONS` | FailedPrecondition | `approval.approve` khi Solution còn `open_questions` `blocking` chưa trả lời; danh sách id nằm trong thông báo | hiện các câu hỏi chặn và nút hỏi (mục 2.6, `clarification.request`); không phải "đã được quyết định" |
| `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS` | FailedPrecondition | `approval.approve` của Plan khi còn `assumptions` `needs_confirmation` chưa trả lời | hỏi bằng `clarification.request` với `source:"plan_assumption"` (đường này của Plan chưa được kiểm trong đợt TASK-V2-023) |
| `REQUEST_ANALYSIS_NO_REPO_PATH` | FailedPrecondition | `solution.generate` (loại chỉ đọc: bug, hotfix, security, performance, spike, question) khi repo mặc định của project không có đường dẫn thư mục trên dev server (URL là địa chỉ từ xa hoặc đường dẫn tương đối) | hướng dẫn gắn repo vào một thư mục trên dev server; không tự hạ xuống chế độ `complete` |
| `REQUEST_SOLUTION_NO_CONNECTION`, `REQUEST_AI_NO_PROVIDER`, `REQUEST_ANALYSIS_NO_CONNECTION`, `REQUEST_ANALYSIS_BUSY` | FailedPrecondition | project chưa có dev server, chưa có AI | hướng dẫn kết nối dev server |
| `REQUEST_SOLUTION_INVALID_OUTPUT`, `REQUEST_SOLUTION_RUN_INTERRUPTED`, `REQUEST_ANALYSIS_*`, `REQUEST_HOTFIX_FIX_TASK_FAILED`, `REQUEST_HOTFIX_APPROVAL_FAILED` | (nằm ở `runs[].errorCode`, không ném) | run sinh Solution lỗi | nút "Sinh lại" |
| `REQUEST_PLAN_NOT_APPLICABLE`, `REQUEST_PLAN_SOLUTION_NOT_APPROVED`, `REQUEST_PLAN_AI_UNAVAILABLE`, `REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_PLAN_*` (kiểm đề xuất, CR-REQ-012 mục 2.4), `REQUEST_AI_COMPLETE_TIMEOUT` | FailedPrecondition, InvalidArgument | `request.generatePlan` | hiện lỗi cụ thể; `REQUEST_PLAN_AI_INVALID_JSON` kèm `raw` để người xem |
| `REQUEST_PHASE_NOT_APPROVED`, `REQUEST_PHASE_PREDECESSOR_NOT_DONE`, `REQUEST_PHASE_NOT_IN_PLAN`, `REQUEST_NOT_EXECUTING`, `REQUEST_EXECUTE_FORBIDDEN` | FailedPrecondition, PermissionDenied | `request.startPhase` | toast |
| `REQUEST_APPROVAL_ALREADY_DECIDED`, `REQUEST_APPROVAL_EXPIRED`, `REQUEST_APPROVAL_DIGEST_MISMATCH`, `REQUEST_APPROVAL_STAGE_MISMATCH` | FailedPrecondition | duyệt lặp, quá hạn, nội dung đã đổi | tải lại cổng |
| `REQUEST_APPROVAL_NOT_APPROVER`, `REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN`, `REQUEST_APPROVAL_AGENT_FORBIDDEN`, `REQUEST_APPROVAL_FORBIDDEN`, `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER` | PermissionDenied, FailedPrecondition | quyền duyệt (CR-REQ-010) | toast, ẩn nút, tải lại |
| `REQUEST_BACKLOG_INVALID_VIEW`, `REQUEST_BACKLOG_BAD_PAGE_TOKEN`, `REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE` | InvalidArgument, Unavailable | `backlog.*` | thử lại |
| `REQUEST_BASE_REF_INVALID`, `REQUEST_BASE_REF_NOT_FOUND`, `REQUEST_BASE_REF_LOCKED` | InvalidArgument, NotFound, FailedPrecondition | `request.create` và `request.setBaseRef` (CR-REQ-039, mục 2.11) | lỗi cạnh ô nhánh gốc; `NOT_FOUND` gợi ý nhánh gần đúng; `LOCKED` khoá ô |
| `REQUEST_EXECUTION_MODE_INVALID`, `REQUEST_EXECUTION_MODE_NOT_APPLICABLE`, `REQUEST_EXECUTION_MODE_ALREADY_PINNED`, `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED` | InvalidArgument, FailedPrecondition | `approval.approve` kèm `executionMode` (CR-REQ-043): giá trị lạ hoặc kèm `reject`; không phải Approval khởi đầu run; Request đã ghim `step_by_step`; người duyệt không có quyền chạy | toast với thông điệp của service; không ghi quyết định |
| `REQUEST_PLAN_AUTO_DISABLED` | FailedPrecondition | `request.retryPlan` khi tự sinh Plan tắt ở triển khai (mới, CR-REQ-044, mục 2.13) | dùng luồng Generate thủ công |
| `REQUEST_FLOW_DISABLED` | FailedPrecondition | cờ tắt | ẩn màn hình Request |
| `REQUEST_RATE_LIMITED`, `REQUEST_PENDING_LIMIT` | ResourceExhausted | chống spam (chủ yếu nguồn MCP), giới hạn theo lớp đọc/ghi/AI/webhook (`REQUEST_RATE_<LỚP>_PER_SECOND`, `_BURST`) | chờ đúng `retryAfterMs` rồi thử lại; thông báo có đuôi JSON `{"retryAfterMs":N}` khi service gửi `RetryInfo` |
| `REQUEST_CLARIFICATION_NOT_FOUND` | NotFound | id Clarification sai hoặc khác tenant | làm mới danh sách |
| `REQUEST_CLARIFICATION_NOT_OPEN`, `REQUEST_CLARIFICATION_ALREADY_ANSWERED`, `REQUEST_CLARIFICATION_EXPIRED`, `REQUEST_CLARIFICATION_STATE_NOT_ALLOWED`, `REQUEST_CLARIFICATION_VERSION_CONFLICT` | FailedPrecondition | trả lời hoặc huỷ khi không còn mở, hoặc `expectedVersion` cũ | tải lại Clarification |
| `REQUEST_CLARIFICATION_INVALID_ANSWER`, `REQUEST_CLARIFICATION_INCOMPLETE`, `REQUEST_CLARIFICATION_NOT_ASSIGNEE` | InvalidArgument, PermissionDenied | giá trị sai kiểu, thiếu câu bắt buộc, người trả lời không được giao | lỗi cạnh trường |
| `REQUEST_PROJECT_NOT_FOUND` | NotFound | `request.create` với project không tồn tại hoặc khác tenant | báo project không hợp lệ |
| `REQUEST_AI_NOT_CONFIGURED`, `REQUEST_AI_AGENT_UNAVAILABLE`, `REQUEST_AI_SESSION_EXPIRED`, `REQUEST_AI_QUOTA_EXHAUSTED`, `REQUEST_AI_BUSY`, `REQUEST_AI_COMPLETE_TIMEOUT` | FailedPrecondition, Unavailable, ResourceExhausted | dev server chưa có AI (khoá API hoặc phiên đăng nhập Claude Code), agent mất kết nối, phiên hết hạn, hết hạn mức, đang bận, quá giờ | banner hướng dẫn cấu hình AI; nút thử lại với `REQUEST_AI_BUSY` và `REQUEST_AI_QUOTA_EXHAUSTED` |
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
