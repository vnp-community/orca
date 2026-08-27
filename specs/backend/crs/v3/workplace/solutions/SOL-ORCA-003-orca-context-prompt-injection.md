> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
# SOL-ORCA-003 — Orca Context & Prompt Injection

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-003](../../../../../../docs/crs/v3/orca/CR-ORCA-003-orca-context-prompt-injection.md) |
| **Title** | Orca Context & Prompt Injection — Inject KGP context vào AI agent prompt |
| **Service** | Orca `TaskAIPlanner` / `TaskAgentExecutor` (repo `orca`, Electron/TypeScript) |
| **Priority** | P1 |
| **Risk** | medium |
| **Status** | 📐 PROPOSED |
| **Phạm vi** | **Ngoài phạm vi backend Go `vnp-workplace`.** Mô tả **hợp đồng dữ liệu** (trường nào `planner-service` (:3013, thuộc `vnp-workplace`) gửi, ý nghĩa từng trường, đảm bảo bảo mật kỳ vọng ở tool permission) mà Orca team hiện thực trong `PlannerPromptBuilder`/`PlannerWorktreeSetup`. Không thiết kế nội bộ TypeScript. |
| **Ghi chú re-scope** | Đã re-scope từ `vnp-planner` sang `vnp-workplace`/`planner-service` theo quyết định kiến trúc 2026-08-10, đồng bộ CR-ORCA-003 đã viết lại — xem `docs/crs/v3/orca/README.md`. Nội dung Orca-side (TypeScript) không đổi, chỉ đổi thuật ngữ hệ thống gửi task. |
| **TDD refs** | — (không có TDD Go tương ứng — thuần đóng góp field mapping) |
| **Depends on** | [SOL-ORCA-001](./SOL-ORCA-001-orca-api-bridge.md) |

---

## 1. Tóm tắt vấn đề & mục tiêu

> ⚠️ **Xác thực với Orca thật (2026-08-10):** `POST /api/planner-tasks` **chưa tồn tại** (SOL-ORCA-001 §9), và cơ chế "inject field vào prompt" mô tả trong SOL này **không khớp** với `TaskAgentExecutor.buildPrompt()` thật (`backend/src/main/task/TaskAgentExecutor.ts:114-131`) — hàm thật chỉ nội suy `task.promptTemplate` (`${task.*}`) hoặc tự sinh prompt từ `title` + `description` + `aiContext`, hoàn toàn không có khái niệm "Why Chain / Anti-Patterns / Required Patterns / Acceptance Criteria" như mô tả dưới đây. Toàn bộ §2 là **đề xuất field mới** cho `PlannerPromptBuilder` — một component **chưa tồn tại**, không phải mô tả hành vi hiện có. Xem §9 cuối file.

Các trường `planner-service` gửi trong `POST /api/planner-tasks` (SOL-ORCA-001 §3.2, sinh ra từ `SubmitOrcaTaskInput` ở SOL-ORCA-002 §3.6) chỉ có giá trị nếu Orca **injects** đúng và đầy đủ vào system prompt của AI agent, đồng thời giới hạn quyền hạn agent (tool permission) sao cho an toàn khi chạy không giám sát trong 2-8 giờ. Mục tiêu SOL này là ràng buộc rõ: (1) field nào **bắt buộc** phải xuất hiện trong prompt cuối, (2) cấu trúc worktree/git identity kỳ vọng, (3) baseline tool-permission mà `planner-service` (`vnp-workplace`) tin tưởng Orca áp dụng — vì đây là **security boundary** giữa 2 hệ thống.

## 2. Field mapping — nguồn sự thật là SOL-ORCA-002 §3.6

