# BL-TG-05 — Task → Spec → Approve → Code → Task (vòng lặp khép kín)

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-TG-05 |
| **Tên** | Task-driven Spec/Approve/Build closed loop |
| **Domain** | Task Graph (mở rộng BL-TG-04) |
| **Actor** | Developer, Lead (người duyệt spec) |
| **Priority** | P1 |
| **Trạng thái** | ✅ Implemented + unit-tested (17 test mới trong `TaskPromptEditor.test.tsx` + 14 trong `task-spec-build-loop.test.ts`, tất cả pass) và **đã deploy** (`2026.09.16-bl-tg-05-spec-build-loop`). Nền tảng backend (BUG-023, BUG-024) đã fix + deploy ở lượt trước. Chờ user xác nhận lại trên `b15.openledger.vn` (bị chặn bởi vấn đề credential/AI Provider account đã báo riêng — xem "Các lỗ hổng"). |

---

## Đánh giá độ sẵn sàng — tự chấm 9.5/10

Thang điểm: 10 = một kỹ sư (hoặc agent) có thể implement thẳng từ tài liệu này, không cần hỏi lại
gì thêm. Lý do chấm 9.5, không phải 10:

| Đã chốt (giữ điểm) | Còn hở (trừ điểm) |
|---|---|
| Toàn bộ RPC shape (`task.execute`/`task.update`/`task.addComment`/`task.listComments`) đã **đọc code thật, verify field-name khớp cả 2 chiều request/response**, không phải suy đoán từ proto | Việc agent có **tuân thủ đúng đường dẫn file spec** (`specs/generated/TASK-{{n}}-spec.md`) mà prompt yêu cầu hay không **không có gì ép buộc phía server** — chỉ là chỉ dẫn trong prompt. Agent lệch hướng (viết sai chỗ/sai tên file) sẽ làm bước "Implement Spec" đọc nhầm/không thấy file. |
| Đã tìm và fix 2 bug nền tảng chặn cứng luồng này (BUG-023 raw snake_case response, BUG-024 sai shape request) — không phải giả định "chắc nó chạy được" | "Duyệt" (approve) chỉ là quy ước label + không bấm nút tiếp, không có gì chặn cứng bấm nhầm — **chấp nhận có chủ đích** cho v1 (xem Phương án B), không phải thiếu sót chưa nghĩ tới |
| Đã thêm field `labels` ghi được vào `task.update` (trước đây hoàn toàn không ghi được) — verify bằng test, không phải "chắc field đã có sẵn" | |
| Prompt template đưa ra nguyên văn, có chỗ interpolate rõ ràng (`{{taskNumber}}`/`{{title}}`/`{{description}}`), không để "viết prompt phù hợp" mơ hồ | |
| Đã xác định chính xác component frontend cần sửa (`TaskPromptEditor.tsx`, `TaskDetail.tsx`) và state hiện tại của chúng (đọc source thật) | |

Không tự chấm 10 vì file-path-compliance-by-prompt là rủi ro cố hữu của cách tiếp cận (không có
gì thực thi được ngoài chỉ dẫn ngôn ngữ tự nhiên) — mục "Rủi ro còn lại, chưa xử lý ở v1" bên dưới
nêu rõ hướng xử lý (v1.1) nếu rủi ro này thành vấn đề thật khi dùng.

---

## Mô tả

Yêu cầu gốc (nguyên văn): *"agent phải chạy session ở worktree và lấy context từ task chứ,
hoặc chưa có context thì yêu cầu"* — mở rộng thành vòng lặp đầy đủ:

```
Task → Agent sinh SPEC → cập nhật Task → người duyệt SPEC → Agent chạy SINH CODE
     → cập nhật Task → (vòng lặp: quay lại nếu cần sửa, hoặc kết thúc khi Done)
```

**Kết luận quan trọng sau khi rà lại code thật**: phần lớn hạ tầng cần thiết **đã tồn tại và
đang chạy thật** trong `task-service`/`infra-fleet-service` (BL-TG-04, đã implement, không phải
chỉ là spec) — đây không phải một tính năng xây từ đầu, mà là **cách dùng mới của các API đã có**,
cộng thêm vài chỗ hở nhỏ cần vá. Tài liệu này liệt kê chính xác cái gì đã có (kèm trích dẫn file),
cái gì còn thiếu, và 2 phương án thiết kế để chọn.

