# BE-CV-SOL-085-waivers-and-trend: Miễn trừ có hạn, điểm xu hướng theo lượt, sự kiện `gate_changed`

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Nửa thứ hai của CR-CV-085. Cần [BE-CV-SOL-085-quality-gate-evaluator-and-profiles](./BE-CV-SOL-085-quality-gate-evaluator-and-profiles.md) (migration `0004`, `EvaluateGate`, `QualityGateServer`).

**CR:** [CR-CV-085](../../../../../../docs/crs/v7/quality-gate/CR-CV-085-quality-gate.md) (mục 2.4, 2.6, 2.8, 2.9)
**Service:** `code-intel-service` (mới) · `codeintel_quality_gate.proto` · `policy/orca-authz`
**Hợp đồng:** [CONTRACT-codeintel-proto-and-data-map.md](../../CONTRACT-codeintel-proto-and-data-map.md) (PQ-05, PQ-22, PQ-33; §3.2; §4.2 T5, T11, T12; §4.3; §5), [CONTRACT-codeintel-ui-api.md](../../CONTRACT-codeintel-ui-api.md) (§3.2 `quality.waive|trend|gate`, §4.7, §5 `quality.gateChanged`)
**TDD tham chiếu:** [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (§Cross-service data consistency, §Transactional outbox), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§Audit logging), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (§Event conventions), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

---

## 0. Hợp đồng áp dụng, lệch, phụ thuộc chéo

### 0.1 Hợp đồng áp dụng

| PQ / mục | Áp dụng |
|---|---|
| PQ-05 | `quality_waivers` **tách hẳn** khỏi `finding_dismissals` (T5); khoá nghiệp vụ `repo_id`; `Dismiss` không miễn cổng với `error`/`blockingSeverities` |
| PQ-04 | `WaiveFinding` và `GetQualityTrend` nhận `selector`; service phân giải `selector → repo_binding → repo_id` |
| PQ-22/F5 | `turn_key` = chuỗi mờ = `agent_turns.client_turn_id` = `"${paneKey}:${doneAt}"` |
| PQ-33 | `QualityTrendPoint` theo dạng dây của hợp đồng (§4.7 ui-api): khoá thiếu = không có số, không phải `0` |
| §4.2 T11, T12 | Cột/khoá bảng; giữ 200 điểm/binding và 90 ngày; dọn waiver sau 365 ngày |
| §5 | `orca.codeintel.quality.gate_changed` payload snake_case chuẩn; consumer `quality.run_finished`, `index.changed`, `agent_turn.recorded` |
| §6.3 | `quality_waive`: member tối đa 7 ngày, không miễn `check`/`error`; owner/admin tối đa 30 ngày |
| PQ-01, PQ-03 | `CODEINTEL_WAIVER_EXPIRY_INVALID` (giữ, `data.maxDays`), `CODEINTEL_QUALITY_GATE_DISABLED`, `CODEINTEL_NOT_FOUND` cho id con |
| PQ-11 | Push `quality.gateChanged` đi qua `StreamCodeIntelEvents` (không RPC riêng) — BE-CV-SOL-024 |

### 0.2 Lệch giữa CR và hợp đồng

| # | CR nói | Hợp đồng quyết | Xử lý |
|---|---|---|---|
| L1 | Duy nhất `quality_trend_points (tenant, binding, head_commit, turn_key, profile_ref)` (Q7 bỏ ngỏ) | T12 thêm `source` vào khoá (O-10 chờ duyệt) | Theo T12; upsert `ON CONFLICT (…, source)` / `ON DUPLICATE KEY UPDATE` |
| L2 | Payload `gate_changed` có `tenant_id` | §5: không có (envelope giữ) | Theo §5 |
| L3 | `WaiveFinding(repo_binding_id, scope: repo|binding)` | PQ-04 `selector`; §3.2 request `subject_kind, subject_key, action (WAIVE|REVOKE), reason, expires_at, scope` | Theo §3.2 |
| L4 | Consumer chỉ `run_finished`, `index.changed`, và `GetQualityGate(record=true)` | §5 liệt `agent_turn.recorded` có consumer 085 | Thêm consumer thứ ba (task 085-10): ghi điểm gắn `turn_key` khi lượt được ghi |
| L5 | Thông báo hết hạn "tự mất hiệu lực không cần tác vụ nền" | T11 hiệu lực = `revoked_at IS NULL AND expires_at > now()` (đồng hồ DB) | Theo T11; không job chuyển trạng thái |
| L6 | Waiver `check` ≥ 20 ký tự, lý do ≤ 1000 | ui-api §3.2 cùng | Theo; UTF-8 hợp lệ, không `\u0000` |
| L7 | `CODEINTEL_WAIVER_MAX_DAYS` (đề xuất) | Hợp đồng §6.2 chỉ nói "theo CR" | Giữ biến, mặc định 30, **giá trị khởi điểm chưa hiệu chỉnh**; mức 7 ngày của `member` là hằng trong domain |