| Trường JSON (`POST /api/planner-tasks`) | Vai trò trong prompt (Orca dựng) | Bắt buộc xuất hiện |
|---|---|---|
| `title` + `description` | Task prompt chính (`# Task: {title}\n\n{description}`) — `description` là nội dung TASK file tự chứa, agent không cần hỏi thêm | Có |
| `why_chain[]` | Mục "Context: Why This Task Exists" — breadcrumb task→solution→CR→goal | Có, nếu non-empty |
| `anti_patterns[]` | Mục "⚠️ Anti-Patterns (DO NOT USE)" | Có, nếu non-empty |
| `required_patterns[]` | Mục "✅ Required Patterns (MUST FOLLOW)" | Có, nếu non-empty |
| `acceptance_criteria[]` | Mục "Acceptance Criteria" dạng checklist — agent PHẢI tick hết trước khi báo `done` | **Bắt buộc** — Orca không được phép báo `success: true` nếu agent tự nhận chưa hoàn thành checklist |
| `worktree_repo` + `worktree_branch` | Base cho `git worktree add {path} -b planner/{planner_task_id} {branch}` | Có |
| `agent_type` | Chọn executor (`claude`/`codex`/`opencode`) | Có |
| `planner_cr_id` | Gắn label truy vết (`planner:{cr_id}`), không đưa vào prompt hiển thị | Không bắt buộc trong prompt, bắt buộc trong metadata/label |

> `planner-service` **không** gửi raw code hoặc bí mật (API key, credential) trong bất kỳ trường nào ở trên — Orca không cần cơ chế redact bổ sung cho input này.
>
> **Nguồn dữ liệu của các trường context (theo CR-TASK-003 — Task Context Enrichment, đã re-scope):** `why_chain`, `required_patterns`, và phần "related context" gộp vào `description` được `planner-service` build qua `TaskEnricher` — `why_chain` build local (đọc DB `planner-service`, không qua network), còn `required_patterns` lấy qua gọi nội bộ `skills-service` (:3008, catalog-injection API). **`anti_patterns` hiện luôn là mảng rỗng** khi `planner-service` gửi sang Orca: `vnp-workplace` không có service tương đương `memory-svc`/lessons-learned của thiết kế gốc (đã xác nhận qua rà soát code — xem CR-TASK-003 Open Tasks), nên trường này **không có nguồn dữ liệu** tại thời điểm hiện tại. Orca team implement `PlannerPromptBuilder` vẫn giữ nhánh xử lý `anti_patterns` (theo §2 dưới), chỉ là nhánh đó sẽ không được kích hoạt cho tới khi có CR riêng bổ sung nguồn tương đương `memory-svc` ở `vnp-workplace`.

## 3. Yêu cầu Worktree Isolation (ràng buộc bảo mật)

> **Xác thực với Orca thật:** worktree creation trong Orca hôm nay là **luồng do người dùng/UI khởi tạo** (`store.createWorktree(...)`, xem `frontend/src/renderer/src/hooks/useComposerState.ts:3644`, `frontend/src/renderer/src/components/terminal-pane/terminal-agent-session-fork.ts:240-242`) — **không phải** do `TaskAgentExecutor` tự động tạo per-task. `TaskAgentExecutor.executeTask(params)` (`backend/src/main/task/TaskAgentExecutor.ts:25-33,48`) nhận `worktreePath` như một **input bắt buộc phải tồn tại sẵn** — hàm này không gọi `git worktree add`, không tạo branch `planner/{task_id}`, không có tham số `worktree_repo`/`worktree_branch`. 4 điểm dưới đây là **yêu cầu bảo mật cần Orca team hiện thực mới** nếu muốn tự động hoá theo mô hình "1 task planner = 1 worktree cô lập tự tạo" — hiện tại hoàn toàn chưa có.

`planner-service` (`vnp-workplace`) tin tưởng Orca đảm bảo, cho **mọi** task nhận từ `/api/planner-tasks` (endpoint đề xuất, xem SOL-ORCA-001 §9):

1. Mỗi task chạy trong **1 git worktree riêng biệt**, không share working directory giữa các task chạy song song (tránh AI agent A ghi đè agent B).
2. Branch tạo mới có tên xác định (`planner/{planner_task_id}`) — **không** commit thẳng lên `worktree_branch` gốc.
3. Git identity commit là identity riêng cho AI (không mạo danh user thật), phục vụ audit sau này (SOL-ORCA-004 đọc `commit_hash` để verify).
4. Worktree được **cleanup** (`git worktree remove`) sau khi kết quả đã được `PlannerResultCollector` (SOL-ORCA-004 — component đề xuất, chưa tồn tại) thu thập xong — không cleanup sớm hơn, nếu không `files_created`/`files_modified` sẽ rỗng sai.

