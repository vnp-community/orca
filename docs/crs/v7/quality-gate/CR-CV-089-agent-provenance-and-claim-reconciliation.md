# CR-CV-089 — Dấu vết agent (provenance) và đối chiếu "agent tự báo" với kết quả chạy lại

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-089 |
| **Tên** | Bảng `agent_turns` (lượt agent bền), cách bắt ranh giới lượt, trích "lời agent tự báo", đối chiếu độc lập với `quality.run`, chính sách che dữ liệu và lưu trữ |
| **Loại** | Feature (dữ liệu + nghiệp vụ; phụ thuộc nhiều vào dữ liệu hook chưa kiểm chứng) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-085 (`quality_trend_points.turn_key`, quyền), CR-CV-081/082 (`quality.run`, `QualityRun`), CR-CV-080 (tín hiệu "agent xong"), CR-CV-011/013 (bảng, che bí mật, quyền), CR-CV-060 (`turnId`) |
| **Mở khoá** | CR-CV-087 (so sánh theo lượt), CR-CV-090 (báo cáo), CR-CV-092 (lượt ↔ task), CR-CV-095 |
| **Tác động** | `backend-go/services/code-intel-service` (domain, usecase, repository hai dialect, migration), `backend-go/proto/orca/codeintel/v1/agent_turn.proto`, `api-gateway` (3 kênh mới), tuỳ chọn `backend-go/services/infra-fleet-service` (nguồn hook phía backend), `frontend/src/renderer/src/components/review-map/turns/` (nối với CR-CV-060). Nguồn: [research 11](../../../research/view-code/11-additions-for-quality-control.md) C5 |

---

## 1. Bối cảnh và vấn đề

Research 11 C5 muốn biết "agent/model nào sinh, prompt, các lệnh đã chạy, và *agent tự nói test pass nhưng kết quả chạy lại thế nào*", với ghi chú "chưa kiểm chứng đủ dữ liệu". Phần này đọc code thật để tách cái **đã có** khỏi cái **chưa lưu bền** (ngày 2026-10-06).

### 1.1 Dữ liệu có thật, và nơi nó dừng lại

| Thông tin | Có không | Ở đâu (đã đọc) | Bền? |
|---|---|---|---|
| Loại agent (`claude`, `codex`…) | Có | `AgentStatusEntry.agentType`; `source` của envelope `agent.hook` (`agent/src/shared/agent-hook-relay.ts`, `AgentHookSource`: 17 giá trị) | Bộ nhớ renderer; `last-status.json` ở desktop main (TTL, chỉ trạng thái cuối theo pane — `agent/src/main/agent-hooks/server.ts` `LAST_STATUS_FILE_NAME`) |
| Trạng thái/ranh giới lượt | Có | `state ∈ working\|blocked\|waiting\|done`, `stateStartedAt`, `interrupted`, `stateHistory[]` tối đa `AGENT_STATE_HISTORY_MAX = 20` (`agent/src/shared/agent-status-types.ts`) | Không bền (slice `agent-status.ts`: "real-time, lives in renderer memory") |
| Prompt | Có, **rút gọn** | `AgentStatusEntry.prompt`, cắt ở `AGENT_STATUS_MAX_FIELD_LENGTH = 200` (`agent-status-field-normalization.ts`) | Không bền |
| Lệnh/tool đã dùng | **Chỉ tool hiện tại** | `toolName`, `toolInput` ("short preview"), không có lịch sử; chú thích của `AgentStateHistoryEntry` nói rõ lịch sử cố ý **bỏ** `toolName/toolInput/lastAssistantMessage` | Không |
| Mã thoát của lệnh | **Không có** | `AgentStatusPayload` không có trường exit code/kết quả tool | — |
| Lời cuối của agent | Có, xem trước | `lastAssistantMessage` ("preview") | Không |
| Model | Một phần | Go: `infra.agent_sessions.model_id` (chỉ phiên do backend spawn; `domain/agent_session.go`, migration `0019_agent_sessions`); AI Vault: `AiVaultSession.model` đọc từ tệp transcript của CLI (`shared/ai-vault-types.ts`; subagent chỉ đọc local, host khác trả rỗng) | Go: có; AI Vault: tệp của CLI, không phải kho của Orca |
| Id phiên của CLI | Có | `providerSession` (`key`,`id`) → Go `agent_sessions.resume_provider_session_*` qua `RecordAgentHookProviderSession` | Có (Go) |
| Task ↔ agent | Có | `task.tasks.WorktreeID`, `AgentSessionID`, `task.execution_links` (engine, `status_mirror`), `LastExecutionOutput` ≤ 8 KB của lần chạy thành công cuối (`task-service/internal/domain/task.go`, migration `0010`) | Có (task-service) |
| Điều phối | Có | `orchestration.messages` (`type` gồm `worker_done`; `payload` JSONB), `dispatch_contexts` | Có (orchestration-service) |