### 0.3 Phụ thuộc chéo khu vực

| Hướng | Solution | Dùng gì |
|---|---|---|
| FE đối ứng | `FE-CV-SOL-087-quality-scorecard-and-state` (nút "Miễn khỏi cổng", danh sách waiver) · `FE-CV-SOL-087-quality-trend-coverage-hotspot` (đồ thị `QualityTrendPoint`, khoảng trống cho `unknown`) · `FE-CV-SOL-085-source-control-quality-notice` (làm mới theo push `quality.gateChanged`) · `FE-CV-SOL-060-review-notes-and-turn-compare` (`ReviewTurnMarker.turnId` khớp `turn_key`) | JSON `QualityTrendPoint`, `QualityWaiver`, push `PushGateChanged` |
| AG | Không có (CR-085 không có AG) | — |
| BE | SOL-085-evaluator (nền); BE-CV-SOL-037-structure-findings-and-dismissals (`finding_dismissals`); BE-CV-SOL-082 (sự kiện `run_finished`); BE-CV-SOL-024-event-distribution + BE-CV-SOL-040-codeintel-quality-channels (đẩy `gateChanged`); BE-CV-SOL-089-agent-turn-provenance (event `agent_turn.recorded`) | |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `common/eventbus/eventbus.go` (`Handler`, `Subscribe` durable = competing consumer, `SubscribeEphemeral`, `Event{ID, TenantID, OccurredAt, Version, Payload}`), `common/outbox/outbox.go`, `common/dbcapability/capability.go`, `common/auditclient/client.go`, `services/mcp-service/internal/adapter/postgres/tenant_tx.go`, ba file hợp đồng, CR-CV-085/089 và README feature.

Xác nhận: durable `Subscribe` với cùng `consumerName` là **một nhóm tranh chấp** — đúng cho ghi điểm xu hướng (mỗi sự kiện xử lý một lần toàn cụm); `SubscribeEphemeral` dành cho phát tán tới mọi replica (push UI, BE-CV-SOL-024). `Subscribe` có `awaitStream` chờ stream `CODEINTEL` do `main.go` của service tạo (`EnsureStream`). Chưa tồn tại `code-intel-service`, `processed_events` của service này, hay consumer nào.

### Correction relative to CR-CV-085

| # | CR nói | Mã thật / hợp đồng | Xử lý |
|---|---|---|---|
| C1 | Idempotent theo `processed_events` | Bảng phải do `0001_init` của BE-CV-SOL-010 tạo, PK `(tenant_id, event_id)` | Dùng bảng đó; kiểm `INSERT` + upsert trong **cùng transaction** |
| C2 | `gate_changed` "outbox cùng transaction với ghi trend" | `common/outbox` có `outbox.Store` + ghi `outbox_events` bằng executor của transaction hiện hành (mẫu SOL-001 v6) | Port `OutboxWriter` dùng chung ctx-transaction của SOL-010/011; không tự mở giao dịch thứ hai |
| C3 | "Consumer: gateway đẩy tới UI" | Gateway nhận qua gRPC `StreamCodeIntelEvents`, không đăng ký NATS (hợp đồng §5, "Đường không qua NATS") | Service **tiêu thụ** `gate_changed` của chính mình để đẩy vào luồng `CodeIntelPush{kind="quality_gate_changed"}` (việc của SOL-024); solution này chỉ phát |

## 2. Giải pháp chi tiết

### 2.1 Cây file (mới)