## 4. Tool Permission Baseline (ràng buộc bảo mật)

> **Xác thực với Orca thật:** không tìm thấy cấu hình "tool permission preset theo nhãn `planner:*`" nào trong Orca hiện tại. Cơ chế quyền thật gần nhất là `TaskGrantService.resolvePermission()` (per-user RBAC trên task: view/comment/edit/execute/manage — `agent/src/shared/task-types.ts:83-96,117-123`), đây là quyền của **user Orca** trên task, không phải quyền của **AI agent process** trên các lệnh shell nó chạy. Bảng baseline dưới đây vẫn là ràng buộc bảo mật hợp lý cần yêu cầu Orca team implement, nhưng nên ghi rõ đây là **tính năng mới** (áp permission-per-command cho tiến trình PTY do `ProfileAwareAgentSpawner.spawn()` khởi tạo), không phải điều chỉnh cấu hình có sẵn.

`planner-service` kỳ vọng preset tối thiểu sau cho mọi task gắn nhãn `planner:*` (không cần giống hệt về mặt cú pháp Orca, nhưng **hành vi** phải tương đương):

| Nhóm lệnh | Chính sách |
|---|---|
| Đọc/ghi file trong worktree, `git add/commit` | Auto-accept |
| `go build`, `go test`, `go vet`, `go generate` (hoặc tương đương ngôn ngữ khác) | Auto-accept |
| `git push` | **Không auto-accept** — `vnp-workplace` merge/PR thủ công hoặc qua pipeline riêng, không để AI agent tự push lên remote |
| Tải file ngoài (`curl`, `wget`), cài package hệ thống (`apt`, `brew`) | **Không auto-accept** — nguy cơ supply-chain khi chạy không giám sát |
| Lệnh phá hoại workspace ngoài phạm vi worktree (`rm -rf /`, thao tác ngoài `worktree_path`) | Bị chặn tuyệt đối bởi Orca sandbox, không thuộc phạm vi cấu hình theo task |

Đây là **ràng buộc hành vi**, không phải API — `planner-service` không có cách kiểm chứng runtime ngoài việc review `agent_output`/`commit_hash` trả về (SOL-ORCA-004). Rủi ro liên quan xem §6.

## 5. Tích hợp với các CR khác

- **CR-ORCA-001**: nguồn field — mọi field ở §2 phải khớp `TaskRequest` JSON schema.
- **CR-ORCA-002**: `SubmitOrcaTaskInput` (temporal-worker) là nơi các field này được set trước khi gửi — không đổi tên/field ở 2 phía mà không đồng bộ.
- **CR-ORCA-004**: `commit_hash`, `files_created`, `files_modified` trả về được đối chiếu ngược với `worktree_repo`/`worktree_branch` đã gửi ở đây để verify tính toàn vẹn.

## 6. Rủi ro & giảm thiểu (góc nhìn `planner-service`/`vnp-workplace`)

| Rủi ro | Giảm thiểu |
|---|---|
| Orca "quên" inject `acceptance_criteria` → agent báo `done` dù chưa đạt tiêu chí | `planner-service` review `agent_output` + chạy lại acceptance test độc lập trước khi coi task thực sự hoàn tất — không tin tuyệt đối `result.success` từ Orca |
| Tool permission bị nới lỏng ngoài baseline §4 (vd. agent tự `git push`) | Yêu cầu Orca log mọi lệnh bash thực thi vào `agent_output`/trace-stream; `vnp-workplace` team audit ngẫu nhiên; escalate nếu phát hiện `git push` không mong đợi |
| Worktree không cô lập đúng, 2 task ghi đè nhau khi chạy song song (liên quan CR-ORCA-002 fan-out) | Trước khi bật `MaxConcurrency > 1` trong production, chạy smoke test 2 task đồng thời trỏ cùng `worktree_repo` khác branch, xác nhận không xung đột |

## 7. Ước tính công việc

