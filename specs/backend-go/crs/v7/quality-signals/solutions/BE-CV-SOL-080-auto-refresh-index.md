# BE-CV-SOL-080: Tự làm mới index khi agent xong lượt: sự kiện, debounce bằng `reindex_jobs`, kế hoạch làm mới, `IndexBasis`

> **📋 Proposed.** Chưa triển khai, chưa chạy build/test/migration nào. Phần ghi "(mới)" là đề xuất; khẳng định về code hiện có do người soạn đọc ngày 2026-10-06 (mục 1). Phần backend của CR-CV-080; phần agent là `AG-CV-SOL-080-index-basis-and-reindex-triggers`.

**CR:** [CR-CV-080](../../../../../../docs/crs/v7/quality-signals/CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md)
**Service:** `code-intel-service` (mới ở series v7, do `BE-CV-SOL-010` dựng) · `infra-fleet-service` (một thay đổi payload) · `proto` (`codeintel_index_basis.proto`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (quy tắc phụ thuộc: use case chỉ phụ thuộc cổng), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (multi-tenancy, outbox, migration), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (quy ước sự kiện: subject `orca.<service>.<entity>.<event>`, consumer idempotent, deadline mọi lời gọi ra), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (backpressure, mẫu chịu lỗi), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) §3, §5 (AgentSession)

## 0. Hợp đồng áp dụng

Nguồn: [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md) (viết tắt **C-DM**), [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md) (**C-AG**), [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) (**C-UI**). Solution **không chép lại kiểu dữ liệu**; chỉ trích.