### 1.2 Hai khoảng trống quan trọng

1. **Backend Go chỉ dùng một phần nhỏ của `agent.hook`.** `agentHookNotificationParams` (`infra-fleet-service/internal/adapter/devserveragent/session.go:539`) chỉ giải mã `worktreeId`, `ptyId`, `providerSession.{key,id}`; `state`, `prompt`, `toolName`, `lastAssistantMessage` bị bỏ. Consumer duy nhất là `RecordAgentHookProviderSession`.
2. **Nguồn phát `agent.hook` cho đường `direct-websocket` (Part A) chưa tìm thấy.** `RelayAgentHookServer` (HTTP loopback có token `x-orca-agent-hook-token`) chỉ được khởi ở `agent/src/relay/relay.ts:554` (Part B, đường SSH relay), và trong `agent/src/relay/` chỉ bốn tệp (`agent-hook-server.ts`, `relay.ts`, `wsl-agent-hook-relay.ts`, `plugin-overlay.ts`) nhắc tới `RelayAgentHookServer`/`AGENT_HOOK_NOTIFICATION_METHOD`/`agent_hook` (grep tên tệp, bỏ test). Tìm trong `agent-session.ts`, `agent-entry.ts`, `agent-connection-*.ts` không thấy khởi hook server (phủ định chưa chứng minh tuyệt đối). `agent-spawner.ts:242` đã gán `ORCA_PTY_ID` cho tiến trình agent do backend spawn "để một script hook sau này có thể báo ptyId" — tức đường Part A **chưa nối xong**. Vì dữ liệu hook ở mức chi tiết là điều kiện cho C5, đây là rủi ro số 1 (mục 6).
3. **Tin cậy.** Tiến trình agent nhận token hook trong env của PTY (relay `buildPtyEnv`), nên chính agent (hoặc mã nó chạy) có thể POST sự kiện giả. Provenance ở đây là **bằng chứng ghi nhận, không chống giả mạo**; UI không được trình bày như "đã xác thực".
4. **CR-CV-060 tự tạo "mốc lượt" ở frontend** (`ReviewTurnMarker`, `turnId = "${paneKey}:${doneAt}"`, tối đa 5 mốc trong `review_states`) và quyết định **không lưu prompt của người dùng lên backend** vì có thể nhạy cảm (CR-CV-060 mục 2.4). CR này phải tương thích, không phá nguyên tắc đó.

Vấn đề: backend chưa có khái niệm "lượt" bền để gắn kết quả chất lượng (CR-CV-085 `turn_key` đang là chuỗi mờ), và chưa có cách so lời agent nói với kết quả chạy độc lập.

## 2. Giải pháp đề xuất

### 2.1 Ranh giới lượt: hai nguồn, khử trùng theo `client_turn_id`

Lượt = một chu kỳ `working → done` của một pane agent trong một worktree (định nghĩa của CR-CV-060/061). Danh tính: `client_turn_id = "${paneKey}:${doneAt}"` (đúng `turnId` của CR-CV-060, `doneAt = entry.stateStartedAt` của lần `done`).