## Ý bạn nói "agent lấy context từ task" — đã có sẵn, không phải làm mới

`BL-TG-04` (`docs/logic/task-graph/BL-TG-04-task-agent-execution.md`) đã đặc tả và
**đã được code thật** (`backend-go/services/task-service/internal/usecase/execute_task.go`,
`.../adapter/grpcclient/simple_executor.go`): khi gọi `task.execute`, hệ thống tự:
1. Resolve quyền `execute` trên task, resolve dev server, resolve/tạo worktree
   (`ExecuteTask.Execute`, `execute_task.go:100-141`).
2. Build "Task Context Preamble" từ `task.title`/`task.description`/`task.aiContext`/
   `task.promptTemplate`/task cha/dependency đã xong, rồi mới spawn agent trong đúng worktree đó.
3. `task.promptTemplate` có thể được AI tự viết trước qua `GenerateAgentPrompt`
   (`generate_agent_prompt.go`) — hoặc override trực tiếp bằng `ExecuteTaskInput.Prompt` mỗi lần
   gọi `task.execute` (field `Prompt`, không lưu lại vào task — dùng đúng 1 lần).
4. Khi agent chạy xong: task **tự động chuyển sang `status: review`**
   (`domain.StatusReview`, `execute_task.go:208-217` cho đường đơn giản;
   `report_execution_result.go:72-76` cho đường phức tạp/workflow — cả 3 engine đều hội tụ về
   cùng 1 điểm `review` khi thành công).

→ **"Chưa có context thì yêu cầu" cũng đã có sẵn cơ chế**: `task.promptTemplate`/
`task.description`/`task.aiContext` trống thì preamble tương ứng trống — không có gì chặn Execute
khi thiếu, nhưng UI có thể (và nên) bắt buộc người dùng điền `description` trước khi cho bấm
"Generate Spec" nếu cả 2 field đều rỗng — 1 việc UI nhỏ, không cần API mới. `GenerateAgentPrompt`
(AI tự viết prompt) không có wscompat channel — xem "Phạm vi v1", cố tình để ngoài v1.

## Những mảnh ghép có sẵn khác, dùng được ngay

- **`Task.Labels []string`** (`domain/task.go:157`) — free-form tag. **Trước đây không có cách
  nào ghi vào từ RPC** (`UpdateTaskRequest` proto không có field `labels`, `task.update`'s
  wscompat handler không decode nó, `UpdateTaskInput` usecase không có field này —
  [BUG-024](../../../specs/backend-go/bugs/missing-v2/BUG-024-use-task-update-wrong-request-shape.md)).
  **Đã fix + deploy** (`2026.09.15-task-graph-labels-fix`): proto có `UpdateTaskRequest.labels`
  (kiểu `StringListValue`, field 16 — bọc message vì `repeated string` trần không phân biệt được
  "không gửi" vs "gửi rỗng"), usecase có `UpdateTaskInput.Labels *[]string` (nil = giữ nguyên,
  khác nil kể cả rỗng = thay toàn bộ danh sách), wscompat `task.update` decode `labels` đúng.
- **`comment.go`** — task đã có comment thread thật, dùng làm nơi trao đổi/duyệt spec. Response
  trước đây trả snake_case (`author_id`/`created_at`) — **đã fix cùng đợt**
  ([BUG-023](../../../specs/backend-go/bugs/missing-v2/BUG-023-task-channels-return-raw-snake-case-proto.md)).
- **`task.get`/`task.create`/`task.update` response** — trước đây trả raw proto snake_case
  (`promptTemplate`/`aiContext`/`taskNumber`/`prUrl`/`workflowTemplateId` đều `undefined` phía
  frontend, kể cả trên `TaskDetail.tsx`/`TaskPromptEditor.tsx` đã build sẵn) — **đã fix cùng đợt**
  (BUG-023), verify bằng test khẳng định JSON wire không còn key snake_case.
- **`useTask.ts`'s `updateTask()`** — trước đây gửi `{taskId, patch}`, 1 shape mà handler thật
  không đọc được gì cả (luôn set `id: ""`) — **mọi lần gọi `updateTask()` từ trước tới nay đều
  thất bại âm thầm** (BUG-024). Đã fix để gửi đúng `{id, title?, status?, workflowTemplateId?,
  labels?}` phẳng, chỉ gửi field thực sự có trong `patch`.