| PQ / mục | Áp dụng thế nào |
|---|---|
| PQ-01, PQ-24 | Cờ hiệu lực = `CODEINTEL_ENABLED ∧ tenant.code_intel_enabled`; `index_policy` (cột `tenant_settings`, C-DM T1) quyết định `auto_in_place|per_worktree|off`; cache cờ ≤ 5 s; **lỗi đọc cờ = tắt** (fail closed). Consumer sự kiện không có "người gọi" nên không trả `CODEINTEL_DISABLED`, chỉ bỏ sự kiện và ghi metric |
| PQ-04 | Binding lấy theo `worktree_id` (UUID do infra-fleet gắn) rồi đối chiếu `repo_bindings` (C-DM T2); không tự tạo binding từ sự kiện |
| PQ-08 | `IndexBasis` ở `codeintel_index_basis.proto` (chủ CR-080, C-DM §2.1 #5, 13 trường theo CR-080 §2.3); `indexScope ∈ exact\|repo_root\|stale\|none`; cột `repo_bindings.index_scope` giữ **vị trí** (`exact\|repo_root\|unresolved`), độ tươi nằm trong `last_status` |
| PQ-16 | `codeintel.reindex` nhận `mode`, `tools`, `trigger`, `ifStale`, `expectHead`; **không có `tiers`**. Tier 1 = `tools:["codegraph"]`, tier 2 = `tools:["gitnexus"]`. Trạng thái job DB: `queued\|running\|succeeded\|failed\|cancelled`; `outcome ∈ already_up_to_date\|superseded\|skipped_scope_repo_root\|""` |
| PQ-17, PQ-18 | Tiến độ/`indexChanged` đến qua `StreamCodeIntelEvents` (SOL-023/024), không thuộc solution này; payload `orca.infra.agent.statusChanged` thêm `worktree_id`, `dev_server_id` (C-DM §2.2, "chủ sở hữu CR-023" nhưng ghi "(CR-080)": solution này làm phần sửa vì CR-080 là CR đòi hỏi) |
| PQ-23 | Biến môi trường tiền tố `CODEINTEL_` |
| PQ-35 | Tín hiệu "agent xong" chỉ dùng `orca.infra.agent.statusChanged`; `codeIntel.hintAgentTurnFinished` là P1 tuỳ chọn; **không** dùng `agent.hook` |
| PQ-37(b) | `codegraph sync` ở worktree liên kết không làm mới: `OVERLAY` dùng diff, **không `analyze`** |
| C-DM §4 T2, T7 | `repo_bindings`, `reindex_jobs` (đã có `trigger`, `trigger_event_id`, `requested_by NULL`, `active_key` UNIQUE, `agent_job_id`); **không bảng mới** |
| C-DM §5 | Subject nhận: `orca.infra.agent.statusChanged` (stream `INFRA`, consumer bền, `processed_events`) |
| C-AG §4.1, §4.10, §6.1 | Chỉ gọi `codeintel.status` và `codeintel.reindex` qua `RelayByDevServer`; agent không nhận lệnh/args |
| §8.3 mục 3, 4 | Hai dialect có test hai dialect; mọi truy vấn có `tenant_id`; test cô lập tenant |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `backend-go/services/infra-fleet-service/internal/{adapter/eventbus/agent_status_publisher.go, adapter/eventbus/publisher.go, usecase/agent_output_classifier.go, usecase/start_agent_session.go (:100-125), usecase/ports.go (:700-715), domain/agent_session.go}`, `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`AgentSession` :1569, `ListAgentSessionsRequest`), `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` (:280-296), `backend-go/common/{eventbus/eventbus.go, outbox/outbox.go, dbcapability/capability.go}`, `backend-go/go.work`, và ba file hợp đồng. Đã kiểm bằng `ls`: **`backend-go/services/code-intel-service/` và `backend-go/proto/orca/codeintel/` chưa tồn tại**; mọi thứ ở code-intel-service là "(mới)" và dựa vào hợp đồng. Chưa chạy gì.

Xác nhận đúng với CR: `statusChanged` publish **trực tiếp** (`p.pub.Publish`, không outbox) và lỗi bị bỏ (`_ = c.publisher.PublishStatusChanged(...)` ở `agent_output_classifier.go:57,96,123`); payload hiện chỉ `{session_id, status}`; stream `INFRA` bắt `orca.infra.>` (`publisher.go` hằng `StreamName`); `domain.AgentSession` **đã có** `WorktreeID`, `DevServerID`; `api-gateway` đẩy nguyên `ev.Payload` tới renderer (`channels_agent.go:291`).

### Correction relative to CR

| # | CR-080 nói | Mã thật / hợp đồng | Xử lý |
|---|---|---|---|
| C1 | "`statusChanged` publish trực tiếp nên at-most-once" | Đúng về phía producer: `js.Publish` vào stream `INFRA` (bền khi NATS đã nhận) nhưng lỗi publish bị nuốt, không thử lại. Phía consumer vẫn là at-least-once | Consumer idempotent bằng `processed_events`; mất sự kiện (lỗi producer) chấp nhận, bù bằng nút thủ công và bước re-plan sau `reindex.finished` |
| C2 | "Thêm `worktree_id`, `dev_server_id` hoặc tra `ListAgentSessions`" (Q4) | Session đã mang hai trường; classifier đã có `session`. Chỉ port `PublishStatusChanged(ctx, tenantID, sessionID, status)` thiếu tham số. C-DM §2.2 chốt: **thêm trường**; không `GetAgentSession`, không tra `ListAgentSessions` (không có lọc theo id, trần 500) | Task 080-02 đổi chữ ký cổng và payload; tương thích ngược (thêm khoá JSON) |
| C3 | Điều kiện kích hoạt: "`status` chuyển từ `running` sang `idle\|completed\|waiting\|error\|stopped`" | Payload không có `previous_status`; thêm trường thứ ba vượt quá C-DM §2.2 | Kích hoạt khi `status ∈ idle\|completed\|waiting\|error\|stopped` **bất kể trạng thái trước**; planner idempotent (index `exact/fresh` thì không làm gì) nên `spawning→idle` ban đầu vô hại |
| C4 | `codeintel.reindex` có `tiers` | PQ-16 bỏ `tiers` | Backend gửi `mode`, `tools`, `trigger:"agent_done"`, `ifStale:true`, `expectHead` |
| C5 | Debounce 20 s/120 s theo binding (không nói lưu ở đâu) | Consumer bền = một con trỏ cạnh tranh giữa các replica; trạng thái debounce trong bộ nhớ sẽ chia đôi theo replica và mất khi khởi động lại | **Slot debounce lưu trong `reindex_jobs`** (`status='queued'`, `trigger='agent_done'`, `active_key = repo_binding_id`), đồng hồ DB; nhiều replica an toàn, khởi động lại an toàn (2.C) |
| C6 | Hạn mức 12 job/giờ/tenant dùng "cơ chế hạn mức CR-CV-013" | Chưa có mã; C-DM §3 chỉ liệt kê quyền, không có API hạn mức | Cổng `AutoRefreshBudget` định nghĩa ở đây; bản mặc định đếm `reindex_jobs` (`trigger='agent_done' AND outcome=''` trong 1 giờ). Cần chỉ mục `(tenant_id, created_at)` (điểm hợp đồng thiếu, mục 7) |
| C7 | `index_policy` ở `repo_bindings.index_policy` hoặc `tenant_settings` (Q2) | C-DM T1 đã chốt `tenant_settings.index_policy` | Theo hợp đồng |
| C8 | Sự kiện dự phòng `agent_completed\|agent_error` đánh dấu "cần kiểm tra lại" | Payload có `connection_id`, **không có `dev_server_id`/worktree**; code-intel-service không có ánh xạ `connection_id→dev_server` | Không làm ở MVP (câu hỏi mở Q2); phạm vi chỉ `statusChanged` |
| C9 | `quality_runs.index_commit`/`index_basis` điền khi kết thúc run | Việc ghi cột thuộc `BE-CV-SOL-082` | Solution này chỉ cung cấp message `IndexBasis` và hàm trạng thái làm mới; SOL-082 dùng cổng `IndexBasisReader` |

## 2. Giải pháp

### A. Cây file (mới, trừ khi ghi khác)

```
backend-go/proto/orca/codeintel/v1/codeintel_index_basis.proto            # IndexBasis (13 trường)
backend-go/services/infra-fleet-service/internal/adapter/eventbus/agent_status_publisher.go   # SỬA payload
backend-go/services/infra-fleet-service/internal/usecase/{ports.go,agent_output_classifier.go} # SỬA chữ ký cổng
backend-go/services/code-intel-service/
  internal/domain/index_refresh_plan.go          # PlanRefresh, RefreshAction, RefreshInputs (hàm thuần)
  internal/domain/index_refresh_state.go         # RefreshStateFromJob (queued|running|deferred|failed|skipped|idle)
  internal/domain/index_dedupe_key.go            # dedupe_key = sha256(binding|head|dirtyFingerprint)
  internal/usecase/auto_refresh_index.go         # AutoRefreshIndex.OnAgentStatus / OnSlotDue
  internal/usecase/auto_refresh_ports.go         # AutoRefreshSlotRepository, AutoRefreshBudget, HostLoadReader...
  internal/usecase/hint_agent_turn_finished.go   # P1
  internal/adapter/postgres/reindex_job_auto_refresh.go   # phương thức slot (không sửa file của SOL-011)
  internal/adapter/mysql/reindex_job_auto_refresh.go
  internal/adapter/eventbus/agent_status_consumer.go      # consumer bền
  internal/adapter/eventbus/auto_refresh_ticker.go        # quét slot đến hạn
  internal/config/auto_refresh.go                         # biến CODEINTEL_AUTOREFRESH_* (mới)
```

Tên file theo khái niệm; không `helpers/utils/common/misc`; không `max-lines` disable (AGENTS.md).

### B. `infra-fleet-service`: payload `statusChanged`

```go
type statusChangedPayload struct {
    SessionID   string `json:"session_id"`
    Status      string `json:"status"`
    WorktreeID  string `json:"worktree_id,omitempty"`   // domain.AgentSession.WorktreeID
    DevServerID string `json:"dev_server_id,omitempty"` // domain.AgentSession.DevServerID
}
// cổng: PublishStatusChanged(ctx, tenantID string, session domain.AgentSession, status domain.AgentStatus) error
```

`omitempty` giữ tương thích với consumer cũ (renderer qua `channels_agent.go:291` chỉ chuyển nguyên payload; trường thêm vô hại). `rateLimited` **không** đổi. Ba điểm gọi trong classifier và `onStartupTimeout` dùng `session` đã có. SSH: `DevServerID` là dev server do infra-fleet phân giải lúc spawn, đúng cả khi dev server nằm sau SSH.

### C. `code-intel-service`: luồng quyết định

```
statusChanged ──▶ [1] cờ+policy ─▶ [2] binding theo worktree_id ─▶ [3] slot (queued, đồng hồ DB)
 (consumer bền,                                                         │ ticker mỗi 5 s, mọi replica
  processed_events)                                                     ▼
                       [4] ClaimDueSlots (CAS) ─▶ [5] codeintel.status ─▶ [6] PlanRefresh ─▶ [7] ngân sách/tải
                                                                                       │
                                               ┌───────────────────────────────────────┼───────────────────────┐
                                               ▼                                       ▼                       ▼
                                   Finish(outcome skipped…/already_up_to_date)   Release(deferred)   codeintel.reindex{trigger:agent_done}
```

**[1] Cờ và chính sách.** Đọc `FlagReader.Effective(ctx, tenant)` (SOL-013/073; cache ≤ 5 s; lỗi = tắt). `index_policy=off` hoặc cờ tắt: bỏ sự kiện, metric `outcome=skipped`. Sự kiện đã nằm trong `processed_events` nên không lặp.

**[2] Binding.** `RepoBindingRepository.GetByWorktreeID(ctx, tenant, worktreeID)` (nếu SOL-011 chưa có thì task 080-04 thêm). Không có binding: bỏ, metric `skipped_no_binding` (chưa ai mở Review nên chưa có `workspaceRoot` đáng tin; không tự `BindRepo`, vì quyền và `selector` thuộc SOL-012). `dev_server_id` của sự kiện khác `repo_bindings.dev_server_id`: bỏ, log cảnh báo (worktree đổi dev server).

**[3] Slot debounce trong `reindex_jobs`.** `UpsertSlot`: `INSERT` job (`status='queued'`, `trigger='agent_done'`, `mode='incremental'`, `requested_by NULL`, `active_key = repo_binding_id`, `trigger_event_id = eventID`). Xung đột `active_key` (UNIQUE): nếu dòng hiện có là `queued ∧ trigger='agent_done'` → **chạm** (`updated_at = now()`, `trigger_event_id`, `version+1`), nghĩa là kéo dài khoảng yên lặng; nếu là `running` hoặc `queued` của người dùng → trả `Busy{jobID,status}` và **không tạo gì** (re-plan sau khi job xong, mục 2.E). Sự kiện `status=running` cùng worktree: `CancelQueuedSlot` (CR 2.2.3 dòng 1); job `running` giữ nguyên (mặc định không kill `analyze`, `CODEINTEL_AUTOANALYZE_CANCEL_ON_RESUME=false`).

**[4] Đến hạn.** Ticker (mọi replica) gọi `ClaimDueSlots(limit, quiet, maxWait)`: slot `queued ∧ trigger='agent_done'` có `updated_at <= now() - quiet` **hoặc** `created_at <= now() - maxWait`, chuyển `running` bằng CAS (`WHERE version=?`), đồng hồ DB (PG `now()`, MySQL `CURRENT_TIMESTAMP(6)`). Truy vấn quét nhiều tenant chạy trong `withMaintenanceTx` (C-DM §4.1); mọi bước sau tải bản ghi theo `(tenant_id, id)`.

**[5] `codeintel.status`.** Cổng `AgentStatusReader` (adapter của collector SOL-021/023) gọi `codeintel.status{workspaceRoot, baseRef}`; kết quả thành `IndexBasis[]` (C-AG §4.1, `classifyIndexBasis` đã ở agent, backend **không** tính lại phân loại; chỉ đọc `indexScope/freshness`).

**[6] `PlanRefresh` (hàm thuần, bảng quyết định).** Đầu vào: `IndexBasis` mỗi công cụ, `policy`, `HostLoad{cores,loadavg1}`, `qualityRunActive`, `lastAnalyzeAt`, `now`, ngưỡng. Đầu ra `RefreshAction`:

| Điều kiện (dừng ở dòng đúng đầu tiên) | Hành động | Kết thúc job |
|---|---|---|
| mọi công cụ `none`/không dùng được | `NoOp(no_index)` | `succeeded`, `outcome=""`, `message=no_index` |
| mọi công cụ `exact ∧ fresh` | `NoOp` | `succeeded`, `outcome=already_up_to_date` |
| công cụ `repo_root` (gốc index ≠ `workspaceRoot`) và `policy=auto_in_place` | `NoOp(repo_root)` (không gửi lệnh) | `succeeded`, `outcome=skipped_scope_repo_root` |
| `policy=per_worktree` | ngoài phạm vi MVP: `NoOp(per_worktree_unsupported)` | `succeeded`, `outcome=skipped_scope_repo_root`, `message=per_worktree_not_enabled` |
| gốc trùng (`exact\|stale`), CodeGraph không `fresh` | `Tier1{tools:["codegraph"]}` | chạy |
| tier 1 thoả và GitNexus `stale`, `changedFilesNotInIndex ≥ MIN_FILES`, đã quá `MIN_INTERVAL` từ lần `analyze` trước | `Tier2{tools:["gitnexus"]}` (sau tier 1) | chạy |
| `loadavg1 > 0,7 × cores` hoặc `qualityRunActive` hoặc trong `MIN_INTERVAL` | `Defer(reason)` | `Release` một lần; lần hai: `cancelled`, `message=deferred` |

`Tier1`+`Tier2` thành **một** lời gọi `codeintel.reindex` (`tools` theo thứ tự cố định codegraph rồi gitnexus ở agent, C-AG §4.10) hoặc hai lời gọi tuần tự; chọn một lời gọi với `tools` gộp để giữ `active_key` một job.

**[7] Ngân sách.** `AutoRefreshBudget.Allow(ctx, tenant)` (mặc định 12/giờ, `CODEINTEL_AUTOREFRESH_PER_HOUR`); vượt: Finish `failed`, `error_code='CODEINTEL_RATE_LIMITED'`, metric; **không xếp hàng**. Tải/`quality.run` đang chạy: dùng `qualityRunActive` từ `quality_runs.status in (queued,running)` của cùng dev server (truy vấn theo `repo_binding_id` của mọi binding trên dev server: cần danh sách binding theo `(tenant, dev_server_id)`, đã có chỉ mục C-DM T2).

**Gọi agent.** `AgentReindexer.Reindex(ctx, target, {mode:"incremental", tools, trigger:"agent_done", ifStale:true, expectHead: headCommit})` → `jobId` lưu `agent_job_id`. Tiến độ/hoàn tất nhận qua luồng sự kiện agent (SOL-024) và cập nhật job bằng cổng `ReindexJobRepository.Finish` của SOL-011/021. Người gọi tự động có `requested_by NULL`; audit ghi `actor_type=system` (cổng `AuditWriter` của SOL-013; `auditclient.Append` hiện thiếu `actor_type`, C-DM §3.3 nên chưa thể ghi đủ: task 080-05 ghi `actor_type` vào `details` cho tới khi SOL-013 sửa).

### D. Sự kiện hoàn tất và re-plan (mục "một lần sau khi xong")

Consumer `orca.codeintel.reindex.finished` (C-DM §5, lọc subject) với `trigger='agent_done'` và không phải `outcome` kết thúc sớm: gọi `codeintel.status`; nếu `freshness ∈ {stale}` ∧ agent không `running` ∧ `dedupe_key` (binding|head|dirtyFingerprint) khác `trigger_event_id` của job gần nhất trong 5 phút → `UpsertSlot` mới (trigger event id = dedupe key). Không cần cờ `pendingRefresh`: trạng thái được suy lại từ index, idempotent.

### E. `IndexBasis` và trạng thái làm mới

`RefreshStateFromJob(job)` (hàm thuần cho SOL-012 dùng): `queued`→`queued`; `running`→`running`; `cancelled ∧ message=deferred`→`deferred`; `failed`→`failed`; `succeeded ∧ outcome ∈ {skipped_scope_repo_root,superseded}`→`skipped`; còn lại `idle`. SOL-012 gắn `refresh_state`, `trigger`, `index_policy` vào `IndexBasis` khi trả `IndexStatus.index_basis` (C-DM §2.3); `OVERLAY` do SOL-012 tính (PQ-08).

### F. Cấu hình (mới; mọi giá trị mặc định là **giả định của CR, chưa đo**)

| Biến | Mặc định |
|---|---|
| `CODEINTEL_AUTOREFRESH_QUIET` | 20 s |
| `CODEINTEL_AUTOREFRESH_MAX_WAIT` | 120 s |
| `CODEINTEL_AUTOREFRESH_TICK` | 5 s |
| `CODEINTEL_AUTOANALYZE_MIN_FILES` | 1 |
| `CODEINTEL_AUTOANALYZE_MIN_INTERVAL` | 10 phút |
| `CODEINTEL_AUTOREFRESH_PER_HOUR` | 12 |
| `CODEINTEL_AUTOANALYZE_CANCEL_ON_RESUME` | `false` (chỉ bật sau phép thử M6, do CR-080 quyết) |
| `CODEINTEL_AUTOREFRESH_LOAD_FACTOR` | 0,7 |

Giá trị sai bị bỏ và log cảnh báo (cùng khuôn agent, C-AG §2.3).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Slot debounce là một dòng `reindex_jobs` | Không bảng mới (C-DM §4.2 chốt danh sách), đồng hồ DB, an toàn đa replica và khởi động lại; `active_key` đã đảm bảo một job mỗi binding |
| Consumer **bền** (cạnh tranh), không `SubscribeEphemeral` | Mỗi sự kiện xử lý một lần cả cụm; `reindex_jobs` là nguồn sự thật. Ephemeral làm N replica cùng tra binding và gọi agent |
| Không `analyze` ở checkout chính vì worktree liên kết đổi | Kết quả không phản ánh code agent (CR-080 1.1, PQ-37b) |
| Agent tự bảo vệ (`skipped_scope_repo_root`) **và** backend không gửi lệnh | Hai lớp: backend tiết kiệm một vòng, agent bảo đảm không lỗi nếu backend lệch |
| Bỏ `tiers`, dùng `tools` | PQ-16 |
| Đếm hạn mức bằng `reindex_jobs` | Không thêm kho trạng thái; cần chỉ mục (mục 7) |
| Chỉ `statusChanged` ở MVP | `agent.hook` không dùng (PQ-35); dự phòng `agent_completed` thiếu ánh xạ |

## 4. Tiêu chí chấp nhận

- [x] `statusChanged` mang `worktree_id`, `dev_server_id` khi session có; test `agent_output_classifier_test.go` cập nhật; renderer không đổi hành vi (payload cũ vẫn hợp lệ).
- [x] `PlanRefresh` có test bảng phủ **cả 7 dòng** ở 2.C[6] và các ca: `indexedCommit` đã bị gc (`stale`), worktree liên kết `fresh_base`, hai công cụ lệch nhau.
- [x] 10 sự kiện `idle` cho cùng binding trong 20 s tạo **đúng một** dòng `reindex_jobs` và đúng một lời gọi `codeintel.reindex` (test hai dialect, hai replica giả).
- [x] Sự kiện trùng `event_id` (giao lặp) không tạo job thứ hai (`processed_events`).
- [x] Worktree liên kết không có index riêng: **không** lời gọi `codeintel.reindex` nào; job kết thúc `outcome=skipped_scope_repo_root`.
- [x] Tenant tắt `code_intel_enabled` hoặc `index_policy=off`: không dòng `reindex_jobs` nào; lỗi đọc cờ = tắt.
- [x] `→ running` trong lúc slot còn `queued`: slot `cancelled`; trong lúc `analyze` đang chạy: **không** gọi `reindexCancel`.
- [x] Vượt `CODEINTEL_AUTOREFRESH_PER_HOUR`: job `failed` `CODEINTEL_RATE_LIMITED`, không gọi agent; job `outcome` khác rỗng **không** tính vào hạn mức.
- [x] Tải cao hoặc `quality.run` đang chạy: hoãn đúng một lần rồi `cancelled(message=deferred)`.
- [x] Mọi truy vấn có `tenant_id`; tenant A không đụng slot/job của tenant B (cả hai dialect; Postgres với role `NOSUPERUSER NOBYPASSRLS`).
- [x] Không tên file `helpers/utils/common/misc`; không `max-lines` disable.

## 5. Kiểm thử (chưa chạy test nào)

- **Unit (Go):** `index_refresh_plan_test.go` (bảng), `index_dedupe_key_test.go`, `auto_refresh_index_test.go` với đồng hồ giả và cổng giả (`AgentStatusReader`, `AgentReindexer`, `AutoRefreshBudget`).
- **Integration (`-tags=integration`, ma trận `dialect: [postgres, mysql]`):** slot `UpsertSlot` đồng thời (UNIQUE `active_key`), `ClaimDueSlots` hai replica (chỉ một thắng CAS), cô lập tenant, đồng hồ DB; NATS testcontainer phát `orca.infra.agent.statusChanged` (mẫu `common/testutil/nats.go` theo CR-080 mục 5, chưa kiểm chứng đường dẫn).
- **Hợp đồng:** `buf lint`, `buf breaking` cho `codeintel_index_basis.proto`; infra-fleet `go test ./...` giữ xanh.
- **Thủ công:** phép thử M1–M7 của CR-080 mục 2.7 là việc của agent/vận hành, không chặn solution này.

## 6. Rủi ro và chưa kiểm chứng

- `worktree_id` của infra-fleet (`AgentSession.WorktreeID`, nguồn giá trị lúc spawn chưa truy ngược) có trùng định danh với `repo_bindings.worktree_id`/`worktree_ref` của PQ-04 hay không: **chưa kiểm chứng** (ba dạng chuỗi). Nếu lệch, mọi sự kiện bị bỏ `skipped_no_binding`; test tích hợp với dữ liệu thật là điều kiện trước khi bật.
- Phiên agent mở thẳng trong terminal (không `StartAgentSession`) không có `statusChanged`; tỷ lệ chưa đo (CR-080 6).
- Thứ tự sự kiện không bảo đảm giữa replica (consumer bền cạnh tranh): `running` có thể đến sau `idle` kế tiếp. Thiết kế dựa trạng thái (slot + status) nên chịu được, nhưng slot có thể bị huỷ sai một lần.
- Debounce 20 s/hạn mức 12/giờ/tải 0,7 là giả định; `analyze` Orca chưa đo.
- `ClaimDueSlots` quét toàn bảng bằng `(status, updated_at)` (đã có chỉ mục C-DM T7); chưa đo ở nhiều tenant.
- Go CI chạy 1.25 trong khi `go.work` ghi 1.26: code mới không dùng API chỉ có ở 1.26 (mỗi `go.mod` service ghi `go 1.25.0`, đã thấy ở các service hiện có).
- **SSH/relay-ssh:** sự kiện chỉ có cho phiên do backend khởi tạo (CR-080 6); chưa kiểm chứng với `relay-ssh`.
- **Git/Provider:** backend không chạy lệnh git; agent chạy `rev-parse`, `merge-base`, `status --porcelain=v1 -z` (dưới 2.25). Không phụ thuộc GitHub/GitLab.

## 7. Điểm hợp đồng thiếu/chưa rõ (không tự sửa hợp đồng)

1. C-DM T2 không có chỉ mục `(tenant_id, worktree_id)` trên `repo_bindings`; truy vấn theo `worktree_id` mỗi sự kiện sẽ quét. Đề nghị thêm vào `0002` trước khi merge (hoặc tra bằng `scope_key='wt:<id>'` kèm `project_id`, nhưng sự kiện không có `project_id`).
2. C-DM T7 không có chỉ mục `(tenant_id, created_at)` cho đếm hạn mức; và không nêu retention cho `reindex_jobs` (mỗi lượt agent có thể sinh một dòng, kể cả no-op). Đề nghị 30 ngày trong C-DM §4.3.
3. C-DM §6.2 không liệt kê các biến `CODEINTEL_AUTOREFRESH_*`; chấp nhận "theo CR" nhưng cần danh sách cuối.
4. `CODEINTEL_RATE_LIMITED` chỉ có trong C-UI §2.3 (lỗi tới client); ở đây dùng làm `error_code` của job.

## 8. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-080-index-basis-and-reindex-triggers` | `codeintel.status` thêm `indexRoot/indexScope/freshness/dirtySinceIndex/changedFilesNotInIndex/mergeBase/host`; `codeintel.reindex` thêm `trigger/ifStale/expectHead`; `outcome=skipped_scope_repo_root` ở worktree liên kết. Backend cần golden fixture G1 (C-DM §7.1) |
| AG | `AG-CV-SOL-004-reindex-and-index-notifications` | Nền `reindex*`, `indexChanged` |
| BE | `BE-CV-SOL-010`, `011-data-model-and-migrations`, `011-repositories-and-maintenance`, `012-target-resolution-and-bindings`, `012-index-status-aggregation`, `013-authorization-flags-and-audit`, `013-agent-call-gate-and-quotas`, `021-agent-collector`, `023-infra-fleet-codeintel-transport`, `024-event-distribution`, `040-codeintel-write-and-stream-channels` (kênh hint), `071-metrics-tracing-and-budgets` | Theo thứ tự §7.2 (`080-BE sau 004 AG, 012, 024`) |
| FE | `FE-CV-SOL-051-review-workspace-shell`, `FE-CV-SOL-087-quality-scorecard-and-state` | Chip index theo `indexScope`/`freshness`/`OVERLAY` (C-UI §4.1). Không có solution FE riêng cho CR-080 |

## 9. Câu hỏi mở

- **Q1.** `worktree_id` của infra-fleet có khớp `repo_bindings.worktree_id` không (mục 6)?
- **Q2.** Có cần đường dự phòng `agent_completed`/`agent_error` (cần `connection_id→dev_server`)? Mặc định: không ở MVP.
- **Q3.** Khi người dùng bấm `RequestReindex` thủ công trong lúc có slot `queued` của `agent_done`: nên nâng slot thành job ngay (SOL-021 gọi `PromoteQueued`) hay trả `CODEINTEL_REINDEX_IN_PROGRESS`? Đề xuất: nâng ngay; cần SOL-021 đồng ý.
- **Q4.** Có đồng ý đếm hạn mức bằng `reindex_jobs` thay vì cổng riêng của SOL-013?
- **Q5.** `per_worktree` (CR-080 2.3.3) giữ ngoài phạm vi tới khi M1–M6 đạt (CR riêng).

## 10. Tham chiếu

- [CR-CV-080](../../../../../../docs/crs/v7/quality-signals/CR-CV-080-agent-worktree-index-strategy-and-auto-refresh.md); [README quality-signals](../../../../../../docs/crs/v7/quality-signals/README.md); [README v7](../../../../../../docs/crs/v7/README.md) mục 2 (O3, O10), mục 8 (điểm 18, 21)
- Hợp đồng: C-DM §1 (PQ-01, 04, 08, 16, 17, 18, 24, 35, 37), §2.1 (#5), §2.2, §3.1, §4 (T1, T2, T7), §5, §6; C-AG §4.1, §4.10, §6; C-UI §4.1
- `backend-go/services/infra-fleet-service/internal/adapter/eventbus/agent_status_publisher.go`, `.../usecase/agent_output_classifier.go`, `.../domain/agent_session.go`, `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/common/eventbus/eventbus.go` (`Subscribe`, `SubscribeEphemeral`), `backend-go/common/outbox/outbox.go`
- `/opt/repos/orca/AGENTS.md` (SSH, Git 2.25, GitLab/provider, max-lines)