| Nguồn | Khi nào dùng | Ưu / nhược |
|---|---|---|
| **A. Renderer ghi** (`use-review-turn-recorder` của CR-CV-060 gọi thêm `RecordAgentTurn`) | Mặc định MVP; không đòi gì ở dev server | Có `agentType`, `interrupted`, danh sách tệp (dấu vân tay thô); **chỉ ghi khi renderer đang mở**; mất lượt nếu đóng app |
| **B. Backend ghi** (infra-fleet `StreamAgentHooks` mở rộng giải mã `state`, `hookEventName`, `toolName`, rồi gửi tới code-intel-service) | Khi Part A phát được `agent.hook` (mục 1.2) và CR-CV-080 chọn tín hiệu này | Không phụ thuộc renderer; cần sửa Go + agent; chưa kiểm chứng |

`RecordAgentTurn` là **upsert theo `(tenant_id, repo_binding_id, client_turn_id)`**: nguồn A và B gộp thành một dòng (trường nào có thì điền, không ghi đè trường đã có bằng giá trị rỗng). `source` ghi `renderer|hook|both`.

**Commit trước/sau**: `end_head_commit` = HEAD của worktree lúc ghi (lấy từ `git.*` qua collector hoặc từ `GitBranchCompareResult.summary.headOid`, CR-CV-060 1.2); `base_head_commit` = `end_head_commit` của lượt liền trước của cùng binding (chuỗi), `NULL` ở lượt đầu. Không tự đoán "lúc agent bắt đầu" vì không có sự kiện đáng tin (research 11 nói "chưa kiểm chứng"). Cũng ghi `tree_dirty_end` (có thay đổi chưa commit) vì phần lớn lượt kết thúc khi chưa commit.

### 2.2 Mô hình `agent_turns` (bảng mới, hai dialect)

| Cột | Kiểu | Ghi chú |
|-----|------|---------|
| `id` | uuid | PK, ứng dụng sinh |
| `tenant_id` | uuid | NOT NULL |
| `repo_binding_id` | uuid | NOT NULL (khoá theo binding như `review_states`; id worktree có 3 dạng chuỗi, CR-CV-011) |
| `client_turn_id` | varchar(128) | NOT NULL; duy nhất cùng `(tenant_id, repo_binding_id)` |
| `agent_type` | varchar(64) | NOT NULL DEFAULT `unknown`; chuỗi tự do chuẩn hoá chữ thường (`AgentType` không phải tập đóng) |
| `model` | varchar(128) | NULL; chỉ khi biết |
| `model_source` | varchar(16) | `agent_session\|transcript\|unknown` |
| `agent_session_id` | uuid | NULL, `infra.agent_sessions.id` (id tham chiếu, không FK) |
| `source` | varchar(8) | `renderer\|hook\|both` |
| `started_at` | timestamptz | NULL (chỉ khi biết, từ `stateHistory`) |
| `ended_at` | timestamptz | NOT NULL (`doneAt`) |
| `interrupted` | boolean | NOT NULL DEFAULT false |
| `base_head_commit`, `end_head_commit` | varchar(64) | `end` NOT NULL |
| `tree_dirty_end` | boolean | NOT NULL |
| `files_changed_count` | int | NOT NULL DEFAULT 0 |
| `files_digest` | char(64) | sha256 của danh sách dấu vân tay tệp đã sắp (CR-CV-060 `turn-file-identity`); dùng so sánh, không giải ngược được |
| `prompt_digest` | char(64) | sha256 của prompt đã chuẩn hoá; **không có nội dung** (mục 2.5) |
| `prompt_excerpt` | varchar(160) | NULL; chỉ khi tenant bật tuỳ chọn (mục 2.5) |
| `commands_summary` | jsonb/json | ≤ 8 KiB (mục 2.3) |
| `claims` | jsonb/json | ≤ 8 KiB (mục 2.4) |
| `verification` | jsonb/json | ≤ 8 KiB (mục 2.4) |
| `created_at`, `updated_at` | timestamptz | đồng hồ DB |
| `expires_at` | timestamptz | NOT NULL (mục 2.6) |
| `version` | bigint | NOT NULL DEFAULT 1 |