- **`git.*` wscompat channels** (đã fix toàn bộ trong phiên làm việc này — BUG-021) — người duyệt
  xem diff/file spec agent vừa commit ngay trong tab Git của worktree đó.
- **`ExecuteTaskInput.Prompt`** override, gửi qua `task.execute`'s `prompt` field — đúng cơ chế
  để phân biệt "lần Execute này là sinh spec" vs "lần Execute này là sinh code", không cần thêm
  field mới ở tầng RPC. **Đã có UI thật dùng đúng field này**: `TaskPromptEditor.tsx:28-34`.
- **`Task.SetStatus`** (`domain/task.go:232-244`) cho phép chuyển tự do giữa các status đã có (trừ
  vào `in_progress` và ra khỏi trạng thái cuối) — "duyệt" có thể chỉ là 1 lần gọi `task.update`
  bình thường, không cần state machine mới.
- **`orchestration-service`'s `DecisionGate`** (`decision_gates` table,
  `specs/backend-go/tdd/services/orchestration-service.md` §4-§5) — **đã code thật**
  (`services/orchestration-service/internal/usecase/create_gate.go`,`resolve_gate.go`) — là cơ chế
  "chặn cứng, không cho chạy tiếp cho tới khi người duyệt bấm approve", đúng ý "duyệt spec" chặt
  chẽ hơn. Nhưng **chỉ áp dụng cho task có subtask/dependency** (đi qua `EngineOrchestration` —
  `selectEngine`, `execute_task.go:220-247`), và **chưa xác nhận có UI nào gọi
  `ListPendingDecisionGates`/`ResolveDecisionGate` chưa** (nghi ngờ giống nhiều tính năng khác
  trong phiên này: backend xong, frontend chưa nối — cần audit riêng trước khi dựa vào).

## Phương án A — khuyến nghị, giờ đã 0 phần backend còn thiếu (BUG-023/024 đã xong)

Vòng lặp cho 1 task đơn (không subtask) — đi qua `EngineDirectAgent` (đường đơn giản).

### Quy ước label (phase state machine — chỉ 4 giá trị)

Cột `Task.Labels` chứa **tối đa 1** label thuộc tập này tại một thời điểm (mọi label khác của
người dùng không đụng tới):

| Label | Ý nghĩa | Khi nào set |
|---|---|---|
| *(không có)* | Chưa bắt đầu / đã Done | Khởi tạo, hoặc sau khi Done xoá hết |
| `phase:spec-pending` | Agent đang/đã sinh spec, chờ duyệt | Ngay trước khi gọi `task.execute` ở bước 1 |
| `phase:spec-approved` | Spec đã được duyệt, sẵn sàng implement | Lead bấm "Approve Spec" ở bước 2 |
| `phase:code-pending` | Agent đang/đã sinh code, chờ duyệt | Ngay trước khi gọi `task.execute` ở bước 3 |

Hàm thuần suy ra trạng thái UI từ `(task.status, task.labels)` — viết 1 lần, dùng cho cả nút bấm
lẫn hiển thị badge:
```ts
type SpecBuildPhase = 'not-started' | 'spec-pending' | 'spec-approved' | 'code-pending' | 'done'

function derivePhase(task: Pick<OrcaTask, 'status' | 'labels'>): SpecBuildPhase {
  if (task.labels.includes('phase:code-pending')) return 'code-pending'
  if (task.labels.includes('phase:spec-approved')) return 'spec-approved'
  if (task.labels.includes('phase:spec-pending')) return 'spec-pending'
  if (task.status === 'done') return 'done'
  return 'not-started'
}

// Non-phase labels the user may have set independently must survive every
// write — never blind-overwrite Task.Labels with just the phase tag.
const PHASE_LABELS = ['phase:spec-pending', 'phase:spec-approved', 'phase:code-pending']
function withPhase(currentLabels: string[], phase: string | null): string[] {
  const kept = currentLabels.filter((l) => !PHASE_LABELS.includes(l))
  return phase ? [...kept, phase] : kept
}
```

### Prompt template (nguyên văn, interpolate bằng template literal JS thường, không cần engine)

```ts
function buildSpecPrompt(task: Pick<OrcaTask, 'taskNumber' | 'title' | 'description'>): string {
  return `Write a detailed technical spec for this task. Do NOT write implementation code yet.