```
internal/domain/quality_waiver.go                  # WaiverView, quy tắc hạn/phạm vi/vai trò, active_key
internal/domain/quality_trend_point.go             # TrendPoint, khoá, delta
internal/domain/quality_gate_changed.go            # quyết định phát sự kiện (so điểm gần nhất)
internal/usecase/waive_quality_finding.go
internal/usecase/record_quality_trend_point.go     # dùng chung cho 3 consumer + record=true
internal/usecase/get_quality_trend.go
internal/usecase/quality_trend_maintenance.go
internal/adapter/{postgres,mysql}/quality_waiver_repository.go
internal/adapter/{postgres,mysql}/quality_trend_repository.go
internal/adapter/eventbus/quality_trend_consumers.go   # 3 consumer durable
internal/adapter/grpc/quality_gate_waiver_trend_server.go  # phương thức của QualityGateServer (WaiveFinding, GetQualityTrend)
```

### 2.2 `WaiveFinding`

Luồng (theo thứ tự chuỗi kiểm của hợp đồng §3): cờ → OPA `quality_waive` → `selector → binding → repo_id` → kiểm đầu vào → ghi → audit.

Quy tắc miễn (domain, hàm thuần, bảng ca):

| Đầu vào | Quy tắc |
|---|---|
| `expires_at` | phải `> now_db` và `≤ now_db + CODEINTEL_WAIVER_MAX_DAYS`; `member` ≤ `now_db + 7d`; vi phạm → `CODEINTEL_WAIVER_EXPIRY_INVALID` + `{"maxDays":N}` |
| `reason` | 1–1000 ký tự, UTF-8 hợp lệ; `subject_kind=check` ≥ 20 |
| `subject_kind=check` | `subject_key` ∈ `checks[].id` của profile hiệu lực; `member` bị từ chối (`CODEINTEL_NOT_AUTHORIZED`) |
| `subject_kind=finding` | tồn tại `quality_findings(tenant_id, repo_id, fingerprint)` mới nhất; `severity=error` và `member` → từ chối; không tồn tại → `CODEINTEL_INVALID_PARAMS` `{"field":"subjectKey","reason":"unknown_subject"}` |
| `subject_kind=structure_finding` | tra qua port `StructureFindingLookup` (SOL-037); thiếu adapter → chỉ `owner/admin` được miễn, ghi `warnings` (hợp đồng chưa định nghĩa; câu hỏi Q2) |
| `scope` | `repo` hoặc `binding` → `scope_key = "repo"` hoặc `"binding:<repo_binding_id>"` |

`WAIVE` = upsert theo `active_key = sha256(tenant|repo|kind|key|scope)` (hex 64): chưa có → chèn; có → cập nhật `expires_at`, `reason`, `created_by`, `version+1` (gia hạn). Hai yêu cầu đồng thời cùng khoá: vi phạm duy nhất (PG `23505`, MySQL `1062`) → thử lại **một lần** bằng nhánh cập nhật; vẫn lỗi → `CODEINTEL_VERSION_CONFLICT`. `REVOKE`: đặt `revoked_at`, `revoked_by`, `active_key=NULL`; đã thu hồi → trả bản ghi hiện có (idempotent); không tồn tại → `CODEINTEL_NOT_FOUND`. `active_key` NULL lặp nhiều hàng hợp lệ ở cả hai dialect (UNIQUE cho phép nhiều NULL).

Hiệu lực tính trong **SQL** bằng đồng hồ DB (PG `expires_at > now()`, MySQL `> CURRENT_TIMESTAMP(6)`), không giờ ứng dụng. Truy vấn `ListActiveWaivers(tenant, repo, binding)` trả ≤ 50 (sắp theo `expires_at` tăng) cho `GetQualityGate`.

Audit: `codeintel.quality.waive`, `codeintel.quality.waive.revoke` qua `auditclient.Append` (`target`: `waiver:<repo>:<kind>:<key cắt 120>`); `reason` ghi vào log `slog` `audit=true`, **không** vào `audit_log`.

### 2.3 Nối miễn trừ và bỏ qua vào cổng

Adapter `WaiverReader`/`DismissalReader` lắp vào `EvaluateQualityGate` (port của SOL-085-evaluator). Quy tắc theo CR §2.4:

| Hành động | Ảnh hưởng cổng |
|---|---|
| `DismissFinding(ignored)` mức `warning`/`info` | không tính vào `warn` |
| `DismissFinding` mức `error` hoặc ∈ `blockingSeverities` | vẫn tính; `reason.code="dismissed_not_waived"` |
| waiver hiệu lực khớp (`fingerprint`/`finding_key`/`check.id`, đúng `scope_key`) | miễn; `reason.code="waived"`, `waivedCount` |
| `REVOKE`/hết hạn | tính lại ngay (hàm thuần đọc lại) |