Chỉ mục `(tenant_id, repo_binding_id, ended_at DESC)`, `(tenant_id, repo_binding_id, end_head_commit)`. Không FK. Postgres: RLS theo mẫu CR-CV-011; MySQL lọc `tenant_id` ở mọi `WHERE`. Giá trị `quality_trend_points.turn_key` của CR-CV-085 = `client_turn_id`, nên hai bảng nối được mà không phụ thuộc thứ tự triển khai.

### 2.3 Tóm tắt lệnh (`commands_summary`) — chỉ dạng đã chuẩn hoá

Không bao giờ lưu nguyên `toolInput`. Bộ chuẩn hoá `domain/agent_command_summary.go` (hàm thuần) nhận chuỗi xem trước, giữ **tên chương trình + tối đa một tiểu lệnh** (`pnpm test`, `go test`, `git commit`, `golangci-lint`), bỏ đối số, đường dẫn, biến môi trường, chuyển hướng, URL; ánh xạ sang `category ∈ test|lint|typecheck|build|install|git|other` bằng bảng mẫu có phiên bản:

```jsonc
{ "v": 1, "totalToolUses": 42,
  "commands": [ { "name": "pnpm", "sub": "test", "category": "test", "count": 3 },
                { "name": "go", "sub": "test", "category": "test", "count": 1 } ],   // ≤ 20 mục
  "toolCounts": { "Edit": 18, "Bash": 12, "Read": 9 }, "truncated": false }
```

Nguồn dữ liệu là chuỗi sự kiện tool (`toolName`/`toolInput` ở mỗi `agent.hook`). Vì backend hiện không nhận được chúng (mục 1.2), `commands_summary` **rỗng cho tới khi có nguồn B** hoặc renderer tự gom (renderer thấy mọi cập nhật `agent-status` nên gom được khi đang mở). Xem Q2.

### 2.4 "Agent tự báo" và đối chiếu độc lập

Cấu trúc claim (`claims`):

```jsonc
{ "v": 1, "items": [
  { "kind": "tests_pass|tests_fail|lint_clean|typecheck_clean|build_ok|all_done",
    "basis": "ran_command|stated",          // chạy lệnh tương ứng | nói trong lời cuối
    "confidence": "medium|low",             // không có "high": không có mã thoát
    "evidence": "<≤120 ký tự, đã che>" } ] }
```

- `ran_command`: `commands_summary` có lệnh thuộc `category` tương ứng. Chỉ chứng minh **agent đã chạy lệnh**, không chứng minh nó đạt (không có mã thoát). Dùng làm "đã chạy kiểm tra", **không** gọi là "báo đạt".
- `stated`: khớp mẫu trong `lastAssistantMessage` (xem trước, bị cắt; nhiều ngôn ngữ). Mức `low`, **mặc định tắt** (`agent_claim_text_enabled=false` theo tenant) vì dễ báo nhầm và là trích dẫn từ văn bản agent.

`verification` do consumer của sự kiện `orca.codeintel.quality.run_finished` (CR-CV-081/082) và `index.changed` điền khi có **run độc lập** cùng `end_head_commit`: run do runner của Orca chạy (`QualityRun.source = local`, hoặc `ci` từ CR-CV-086), **không** phải kết quả do agent tự chạy. Mỗi claim nhận:

| `agreement` | Điều kiện | Câu hiển thị gợi ý (không kết tội) |
|---|---|---|
| `consistent` | claim có, run độc lập cùng category `succeeded`, không `error` | "Chạy lại: đạt" |
| `contradicted` | claim `stated/ran_command` có, run độc lập cùng category có `error`/test thất bại | "Kết quả chạy lại khác với lời agent nói: N lỗi" kèm liên kết tới run |
| `unverified` | chưa có run độc lập, hoặc run `unknown` do môi trường (`CODEINTEL_ENV_NOT_READY`) | "Chưa đối chiếu" + nút "Chạy lại kiểm tra" |
| `not_claimed` | agent không đề cập | không hiển thị mặc định |

Quy tắc diễn giải: (1) chạy lại khác kết quả **không** có nghĩa agent nói dối (môi trường khác, test không ổn định, cây làm việc thay đổi sau lượt); UI không dùng chữ "nói dối", "giả mạo", "đánh lừa"; (2) `contradicted` chỉ được đặt khi run có `headCommit = end_head_commit` và `tree_dirty_end=false`, hoặc khi run mang dấu vân tay cây làm việc khớp (nếu CR-CV-081/082 thêm `treeFingerprint`, Q4 của CR-CV-085); cây còn bẩn mà không có dấu vân tay → `unverified` + lý do `tree_may_differ`; (3) `consistent` không bao giờ nâng cổng của CR-CV-085 (cổng chỉ dựa kết quả chạy độc lập, không dựa lời agent); (4) có tuỳ chọn (mặc định tắt) cho phép `contradicted` thêm một dòng `warn` trong cổng — Q3.

Giao diện (do CR-CV-087/060 vẽ, CR này chỉ cấp dữ liệu): thẻ lượt trong `ReviewTurnSwitcher` hiện "Agent đã chạy: test (3 lần), lint · Chạy lại: test thất bại (2)".

### 2.5 Che dữ liệu nhạy cảm và quyền riêng tư

- Mọi chuỗi văn bản (evidence, excerpt, tên lệnh) đi qua `secret_redactor.go` của CR-CV-013 2.5 trước khi lưu; che thừa chấp nhận được. Nội dung bị chặn đường dẫn (`.env`, `*.pem`… của CR-CV-013) không bao giờ xuất hiện vì không lưu tham số lệnh hay đường dẫn.
- **Prompt:** mặc định chỉ lưu `prompt_digest`. Lưu `prompt_excerpt` (≤ 160 ký tự, đã che) cần cờ tenant `agent_turn_store_prompt_excerpt` (mặc định `false`, cột mới trong `tenant_settings`, đổi bởi `admin`, có audit). Lý do: đồng nhất với quyết định "không lưu prompt lên backend" của CR-CV-060; digest đủ để biết hai lượt cùng prompt.
- **Không** lưu `lastAssistantMessage` đầy đủ, không lưu transcript, không lưu `interactivePrompt`, không lưu nội dung `Edit`/`Write`.
- Truy cập: đọc cần `quality_read` (CR-CV-085 2.9); ghi `RecordAgentTurn` cần `review_write` (cùng vai trò `SaveReviewState`). Người dùng khác trong cùng project thấy cùng dữ liệu (dữ liệu của worktree, không phải của cá nhân); khác với `review_states.notes` ở chỗ không có nội dung người dùng gõ. Ghi vào audit chỉ `agent_turn:<binding>` khi xoá; ghi bình thường không audit (khối lượng).
- Kênh telemetry (CR-CV-095) không bao giờ nhận `agent_type`, `model`, nội dung, id.

### 2.6 Lưu trữ và giữ bao lâu

Mặc định (đề xuất, chưa hiệu chỉnh): giữ ≤ 200 lượt/binding và 90 ngày (`expires_at = ended_at + 90d`); trường văn bản (`prompt_excerpt`, `claims[].evidence`) bị xoá sau 30 ngày, giữ metadata để xu hướng. Cấu hình `CODEINTEL_AGENT_TURN_RETENTION_DAYS`, `CODEINTEL_AGENT_TURN_TEXT_RETENTION_DAYS`. Xoá theo lô trong công việc bảo trì (cùng cơ chế CR-CV-011 2.5), idempotent. Xoá binding (worktree bị xoá) → xoá lượt của binding (khác `finding_dismissals`: đây là dữ liệu theo worktree). Có RPC `DeleteAgentTurns` (admin) cho yêu cầu xoá dữ liệu — Q5.