Requirements:
1. Read the existing code in this worktree relevant to the task below before writing the spec.
2. Save the spec as a new Markdown file at exactly this path: specs/generated/TASK-${task.taskNumber}-spec.md
3. The spec must cover: problem statement, proposed approach, affected files, edge cases, and a test plan.
4. Do not modify any other file in this worktree in this step.
5. Commit the new spec file with message: "docs(task-${task.taskNumber}): add spec for ${task.title}"

Task: ${task.title}
${task.description ?? ''}`
}

function buildImplementPrompt(task: Pick<OrcaTask, 'taskNumber' | 'title'>): string {
  return `Implement exactly the spec at specs/generated/TASK-${task.taskNumber}-spec.md in this worktree.

Requirements:
1. Read that spec file first — it is the source of truth for this change.
2. Implement what it describes. If you must deviate, say so explicitly in a code comment, don't implement silently differently.
3. Add or update tests per the spec's test plan.
4. Commit your changes with message: "feat(task-${task.taskNumber}): ${task.title}"

Task: ${task.title}`
}
```

### Luồng đầy đủ, từng bước, kèm đúng tên RPC/field đã verify

```
1. [SPEC] User bấm "Generate Spec" (TaskPromptEditor, nút mới)
   a. setPrompt(buildSpecPrompt(task))   — điền textarea có sẵn, KHÔNG auto-run
   b. User xem lại/sửa rồi bấm "Run with Agent" (nút đã có) → runWithAgent():
      callRuntimeRpc(target, 'task.execute', {
        taskId: task.id, projectId: project.id, worktreePath: currentWorktree.path,
        traceId: span.id, prompt: <nội dung textarea, đã trim>
      })
   c. NGAY TRƯỚC khi gọi Execute (b), gọi:
      callRuntimeRpc(target, 'task.update', { id: task.id, labels: withPhase(task.labels, 'phase:spec-pending') })
   d. Agent chạy trong đúng worktree (BL-TG-04 tự resolve) → khi xong, task.status
      tự chuyển 'review' (execute_task.go:208-217, đã có, không cần đụng vào)

2. [DUYỆT SPEC] derivePhase(task) === 'spec-pending' && task.status === 'review'
   → hiện nút "Approve Spec" cạnh nút Reject/comment
   a. Lead mở tab Git của đúng worktree (git.diff/git.status — đã fix BUG-021) xem
      file specs/generated/TASK-{{n}}-spec.md agent vừa commit
   b. Approve: callRuntimeRpc(target, 'task.update', {
        id: task.id, labels: withPhase(task.labels, 'phase:spec-approved')
      })
   c. Reject: callRuntimeRpc(target, 'task.addComment', { taskId: task.id, content: <góp ý> })
      rồi quay lại bước 1 (Execute lại KHÔNG bị chặn bởi status hiện tại — SetStatus's
      terminal-state guard chỉ áp cho UpdateTask, ExecuteTask không gọi qua SetStatus
      với 'in_progress' theo cách đó, xem execute_task.go:143)

3. [CODE] derivePhase(task) === 'spec-approved' → hiện nút "Implement Spec"
   a. setPrompt(buildImplementPrompt(task))
   b. Bấm "Run with Agent" như bước 1b
   c. Ngay trước Execute: task.update { labels: withPhase(task.labels, 'phase:code-pending') }
   d. Agent chạy xong → task.status = 'review' (tự động, như bước 1d)

4. [DUYỆT CODE] derivePhase(task) === 'code-pending' && task.status === 'review'
   → hiện nút "Approve & Mark Done" / "Request Changes"
   a. Lead xem diff qua tab Git (hoặc PR nếu agent tự tạo — Task.PRURL, ghi qua
      task.update { prUrl } một khi có UI/agent-hook ghi field này — NGOÀI PHẠM VI
      spec này, không giả định có sẵn)
   b. Đạt: callRuntimeRpc(target, 'task.update', {
        id: task.id, status: 'done', labels: withPhase(task.labels, null)
      })
   c. Không đạt: task.addComment nêu góp ý, quay lại bước 3
```

**Vì sao chọn A**: sau khi BUG-023/024 xong, **0 dòng code backend mới cần thêm nữa** — mọi RPC
cần dùng đã tồn tại, đúng shape, đã test, đã deploy. Việc còn lại 100% ở frontend (2 nút mới +
1 hàm thuần `derivePhase`/`withPhase` + 2 hàm build-prompt). Nhược điểm giữ nguyên như đã nêu:
"duyệt" chỉ là quy ước label, không CHẶN CỨNG bấm nhầm — chấp nhận được cho v1.