### 2.4 Điểm xu hướng

- Khoá/ghi theo T12 (có `source`). `RecordQualityTrendPoint(binding, head, base, turn_key, profile_ref, gate, source)`: tính cổng (dùng `EvaluateQualityGate`), upsert điểm; so với **điểm gần nhất của cùng `(binding, profile_ref, source)`** để quyết định phát `gate_changed` (domain `quality_gate_changed.go`): phát khi chưa có điểm hoặc `verdict` khác; khử trùng bằng khoá `(binding, head, profile_ref, verdict, sha256(sorted run_ids))` lưu trong **outbox `event_id`** = UUID v5 của khoá đó (publish `WithMsgID` của `Publisher` khử trùng phía JetStream — đã đọc `Publish(..., WithMsgID(event.ID))` — nhưng chỉ trong cửa sổ trùng lặp của stream; thêm kiểm tra điểm gần nhất ở DB là lớp chính).
- Ba nguồn ghi: (a) consumer `orca.codeintel.quality.run_finished`; (b) consumer `orca.codeintel.index.changed` chỉ khi `stale` khác điểm gần nhất; (c) consumer `orca.codeintel.agent_turn.recorded` (L4) với `turn_key = client_turn_id` tra từ `agent_turns`; (d) `GetQualityGate(record=true)` chỉ ghi **nếu chưa có** điểm cho khoá `(head, turn_key, profile_ref, source)`. Mọi consumer: durable `Subscribe` với `consumerName` cố định (ví dụ `code-intel-service-quality-trend-run-finished`), lọc đúng subject, `INSERT processed_events` + upsert điểm + ghi outbox **một transaction**; `event_id` đã xử lý → bỏ qua.
- `GetQualityTrend`: `limit ≤ 200` (ngoài khoảng → `CODEINTEL_INVALID_PARAMS`, không kẹp ngầm — ui-api §2.4), `from/to`, `group_by (commit|turn)`; trả `points[]` kèm `truncated`, `total_count`; điểm `unknown` giữ nguyên (không nội suy). Delta giữa hai điểm liền kề tính ở frontend từ `counts/metrics` (hợp đồng PQ-33 không có `delta` trong dạng dây; **CR §2.6 có `delta`** — xem Q1).
- Bảo trì (§4.3, `withMaintenanceTx`, lô 500): xoá `quality_trend_points` quá 90 ngày hoặc vượt 200 điểm/binding (giữ mới nhất); xoá `quality_waivers` quá 365 ngày kể từ `expires_at`/`revoked_at`; idempotent.

### 2.5 Sự kiện `gate_changed`

Subject `orca.codeintel.quality.gate_changed`; payload theo §5: `{repo_binding_id, head_commit, base_commit, profile_ref, previous_verdict|null, verdict, run_ids[], turn_key?, evaluated_at}`. Không chứa thông điệp lỗi, mã nguồn, đường dẫn. Dùng chung `common/outbox` + relay của service.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Waiver tách bảng, buộc hạn | PQ-05, F4 |
| D2 | `WAIVE` là upsert theo `active_key` + thử lại một lần | At-least-once, hai dialect cùng ngữ nghĩa |
| D3 | Hiệu lực tính trong SQL bằng đồng hồ DB | Máy ứng dụng lệch giờ không kéo dài miễn trừ |
| D4 | Chỉ phát `gate_changed` khi `verdict` đổi so điểm gần nhất cùng `source` | Tránh nhiễu; local và CI không giẫm nhau |
| D5 | Consumer durable (competing), không ephemeral | Ghi DB một lần toàn cụm |
| D6 | `delta` không đưa vào proto | Hợp đồng PQ-33 chốt dạng dây không có `delta` |
| D7 | `structure_finding` không tra được thì chỉ owner/admin miễn | Fail closed khi không biết mức `error` |

## 4. Tiêu chí chấp nhận