### 2.7 RPC, kênh, sự kiện

Mở rộng `orca.codeintel.v1.QualityGateService` (README 3.10 chưa có RPC cho `agent_turns`; ghi vào "Điều chỉnh hợp đồng"):

| RPC | Mục đích | Quyền | Kênh WS (đề xuất) |
|---|---|---|---|
| `RecordAgentTurn` | upsert theo `client_turn_id` (nguồn A/B) | `review_write` | `codeIntel.quality.turn.record` |
| `ListAgentTurns` | `repo_binding_id`, `limit ≤ 50`, `before?` → lượt kèm `claims`, `verification`, `quality_gate` tóm tắt (nối `quality_trend_points`) | `quality_read` | `codeIntel.quality.turns` |
| `GetAgentTurn` | một lượt đầy đủ | `quality_read` | `codeIntel.quality.turn` |

Sự kiện outbox `orca.codeintel.agent_turn.recorded` (mới, chỉ id: `tenant_id`, `repo_binding_id`, `turn_id`, `end_head_commit`) để consumer đối chiếu và CR-CV-085 ghi điểm xu hướng; consumer idempotent theo `processed_events`. Không đẩy lên UI riêng (UI nhận qua push `codeIntel.quality.gateChanged`/`finished`).

### 2.8 Nối frontend với CR-CV-060 (mô tả điểm tiếp, không sửa ở đây)

`use-review-turn-recorder.ts` (CR-CV-060) sau khi tạo `ReviewTurnMarker` gọi thêm `codeIntelClient.call(worktreeId, 'codeIntel.quality.turn.record', {…})` với các trường mục 2.2 (không gửi `prompt`; gửi `prompt_digest` tính cục bộ bằng `crypto.subtle` — hàm thuần, chạy cả Electron và web). Chỉ khi cờ chất lượng bật (CR-CV-085 2.2). Lỗi gọi không chặn UI, thử lại tối đa 3 lần có backoff rồi bỏ (lượt thiếu vẫn hợp lệ).

### 2.9 Cấu trúc file (mới)

Backend `code-intel-service/internal/`: `domain/agent_turn.go`, `agent_command_summary.go`, `agent_claim_extractor.go`, `agent_claim_reconciler.go`; `usecase/record_agent_turn.go`, `reconcile_agent_turn.go`, `agent_turn_maintenance.go`; `adapter/{postgres,mysql}/agent_turn_repository.go`; `adapter/grpc/agent_turn_server.go`; migration `…_agent_turns`. Frontend: `components/review-map/turns/agent-turn-recorder-payload.ts` (hàm thuần dựng payload). Tên theo khái niệm, không `helpers/utils`.

## 3. Quyết định thiết kế

1. **Hai nguồn, một dòng**: không buộc chờ Part A phát hook; nguồn renderer chạy được ngay, nguồn hook bổ sung độ tin cậy sau.
2. **Chỉ tóm tắt lệnh, không lưu tham số**: tránh rò đường dẫn/secret; vẫn đủ để đối chiếu theo category.
3. **Claim `ran_command` ≠ "báo đạt"**: không có mã thoát nên không dựng kết luận không có căn cứ; `stated` tắt mặc định.
4. **Đối chiếu chỉ dựa run độc lập**; cổng không dựa lời agent (nhất quán O13 cho phần AI).
5. **Neutral wording**: "chạy lại khác lời agent nói", không quy kết.
6. **`unverified` là trạng thái thật**, không ép thành `consistent`.
7. **Không lưu prompt mặc định** (đồng nhất CR-CV-060).