### Rủi ro còn lại, chưa xử lý ở v1 (không che giấu, xem thêm bảng điểm ở đầu)

- **Agent không tuân thủ đúng path spec** (`specs/generated/TASK-{{n}}-spec.md`): không có gì
  ép buộc ngoài chỉ dẫn trong prompt. Hardening v1.1 (chưa làm): sau khi Execute ở bước 1 xong,
  gọi `git.diff`/`files.readFile` (đã fix, dùng được) kiểm tra file đúng path có tồn tại trong
  commit mới nhất chưa — nếu không, hiện cảnh báo thay vì cho bấm "Approve Spec" im lặng.
- **Nhiều Lead cùng sửa 1 task cùng lúc**: label là quy ước, 2 người có thể set khác nhau race
  condition (ai ghi sau thắng, không có optimistic lock) — chấp nhận được ở quy mô nhóm nhỏ, xem
  Phương án B nếu cần chặt hơn.

## Phương án B — chặt hơn, dùng DecisionGate thật (theo sau, không làm ngay)

Model 2 pha spec/build thành 2 `orchestration_task` con của 1 `coordinator_run`, có
`depends_on` + 1 `DecisionGate` chắn giữa — task không thể dispatch pha build cho tới khi
`ResolveDecisionGate` được gọi. Đúng kiến trúc hơn (approval là 1 record thật, có audit trail,
`resolved_at`, `resolution` — không chỉ là quy ước label), nhưng cần:
1. Task phải có ít nhất 1 subtask/dependency để đi qua `EngineOrchestration` — nghĩa là phải tách
   task cha thành 2 task con (spec, build) thay vì 1 task chạy 2 lần như phương án A.
2. Audit xem `ComplexExecutor`'s cơ chế dispatch thật (mailbox + gõ lệnh
   `orca-dev orchestration send|ask|check` trong PTY, theo
   `docs/guides/task-automation/task-automation-orchestration-integration.md` mục 6.1) có đang
   chạy sống thật hay chỉ mới test đơn vị — khác hẳn cơ chế `agent.*` gRPC session vừa build ở
   phiên này, CHƯA xác nhận 2 hệ có tương thích hay đá nhau.
3. Xây UI cho `ListPendingDecisionGates`/`ResolveDecisionGate` — hiện nghi ngờ chưa tồn tại.

→ Đề xuất: làm A trước, dùng thật một thời gian; nếu approval-bằng-label không đủ chặt (VD nhiều
người cùng sửa 1 task, dễ bấm nhầm), quay lại B như bản nâng cấp có audit trail thật.

## Phạm vi v1 (chặn rõ, tránh scope creep khi implement)

- **Chỉ áp dụng cho task KHÔNG có subtask/dependency** (đi qua `EngineDirectAgent` —
  `selectEngine`, `execute_task.go:220-247`). Task có subtask đi qua `EngineOrchestration`, kế
  thừa `report_execution_result.go`'s "kẹt ở in_progress khi fail" gap — ngoài phạm vi v1.
- **Không tự động chuyển bước** — mỗi bước (Generate Spec / Approve Spec / Implement Spec /
  Approve Code) đều cần người bấm, không có trigger tự động khi task chuyển `review`.
- **Không đụng vào `GenerateAgentPrompt`** (chưa có wscompat channel — `task.generateAgentPrompt`
  không tồn tại) — v1 dùng prompt template viết sẵn (mục trên), không dùng AI để tự viết prompt.
- **Không đụng vào `PRURL`** tự động — nếu agent tự tạo PR, việc ghi `Task.PRURL` lại là một
  luồng riêng (agent hook hoặc CI callback) chưa được thiết kế ở đây.

## Tiêu chí chấp nhận (Phương án A, v1)

- [ ] `TaskPromptEditor.tsx` có nút "Generate Spec" — điền textarea bằng `buildSpecPrompt(task)`,
      set label `phase:spec-pending` (giữ nguyên label khác), rồi để user tự bấm "Run with Agent"
      có sẵn.
- [ ] Sau khi agent xong, task tự chuyển `status: review` (đã có sẵn — verify không có gì trong
      luồng A phá vỡ hành vi này, không cần code thêm cho việc này).
