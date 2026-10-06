# BE-CV-SOL-040-codeintel-quality-channels: 20 kênh `codeIntel.quality.*` (`QualityGateService`)

> **Proposed.** Chưa triển khai, chưa chạy test nào. Nhóm kênh cổng chất lượng (PQ-27), mặc định tắt theo tenant (`quality_gate_enabled`).

**CR:** [CR-CV-040](../../../../../../docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md) (mở rộng bởi PQ-27; chủ sở hữu RPC: CR-082/083/085/086/089/090/092/093)
**Service:** `api-gateway` (`internal/adapter/wscompat`)
**TDD tham chiếu:** [`arch/07`](../../../../tdd/architecture/07-security-architecture.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) mục 2

---

## Hợp đồng áp dụng

| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-ui-api.md` | §3.2 (20 kênh), §4.7 (kiểu), §2.3 (`QUALITY_GATE_DISABLED`, `AI_REVIEW_DISABLED`, `PROFILE_*`, `ENV_NOT_READY`, `RUN_*`, `WAIVER_EXPIRY_INVALID`, `SECRET_LEAK_BLOCKED`), §2.4 (cỡ), ghi chú `quality.summary` |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-01 (mã cờ), PQ-13 (timeout), PQ-14 (cỡ), PQ-25 (`quality.ci`), PQ-26, PQ-27, PQ-33, PQ-34, PQ-35, §3.2 (RPC `QualityGateService`: `StartQualityRun, CancelQualityRun, GetQualityRun, ListQualityRuns, ListQualityFindings, WaiveFinding, GetQualityGate, GetQualityProfile, SaveQualityProfile, GetQualityTrend, GetCoverage, GetRequirementTrace, ConfirmRequirementEvidence, LinkWorktreeTask, GenerateReviewSummary, ExportReviewReport, RefreshCiRun, RecordAgentTurn, ListAgentTurns, GetAgentTurn`) |

## Lệch giữa CR và hợp đồng

| # | CR-CV-040 | Hợp đồng | Theo |
|---|---|---|---|
| Q1 | không có kênh `quality.*` | 20 kênh (PQ-27); client `QualityGateServiceClient` riêng | hợp đồng |
| Q2 | một timeout đọc 20 s | `quality.summary`: cột T/o ghi 120 s nhưng chú thích: WS không vượt 25 s; trả `CODEINTEL_TIMEOUT | {"inProgress":true,"retryAfterMs":3000}` sau ≤ 24 s, hoàn tất nền (cache 24 h) | hợp đồng (gateway 24 s) |
| Q3 | cờ chỉ `CODEINTEL_DISABLED` | thêm `CODEINTEL_QUALITY_GATE_DISABLED`, `CODEINTEL_AI_REVIEW_DISABLED` (chuyển nguyên; client `quality-disabled`, `ai-disabled`) | hợp đồng |
| Q4 | `limit` trong ngân sách | `quality.runs ≤ 50`, `findings ≤ 500`, `trend ≤ 200`, `turns ≤ 50`, `report maxFindings/maxReadingSteps ≤ 50` | hợp đồng |
| Q5 | trần `args[0]` 16 KiB cho mọi kênh | `quality.profile.save ≤ 96 KiB`; `trace.confirm|link ≤ 8 KiB`; còn lại 16 KiB | hợp đồng |

## Phụ thuộc chéo khu vực

| Hướng | Solution | Ghi chú |
|---|---|---|
| BE trước | `BE-CV-SOL-040-codeintel-channel-foundation`; stream `quality.*` push do `BE-CV-SOL-040-codeintel-write-and-stream-channels` (TASK-040-21) | |
| BE trước (RPC) | `BE-CV-SOL-082-quality-run-storage-and-ingest`, `BE-CV-SOL-083-coverage-storage-and-diff`, `BE-CV-SOL-085-quality-gate-evaluator-and-profiles` + `-waivers-and-trend`, `BE-CV-SOL-086-ci-run-merge-and-comparison`, `BE-CV-SOL-089-agent-turn-provenance`, `BE-CV-SOL-090-review-report-model`, `BE-CV-SOL-092-requirement-trace`, `BE-CV-SOL-093-ai-review-summary` | đợt 8–9; tới lúc đó giữ placeholder |
| FE sau | `FE-CV-SOL-085-source-control-quality-notice`, 087, 089, 090, 092, 093 | |
| AG | không (chạy kiểm tra là việc `AG-CV-SOL-081-*`, qua service) | |

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: như SOL-040-foundation mục 1 (cùng phiên). Xác nhận: không có kênh/RPC `quality`; `rpcTimeout 8 s`, `invokeTimeout 25 s` (`handler.go:247`); `writeTimeout 5 s` độc lập với `invokeTimeout` (BUG-001, `handler.go` chú thích) nên deadline kênh 24 s vẫn ghi được phản hồi; `ChannelDeps` chưa có `QualityGate` (TASK-040-01 thêm).

### Correction relative to CR-CV-040

| # | CR nói | Thực tế | Xử lý |
|---|---|---|---|
| C1 | (không có) | `quality.summary` vừa cột T/o 120 s vừa chú thích ≤ 24 s | catalog đặt `codeIntelSummaryTimeout = 24 s`; test bất biến `< invokeTimeout` |
| C2 | (không có) | `quality.waive` mô tả `expiresAt` bắt buộc cả khi `action:'revoke'` | gateway chỉ bắt buộc `reason`/`expiresAt` khi `action` là waive (suy luận, Q1) |

## 2. Giải pháp

### 2.1 Bảng tham số và kiểm tra gateway (đầu vào cho task)

Mọi kênh có `(sel)` = `projectId`, `worktreeId`. Kết quả: chủ yếu object phẳng `{run}`, `{gate,...}` (không phong bì `Env<T>`; UI-API 3.2).

| Kênh | Trường | Kiểm gateway | RPC | T/o | Cỡ |
|---|---|---|---|---|---|
| `quality.start` | `profile` (≤ 128), `scope` worktree\|changed\|commitRange, `base?` | `profile` bắt buộc; enum; `base` qua ref | `StartQualityRun` | 8 s | 16 KiB |
| `quality.cancel` | `runId` | `checkOpaqueID` | `CancelQualityRun` | 8 s | |
| `quality.run` | `runId` | idem | `GetQualityRun` | 8 s | |
| `quality.runs` | `limit? ≤ 50`, `source?` local\|ci, `pageToken?` | 1..50 | `ListQualityRuns` | 8 s | |
| `quality.findings` | `runId?`, `severities?`, `categories?`, `file?`, `inScope?`, `limit? ≤ 500`, `pageToken?` | `file` an toàn (`PATH_NOT_ALLOWED`); `severities` ⊂ error\|warning\|info; `categories` ⊂ 10 giá trị §4.7 | `ListQualityFindings` | 20 s | |
| `quality.waive` | `subjectKind`, `subjectKey`, `action?`, `reason`, `expiresAt`, `scope?` | enum; `reason` 1..1000 (`check` >= 20); `expiresAt` RFC 3339; `scope` repo\|binding | `WaiveFinding` | 8 s | |
| `quality.gate` | `base?`, `profileName?`, `turnKey?`, `record?`, `includeWaivedDetail?` | refs, ≤ 128 | `GetQualityGate` | 8 s | |
| `quality.profile.get` | `name?` | ≤ 128 | `GetQualityProfile` | 8 s | |
| `quality.profile.save` | `scope` tenant\|repo, `name`, `definition` (object), `expectedVersion` | `definition` là object JSON; tổng ≤ 96 KiB | `SaveQualityProfile` | 8 s | 96 KiB |
| `quality.trend` | `from?`, `to?`, `limit? ≤ 200`, `groupBy?` commit\|turn | RFC 3339; `from <= to` | `GetQualityTrend` | 8 s | |
| `quality.coverage` | `runId?` hoặc `headCommit?` | không được cả hai | `GetCoverage` | 20 s | |
| `quality.trace` | `base?`, `includeInferred?`, `taskId?`, `turnKey?` | ≤ 128 | `GetRequirementTrace` | 20 s | |
| `quality.trace.confirm` | `requirementKey`, `evidenceKind` change\|test\|check_run\|manual_confirmation, `evidenceRef`, `linkKind` confirm\|reject, `scope?` | bắt buộc/enum | `ConfirmRequirementEvidence` | 8 s | 8 KiB |
| `quality.trace.link` | `taskId` (khoá bắt buộc; rỗng = gỡ) | khoá có mặt | `LinkWorktreeTask` | 8 s | 8 KiB |
| `quality.summary` | `base?`, `profileName?`, `level` metadata\|diff, `dryRun?`, `forceRefresh?`, `locale?` | `level` bắt buộc; `locale` ≤ 16 | `GenerateReviewSummary` | **24 s** | |
| `quality.report` | `base?`, `profileName?`, `turnKey?`, `sections?` (≤ 32), `maxFindings ≤ 50`, `maxReadingSteps ≤ 50`, `includePeople?`, `includeWaiverReasons?` | 1..50 | `ExportReviewReport` | 20 s | |
| `quality.ci` | `force?` | — | `RefreshCiRun` | 20 s | |
| `quality.turn.record` | `clientTurnId`, `agentType`, `model?`, `endedAt`, `startedAt?`, `interrupted?`, `endHeadCommit`, `treeDirtyEnd`, `filesChangedCount >= 0`, `filesDigest`, `promptDigest`, `promptExcerpt?`, `commandsSummary?`, `claims?` | bắt buộc/RFC 3339; không log | `RecordAgentTurn` | 8 s | 16 KiB |
| `quality.turns` | `limit? ≤ 50`, `before?` | 1..50 | `ListAgentTurns` | 8 s | |
| `quality.turn` | `turnId` | opaque id | `GetAgentTurn` | 8 s | |

### 2.2 Quy tắc riêng

- **Cờ**: `QualityGateService` trả `CODEINTEL_QUALITY_GATE_DISABLED` (cờ phụ) hoặc `CODEINTEL_DISABLED` (cờ chính); `CODEINTEL_AI_REVIEW_DISABLED` ở `summary`; `PROFILE_UNKNOWN | {"available":[...]}` khi tắt quét bảo mật. Gateway chuyển nguyên.
- **`quality.summary`**: timeout 24 s; khi hết hạn do gateway: `CODEINTEL_TIMEOUT: ... | {"retryAfterMs":3000,"inProgress":true}` (đã có ở `codeIntelChannelError`); không thử lại ở gateway; `dryRun:true` trả `manifest` không gọi AI.
- **Kiểm tra chạy**: gateway **không** nhận lệnh, `args`, `env`, `cwd`, `timeout` (H3, O11); chỉ tên profile. Khoá lạ bị từ chối bởi giải mã chặt.
- **Riêng tư**: `promptExcerpt`, `commandsSummary`, `claims` là dữ liệu người dùng/agent; không log, không đưa vào lỗi; `includePeople`/`includeWaiverReasons` mặc định false do service.
- **Dịch kết quả**: `QualityRun`, `QualityGate`, ... qua encoder; `QualityGate.verdict`/`GateResult` chữ thường, `unknown` giữ nguyên (không bao giờ thành `pass`, H7); `QualityTrendPoint.metrics` khoá vắng giữ vắng (không điền `0`).

### 2.3 Tệp mới

`channels_codeintel_quality_run.go`, `..._quality_findings.go` (findings, waive), `..._quality_gate.go` (gate, profile), `..._quality_signals.go` (trend, coverage, ci), `..._quality_trace.go`, `..._quality_ai.go` (summary, report), `..._quality_turns.go`, tests, và `registerCodeIntelQualityChannels` được thêm vào `registerCodeIntelChannels`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `quality.summary` 24 s, không 120 s | giới hạn WS 25 s (UI-API ghi chú, PQ-13) |
| D2 | Không phong bì `Env<T>` cho `quality.*` | UI-API 3.2 kết quả là object phẳng |
| D3 | Gateway không giữ mặc định (`record`, `includeInferred`, ...) | service áp; con trỏ để phân biệt vắng |
| D4 | Không nhận lệnh/`env`/`cwd`/`timeout` | O11/H3 |
| D5 | Không retry; không cache | ghi/AI không idempotent; cache ở service |

## 4. Phụ thuộc và thứ tự

TASK-040-23..29 song song (sau 040-07; mỗi task chỉ nối khi RPC có), 040-30 cuối. Thứ tự đợt: 82 → 85 → 86/89/90/92/93.

## 5. Kiểm thử

Mỗi task: bảng validate biên/enum; fake `QualityGateServiceClient`; deadline; ánh xạ mã lỗi đặc thù (`RUN_IN_PROGRESS | {runId,reason}`, `ENV_NOT_READY | {missing[]}`, `WAIVER_EXPIRY_INVALID | {maxDays}`, `VERSION_CONFLICT`); tenant (metadata); `DeviceID` bị từ chối; không khoá snake_case; mảng rỗng `[]`. TASK-040-30: bộ kiểm chung và tổng 46 kênh. Chưa chạy bất kỳ test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Proto `codeintel_quality*.proto` chưa tồn tại (số field/tên do CR-082/085/... gán).
- `quality.start` chạy lệnh trên dev server: quyền `review_write`, hạn mức và `ENV_NOT_READY` do service/agent; gateway chỉ chuyển.
- `quality.summary` đồng bộ tối đa 24 s có thể vẫn hay `TIMEOUT` lúc nguội; client thử lại (PQ-13).
- Waive: quyền theo vai trò (`member` ≤ 7 ngày, không `check`/`error`) là của service (CR-013/085).
- SSH: không chạm dev server; mọi chạy kiểm tra đi qua service → infra-fleet → agent.

## 7. Câu hỏi mở

- **Q1.** `quality.waive` với `action:'revoke'`: có cần `reason`/`expiresAt`? (suy luận: không.)
- **Q2.** `quality.coverage` không có `runId` lẫn `headCommit`: mặc định run mới nhất (service) — xác nhận.
- **Q3.** Giới hạn độ dài `profile`, `name`, `turnKey`, `clientTurnId`, `agentType` không nêu trong hợp đồng: dùng 128 (quyết định solution).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` (§3.2, §4.7, §2.3)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-01/13/14/25/26/27/33/34/35, §3.2)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`