## 4. Tiêu chí chấp nhận

- [ ] `RecordAgentTurn` gọi hai lần cùng `client_turn_id` (nguồn renderer rồi hook) cho **một** dòng, `source=both`, trường đã có không bị ghi đè bằng rỗng; hai dialect cho cùng kết quả.
- [ ] `base_head_commit` của lượt N bằng `end_head_commit` của lượt N−1; lượt đầu là `NULL`.
- [ ] Bộ chuẩn hoá lệnh: bảng ca (`pnpm test --filter x`, `cd /tmp && rm -rf …`, URL, biến môi trường, `curl -H "Authorization: …"`) không chứa đối số/đường dẫn/secret ở đầu ra; ≤ 20 mục; không panic với chuỗi 200 ký tự cắt giữa ký tự UTF-8.
- [ ] Không lưu `prompt_excerpt` khi cờ tenant tắt; bật thì ≤ 160 ký tự và đã qua che bí mật; `prompt_digest` luôn có, không đảo ngược.
- [ ] Claim `ran_command` không bao giờ có `confidence` cao hơn `medium`; claim `stated` không sinh khi `agent_claim_text_enabled=false`.
- [ ] Đối chiếu: run độc lập `failed` có lỗi test + claim test → `contradicted`; không có run → `unverified`; run `unknown` do `CODEINTEL_ENV_NOT_READY` → `unverified`; cây bẩn không dấu vân tay → `unverified` (`tree_may_differ`); test bao phủ cả bốn trạng thái.
- [ ] `verification` không bao giờ thay đổi `verdict` của cổng khi cờ `contradicted → warn` tắt (mặc định).
- [ ] Bảo trì xoá lượt quá 90 ngày và xoá văn bản quá 30 ngày; chạy lặp lại không đổi kết quả.
- [ ] Quyền: người không có `review_write` không ghi được; người không thuộc project nhận `CODEINTEL_NOT_AUTHORIZED`; cờ chất lượng tắt → `CODEINTEL_QUALITY_GATE_DISABLED`.
- [ ] Không có `agent_type`, `model`, id lượt trong payload telemetry (CR-CV-095).
- [ ] Phần frontend nối `use-review-turn-recorder` chạy được ở Electron và web; lỗi gọi RPC không làm hỏng ghi mốc lượt cục bộ của CR-CV-060.

## 5. Kiểm thử

- **Unit**: chuẩn hoá lệnh (fixture từ đầu ra hook thật của Claude/Codex khi có), trích claim, bộ đối chiếu (bảng), bảo trì theo hạn; fuzz ngắn cho bộ chuẩn hoá lệnh.
- **Repository (integration, hai dialect)**: upsert đồng thời hai nguồn, duy nhất, chỉ mục, UTF-8.
- **Hợp đồng**: fixture JSON của `claims`/`verification`; so frontend/Go.
- **Spike bắt buộc trước khi viết mã**: ghi lại thật các envelope `agent.hook` của một phiên Claude và một phiên Codex ở cả hai đường (renderer-launched và backend-spawned) để xác nhận: có `state=done` không, `toolName/toolInput` ở từng lần dùng tool không, `lastAssistantMessage` độ dài, và Part A có phát không. Kết quả quyết định có làm nguồn B và `ran_command` hay không (mục 6).
- **E2E (CR-CV-073)**: agent giả báo "test pass" + run độc lập thất bại → UI hiện "khác với lời agent nói".

## 6. Rủi ro và điểm chưa kiểm chứng