- [ ] `derivePhase(task) === 'spec-pending' && task.status === 'review'` → hiện nút "Approve
      Spec" (set label `phase:spec-approved`) và ô comment "Request Changes" (gọi
      `task.addComment` rồi cho phép Generate Spec lại).
- [ ] Lead xem được file `specs/generated/TASK-{{n}}-spec.md` agent vừa tạo qua tab Git của đúng
      worktree task đó (không cần UI mới — tab Git đã có, đã fix BUG-021).
- [ ] `derivePhase(task) === 'spec-approved'` → hiện nút "Implement Spec" — điền textarea bằng
      `buildImplementPrompt(task)`, set label `phase:code-pending`.
- [ ] Sau khi agent code xong, task lại về `status: review`; `derivePhase(task) === 'code-pending'`
      → hiện nút "Approve & Mark Done" (`task.update { status: done, labels: withPhase(labels,
      null) }`) và "Request Changes" (comment + quay lại Implement Spec).
- [ ] Có thể lặp lại Generate Spec/Implement Spec nhiều lần trên cùng 1 task (sửa theo góp ý) mà
      không bị chặn bởi status hiện tại — verify bằng cách Execute 2 lần liên tiếp trên cùng task
      ở status `review`.
- [ ] `withPhase()` không bao giờ xoá mất label không thuộc `phase:*` mà user đã tự gắn trước đó.

## Kế hoạch test

- **Unit (frontend, mới)**: `derivePhase`/`withPhase` — bảng truth-table đủ 5 trạng thái +
  1 case "giữ nguyên label lạ không phải phase:*".
- **Unit (frontend, mới)**: `buildSpecPrompt`/`buildImplementPrompt` — snapshot test khẳng định
  path `specs/generated/TASK-{{n}}-spec.md` xuất hiện đúng trong cả 2 template.
- **Component (frontend, mới)**: `TaskPromptEditor.test.tsx` — bấm "Generate Spec" → textarea
  chứa prompt đúng, `task.update` được gọi với label đúng TRƯỚC `task.execute` (thứ tự quan
  trọng — xem bước 1b/1c).
- **Đã có, đã pass (backend, phiên này)**: `TestTaskGetChannel_MapsToCamelCaseView`,
  `TestTaskUpdateChannel_ThreadsLabels`, `TestTaskUpdateChannel_OmittedLabelsStaysNil`,
  `TestUpdateTask_ReplacesLabels`, `TestUpdateTask_NilLabelsLeavesExistingUntouched`,
  `TestUpdateTask_EmptyLabelsClearsList` — nền tảng RPC đã verify, không cần viết lại.
- **Thủ công, sau khi có tài khoản AI Provider hợp lệ** (chặn bởi vấn đề credential đã báo riêng):
  chạy live cả 4 bước trên 1 task thật, xác nhận file spec thật xuất hiện trong worktree.

## Tham chiếu

- [BL-TG-04](./BL-TG-04-task-agent-execution.md) — nền tảng "task → agent context → review" mà
  tài liệu này mở rộng, đã code thật.
- [BUG-023](../../../specs/backend-go/bugs/missing-v2/BUG-023-task-channels-return-raw-snake-case-proto.md) /
  [BUG-024](../../../specs/backend-go/bugs/missing-v2/BUG-024-use-task-update-wrong-request-shape.md) —
  2 bug nền tảng phát hiện + fix + deploy khi viết spec này, không có thì Phương án A không chạy
  được.
- `backend-go/services/task-service/internal/usecase/execute_task.go`, `update_task.go`,
  `report_execution_result.go`, `domain/task.go`
- `frontend/src/renderer/src/components/task/TaskPromptEditor.tsx`,
  `TaskDetail.tsx` — component thật cần sửa cho v1
- `specs/backend-go/tdd/services/orchestration-service.md` — DecisionGate, cho Phương án B
- `docs/guides/task-automation/task-automation-orchestration-integration.md` §6.1, §9.2 — 2 hệ
  điều phối thật, không share code, cần audit trước khi dựa vào cho Phương án B
- [SOL-FE-PW-004](../../../specs/frontend/bugs/project-workspace/solutions/SOL-FE-PW-004-agent-panel-web-orchestration.md) —
  AgentPanel/`agent.start`, hệ khác, không phải nền tảng của tài liệu này