Thuộc repo `orca` (TypeScript) — xem effort estimate gốc trong CR-ORCA-003 (17h). Phía `vnp-workplace` (`planner-service`): 0h thực thi, nhưng cần 2h review/ký xác nhận field mapping (§2) trước khi Orca team code.

## 8. Dependencies

Phụ thuộc CR-ORCA-001. Song song với CR-ORCA-002 (không block lẫn nhau, nhưng field contract phải khớp).

---

## 9. Xác thực với Orca thật (cập nhật — khảo sát ngày 2026-08-10)

Đã đối chiếu với code thật tại `/opt/repos/orca`. Sửa đổi chính:

1. **`buildPrompt()` thật đơn giản hơn nhiều và không có 5/8 field ở bảng §2.** Source thật (`backend/src/main/task/TaskAgentExecutor.ts:114-131`):
   ```ts
   buildPrompt(task: OrcaTask): string {
     if (task.promptTemplate) { return task.promptTemplate.replace(/\$\{task\.([^}]+)\}/g, ...) }
     const lines = [`# Task: ${task.title}`]
     if (task.description) lines.push(`\n## Description\n${task.description}`)
     if (task.aiContext)   lines.push(`\n## AI Context\n${task.aiContext}`)
     lines.push(`\n## Instructions`, `Complete the task described above. When finished, the task status will be moved to "review".`)
     return lines.join('\n')
   }
   ```
   Không có mục "Why This Task Exists", "Anti-Patterns", "Required Patterns", hay "Acceptance Criteria" dạng checklist. `why_chain`/`anti_patterns`/`required_patterns`/`acceptance_criteria` **không tồn tại** trong `OrcaTask`/`CreateTaskParams` (`agent/src/shared/task-types.ts:39-80`) — bảng field mapping ở §2 là đặc tả **yêu cầu bổ sung** cho một `PlannerPromptBuilder` chưa tồn tại, cần: (a) thêm field vào `OrcaTask`/`CreateTaskParams` + migration `orca_tasks` (migration 0010 hiện tại: `id, project_id, parent_id, title, description, type, status, priority, labels, visibility, reporter_id, assignee_id, estimated_hours, progress_percent, ai_context, prompt_template, due_date` — xem `TaskService.ts:106-160`), (b) viết lại `buildPrompt()` hoặc thêm `PlannerPromptBuilder` riêng khi task có nguồn gốc từ planner (ví dụ đánh dấu qua `labels: ['planner:*']`).
2. **Worktree isolation (§3) là tính năng chưa tồn tại**, không phải hành vi hiện có — xem ghi chú tại §3 ở trên. `TaskAgentExecutor.executeTask()` giả định `worktreePath` đã tồn tại sẵn, được set từ đâu đó ngoài phạm vi class này (điều tra thêm nếu cần: caller thật trong `task-rpc-handler.ts`, RPC `tasks.runAgent`).
3. **Tool permission baseline (§4) chưa có cơ chế tương ứng** — xem ghi chú tại §4 ở trên. Không tìm thấy allowlist/denylist lệnh shell theo nhãn task nào trong `ProfileAwareAgentSpawner`/`node-pty spawn` path.
4. **`agent_output`/`commit_hash` không có nguồn thu thập** — không có collector nào trong `TaskAgentExecutor` hay `TaskService` capture git diff/commit hash/test output; chỉ có 1 dòng activity comment text khi hoàn tất (`TaskAgentExecutor.ts:87-104`). Rủi ro ở bảng §6 dòng 1 ("Orca quên inject acceptance_criteria") cần bổ sung thêm: **hiện tại còn chưa có gì để "quên" — cả acceptance_criteria lẫn cơ chế collect kết quả đều chưa tồn tại.**

**Kết luận:** SOL-ORCA-003 vẫn đúng vai trò là **đặc tả hợp đồng bảo mật/dữ liệu cần có**, nhưng cần đọc như yêu cầu xây mới hoàn toàn phía Orca (prompt builder mở rộng + worktree automation + tool permission theo nhãn), không phải mô tả field mapping của tính năng đã tồn tại.