- **Nguồn hook Part A chưa có** (1.2): nếu không có thì chỉ nguồn renderer, `commands_summary` rỗng khi renderer đóng, và đối chiếu chỉ còn `stated`. Đây là **điểm chưa kiểm chứng quan trọng nhất**.
- Không có mã thoát và lịch sử tool: "agent tự báo" thực chất chỉ là `ran_command` + văn bản xem trước; độ chính xác của `stated` chưa đo (đa ngôn ngữ, bị cắt).
- Hook có thể bị giả mạo bởi tiến trình agent (1.2 điểm 3): dữ liệu là bằng chứng ghi nhận, không phải xác thực.
- Model chỉ biết với phiên do backend spawn hoặc qua AI Vault local; nhiều lượt sẽ `model=NULL`.
- `AgentType` là chuỗi tự do (17 nguồn hook + agent tuỳ ý): bộ so khớp lệnh theo category phụ thuộc cách mỗi CLI gọi shell (tên tool khác nhau).
- Lượt có thể kéo dài qua nhiều commit hoặc kết thúc khi chưa commit; chuỗi `base/end` có thể bỏ qua commit người dùng tự làm giữa hai lượt.
- Chi phí lưu và quyền riêng tư của dữ liệu lượt chưa được đội bảo mật duyệt; giá trị giữ 30/90 ngày là đề xuất.
- Cần chốt `QualityRun.source`, `errorCode`, `treeFingerprint` ở CR-CV-082 (xem CR-CV-085 Q4).

## 7. Câu hỏi mở

- **Q1.** Làm nguồn B (backend nhận hook) ở CR này hay tách CR riêng cho infra-fleet + agent? Đề xuất: spike trước, quyết sau.
- **Q2.** Bộ gom lệnh nằm ở renderer (hiện thấy mọi cập nhật trạng thái) hay ở infra-fleet? Renderer không thấy khi đóng app; infra-fleet cần giữ trạng thái theo pty.
- **Q3.** Có cho `contradicted` thêm một dòng `warn` vào cổng CR-CV-085 không? Mặc định không.
- **Q4.** Có đọc transcript của CLI trên dev server (đường `providerSession` transcript path, `agent/src/shared/agent-session-resume.ts`) để lấy mã thoát và lời cuối đầy đủ không? Tăng độ chính xác nhưng lộ nhiều dữ liệu hơn; cần quyết định riêng.
- **Q5.** Có RPC xoá dữ liệu lượt theo yêu cầu (quyền admin) không, và có đưa vào hạn mức audit không?
- **Q6.** Có cần liên kết `agent_turns` ↔ `task.execution_links` (agent do task thực thi) ngay không, hay để CR-CV-092?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (3.10 `agent_turns`; mục 8 điểm 15)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (C5, §7)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-060-review-notes-send-to-agent-and-turn-compare.md` (1.2, 2.3, 2.4), `CR-CV-061-review-entry-points.md` (1.1)
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md`, `CR-CV-013-authorization-audit-and-quotas.md` (2.5)
- `/opt/repos/orca/agent/src/shared/agent-hook-relay.ts` (envelope `agent.hook`), `/opt/repos/orca/agent/src/shared/agent-status-types.ts`, `/opt/repos/orca/agent/src/shared/agent-status-field-normalization.ts`
- `/opt/repos/orca/agent/src/relay/agent-hook-server.ts` (token `:300`, cache `MAX_CACHED_PANES`), `/opt/repos/orca/agent/src/relay/relay.ts` (`:554`), `/opt/repos/orca/agent/src/relay/agent-spawner.ts` (`ORCA_PTY_ID` `:242`)
- `/opt/repos/orca/frontend/src/renderer/src/store/slices/agent-status.ts`, `/opt/repos/orca/frontend/src/shared/ai-vault-types.ts`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go` (`agentHookNotificationParams` `:539`), `…/domain/agent_session.go`, `…/migrations/postgres/0019_agent_sessions.up.sql`
- `/opt/repos/orca/backend-go/services/task-service/internal/domain/task.go`, `…/migrations/postgres/0010_execution_links.up.sql`; `/opt/repos/orca/backend-go/services/orchestration-service/migrations/postgres/0001_init.up.sql`
- CR cùng nhóm (chỉ ID): CR-CV-080, 081, 082, 085, 086, 087, 092, 095
- `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`