- [ ] `WaiveFinding` hai lần cùng đầu vào → đúng một hàng hiệu lực; gia hạn tăng `version`; hai dialect cùng kết quả.
- [ ] `member`: miễn `error`, miễn `check`, hạn > 7 ngày → bị từ chối; `owner` miễn tối đa 30 ngày; `expires_at` quá khứ hoặc quá hạn → `CODEINTEL_WAIVER_EXPIRY_INVALID` kèm `maxDays`.
- [ ] Waiver hết hạn mất hiệu lực khi đọc, không cần job; `REVOKE` và gia hạn đổi `GetQualityGate` ngay.
- [ ] `DismissFinding` một `error` **không** làm `fail` thành `pass`; `WaiveFinding` thì có và lý do `waived`/`waivedCount` hiện ra.
- [ ] `quality.run_finished` đến hai lần cùng `event_id` → đúng một điểm; `gate_changed` phát khi `verdict` đổi và **không** phát khi không đổi.
- [ ] `local` và `ci` cùng commit/profile → hai điểm, không ghi đè.
- [ ] `GetQualityTrend`: `limit>200` bị từ chối; `truncated/totalCount` đúng; điểm `unknown` không nội suy; `turn_key` khớp `agent_turns.client_turn_id`.
- [ ] Bảo trì: xoá điểm >90 ngày/>200 điểm và waiver >365 ngày; chạy lặp không đổi kết quả.
- [ ] Cờ tắt → `CODEINTEL_QUALITY_GATE_DISABLED`; consumer nội bộ vẫn chạy để không tắc outbox (CR §2.2).
- [ ] Mọi truy vấn có `tenant_id`; test AST + test cách ly tenant hai dialect.

## 5. Kiểm thử

- **Unit:** quy tắc waiver (bảng: vai trò × kind × hạn × severity), `active_key`, quyết định phát sự kiện, giữ-200.
- **Repository (integration, mỗi dialect):** upsert đồng thời hai goroutine (chỉ một hàng hiệu lực), `active_key` NULL lặp, hiệu lực theo đồng hồ DB (không dùng giờ giả ở ứng dụng; dùng `expires_at = now()+1s` rồi chờ), lô xoá, UTF-8.
- **Consumer:** NATS test-container; giao lặp cùng `event_id`; stream chưa có (`awaitStream` chờ).
- **Hợp đồng:** golden `QualityTrendPoint`, `QualityWaiver`, payload `gate_changed`.

## 6. Rủi ro và điểm chưa kiểm chứng

- `finding_key` đổi khi đổi tên/refactor (CR-037) → miễn trừ `structure_finding` "mất" sau refactor; waiver theo `fingerprint` ổn định hơn nhưng phụ thuộc parser BE-CV-SOL-082.
- Chưa kiểm chứng `SubscribeEphemeral`/`Subscribe` với stream `CODEINTEL` chứa cả `review.saved`: consumer **phải** lọc theo subject (hợp đồng §5).
- Cạnh tranh tên: `CODEINTEL_WAIVER_MAX_DAYS` chưa nằm trong bảng env của hợp đồng (§6.2).
- Chưa đo khối lượng điểm/waiver; mọi hạn mức là giá trị khởi điểm chưa hiệu chỉnh.
- `auditclient.Append` nuốt lỗi và thiếu `actor_type` (hợp đồng §3.3); audit miễn trừ có thể mất âm thầm.

## 7. Câu hỏi mở

- **Q1.** CR §2.6 muốn `delta` trong `GetQualityTrend`; hợp đồng PQ-33 không có. Giữ ở frontend (mặc định) hay thêm trường additive?
- **Q2.** `WaiveFinding(structure_finding)`: ai cung cấp `StructureFindingLookup` (SOL-037)? Nếu không có, `structure_finding` chỉ do owner/admin.
- **Q3 (kế thừa O-18).** `member` có miễn `error` không (mặc định không) và ai duyệt `check`.
- **Q4.** Có RPC liệt kê toàn bộ waiver của repo (quản trị) không? Hợp đồng chỉ có `waivers[≤50]` trong `GetQualityGate`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-085-quality-gate.md` (2.4, 2.6, 2.8, 2.9)
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` (PQ-05, §4.2 T5/T11/T12, §4.3, §5, §6.3)
- `/opt/repos/orca/backend-go/common/eventbus/eventbus.go`, `/opt/repos/orca/backend-go/common/outbox/outbox.go`, `/opt/repos/orca/backend-go/common/auditclient/client.go`
