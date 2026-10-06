# BE-CV-SOL-082: Lưu `QualityRun`/`QualityFinding` và nạp kết quả từ agent (proto, migration `0003`, repository hai dialect, `IngestQualityRun`)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phần backend của CR-CV-082; parser, fingerprint, ánh xạ đường dẫn là phần agent (`AG-CV-SOL-082-quality-parsers-and-fingerprint`). Mọi thứ ở `code-intel-service` là "(mới)".

**CR:** [CR-CV-082](../../../../../../docs/crs/v7/quality-signals/CR-CV-082-quality-finding-model-and-parsers.md)
**Service:** `code-intel-service` (mới, do `BE-CV-SOL-010` dựng) · `proto` (`codeintel_quality.proto`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (domain/usecase/adapter), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (multi-tenancy, outbox, migration up/down/up, expand/contract), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (cô lập tenant nhiều lớp, input validation), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (sự kiện at-least-once, idempotent), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (backpressure, deadline)

## 0. Hợp đồng áp dụng

| PQ / mục | Áp dụng thế nào |
|---|---|
| PQ-04 | `quality_runs.worktree_ref` lưu chuỗi worktree đã chuẩn hoá; proto/dây giữ tên `worktree_id` mang chuỗi đó. Request luôn có `selector`; `repo_binding_id` chỉ nằm trong bảng và response |
| PQ-05 | Khoá nghiệp vụ là `repo_id` (cột `quality_runs.repo_id`, `quality_findings.repo_id`); `repo_binding_id` là xuất xứ |
| PQ-06 | `QualityFinding` (công cụ) **tách** khỏi `Finding` (cấu trúc); không trộn danh sách |
| PQ-07, PQ-29 | File `codeintel_quality.proto`, tiền tố `Quality*`; chỉ **message**, RPC của `QualityGateService` ở SOL-085 |
| PQ-16 | Trạng thái run DB: `queued\|running\|succeeded\|failed\|cancelled`. Agent còn `cancelling`, `interrupted` (C-AG §5.3) — ánh xạ ở 2.D |
| PQ-17 | Thông báo `quality.progress|finished` có `workspaceRoot`; code-intel-service gắn `worktreeId` bằng `(tenant, dev_server_id, path_hash)` |
| PQ-21 | Mọi method `quality.*` nhận `workspaceRoot`; `runId` lạ → `CODEINTEL_RUN_NOT_FOUND` |
| PQ-26 | `ruleId` khớp `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`; backend **từ chối** dòng sai thay vì sửa |
| PQ-33 | Số đếm `error/warning/info` trên run là số **trước cắt**; cổng dùng số này |
| C-DM §4.2 T8, T9 | Cột chính xác của `quality_runs` (kể cả cột CI) và `quality_findings`, migration `0003_quality_runs_and_findings` |
| C-DM §4.3 | Bảo trì: `quality_findings` 30 ngày, `quality_runs` 180 ngày, run mồ côi `CODEINTEL_QUALITY_RUN_STALE_AFTER`=60 phút → `failed`/`CODEINTEL_QUALITY_RUN_ORPHANED` |
| C-DM §5 | Sự kiện `orca.codeintel.quality.run_finished` qua outbox cùng giao dịch với `Finish` |
| C-AG §5.2–5.5, §6.3–6.4 | Quy trình agent: `quality.finished` → `runStatus` → `results` phân trang (≤ 500) → **nạp findings trước, `Finish` sau** |
| PQ-14 | Phản hồi ≤ 2 MiB; một trang `ListQualityFindings` ≤ 500 mục |
| §8.3 | Test hai dialect; mọi truy vấn có `tenant_id`; test cô lập tenant; agent chỉ gọi method trong C-AG §4–5 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `backend-go/go.work` (19 module service, không có `code-intel-service`), `backend-go/common/{dbcapability/capability.go, outbox/outbox.go, eventbus/eventbus.go, tenant/tenant.go, grpcmw/grpcmw.go}`, `backend-go/services/scm-integration-service/{migrations/postgres/0001_init.up.sql, internal/adapter/postgres/rate_limit_cache.go, cmd/server/main.go}` (mẫu hai dialect và outbox), `specs/backend-go/crs/v6/request-service-foundation/solutions/BE-REQ-SOL-001-*.md` (mẫu `InTx` và RLS), và ba file hợp đồng. Đã kiểm bằng `ls`: `backend-go/services/code-intel-service/` và `backend-go/proto/orca/codeintel/` **chưa tồn tại**; mọi tên file dưới đây là đề xuất theo layout `arch/03` và C-DM §4.1. Mẫu RLS **chạy thật** chỉ có ở `mcp-service` (`tenant_tx.go`, C-DM §4.1); `scm-integration-service` đặt policy `current_setting('app.tenant_id', true)::uuid` (không `NULLIF`, không `FORCE`) nên không dùng làm mẫu. Chưa chạy gì.

### Correction relative to CR / hợp đồng

| # | CR-082 nói | Hợp đồng / mã thật | Xử lý |
|---|---|---|---|
| C1 | `QualityRun` proto có 18 trường; cột CI "CR-CV-086 có thể thêm, không đặt trước" (2.9) | C-DM §2.1 #18: thêm `error_code=19, tree_fingerprint=20, provider=21, external_ref=22, external_url=23, fetched_at=24, stale_after=25, dirty=26`; C-DM T8: `quality_runs` **đã có** cột CI trong `0003` | Theo hợp đồng: `0003` tạo đủ cột, SOL-086 không `ALTER` |
| C2 | Proto có `dirty_fingerprint=15` | C-DM T8: cột `dirty_fingerprint` = `treeFingerprint` trên dây; §2.1 lại thêm `tree_fingerprint=20` | Hai trường proto cùng giá trị (điền cả hai); **mâu thuẫn nhỏ của hợp đồng**, hỏi chủ sở hữu có bỏ một (mục 7) |
| C3 | Trạng thái run `queued\|running\|succeeded\|failed\|cancelled` | Agent kết thúc có thể `interrupted` (C-AG §6.4) | `interrupted` → DB `failed` + `error_code='CODEINTEL_QUALITY_RUN_INTERRUPTED'` (**suy theo PQ-16 cho job**, mã chưa có trong hợp đồng, mục 7) |
| C4 | `severity`, `category` chuỗi, kiểm ở domain | Đồng ý; CHECK DB cũng có (T9) | Hai lớp kiểm: domain + CHECK (MySQL < 8.0.16 bỏ CHECK nên domain là lớp bắt buộc) |
| C5 | Lọc theo tệp trong run bằng quét ≤ 20 000 dòng (file không chỉ mục) | T9 xác nhận không chỉ mục `file` | Giữ; trần `page_size ≤ 500`, `in_scope` lọc ở SQL |
| C6 | Cắt secret ở agent (`quality-output-redaction.ts`) | H8: không secret trong lỗi/log/cache; backend vẫn phải phòng thủ | Cổng `FindingSanitizer` ở backend (che lần hai, chặn đường dẫn tuyệt đối, `..`); không thay thế che ở agent |
| C7 | Nạp lặp an toàn nhờ UNIQUE | Nhiều replica code-intel-service cùng nhận `quality_finished` (mỗi replica mở một luồng, PQ-18) | Mọi replica có thể cùng nạp: UNIQUE `(tenant_id, run_id, ordinal)` và CAS `Finish` bảo đảm đúng một lần; thêm jitter ngẫu nhiên 0–500 ms và kiểm lại trạng thái run trước khi gọi agent để giảm gọi thừa (chưa đo) |
| C8 | `quality_runs.index_commit`, `index_basis` điền khi kết thúc | `IndexBasis` thuộc SOL-080; nguồn dữ liệu là `codeintel.status` | Cổng `IndexBasisReader.Snapshot(ctx, target)` do SOL-012 cung cấp; lỗi đọc → `index_commit=''`, `index_basis NULL` (không làm run thất bại) |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_quality.proto
backend-go/services/code-intel-service/
  migrations/postgres/0003_quality_runs_and_findings.{up,down}.sql
  migrations/mysql/0003_quality_runs_and_findings.{up,down}.sql
  internal/domain/quality_run.go                 # QualityRun, QualityRunSummary, QualityStepResult
  internal/domain/quality_run_transitions.go     # CanTransition, ánh xạ trạng thái agent -> DB
  internal/domain/quality_finding.go             # QualityFinding + Validate
  internal/domain/quality_finding_limits.go      # hằng giới hạn (5000/20000/2048/...)
  internal/domain/quality_errors.go              # mã CODEINTEL_RUN_*, ErrRunActive...
  internal/usecase/quality_run_ports.go          # QualityRunRepository, QualityFindingRepository, AgentQualityClient, FindingSanitizer, IndexBasisReader
  internal/usecase/ingest_quality_run.go         # IngestQualityRun (+ OnProgress, Reconcile)
  internal/usecase/quality_run_maintenance.go    # MarkOrphans, PurgeExpired
  internal/adapter/postgres/quality_run_repository.go, quality_finding_repository.go
  internal/adapter/mysql/quality_run_repository.go, quality_finding_repository.go
  internal/adapter/agentquality/decode.go        # JSON agent -> domain (nghiêm ngặt)
  internal/adapter/agentquality/client.go        # gọi quality.* qua collector (SOL-021/023)
```

### B. Proto `codeintel_quality.proto`

Nội dung = C-DM §2.1 #18 (số field theo CR-082 §2.1 + 19..26). Không trùng tên (`QualityFinding`, không `Finding`). Test phản chiếu: `QualityFinding` **không** có `line_text`, `source`, `snippet` (CR-082 5). `import` `codeintel_index_basis.proto` (SOL-080) cho `repeated IndexBasis index_basis = 14`. Thứ tự import: `codeintel_quality` ← `codeintel_quality_gate`, `codeintel_coverage`, `codeintel_ci` (C-DM §2.1).

### C. Migration `0003_quality_runs_and_findings`

Cột chính xác theo C-DM T8, T9 (không chép lại ở đây); điểm cần chú ý khi viết:

- Postgres: schema `codeintel`; `ENABLE` + `FORCE ROW LEVEL SECURITY`; policy `tenant_isolation` (`USING` và `WITH CHECK`) theo `NULLIF(current_setting('app.tenant_id', true), '')::uuid`; policy bảo trì hẹp `app.maintenance` cho `SELECT/UPDATE/DELETE` của job dọn (mẫu `mcp-service`, C-DM §4.1).
- `quality_runs.active_key` UNIQUE (`uuid`/`char(36)`, NULL khi kết thúc); UNIQUE `(tenant_id, repo_binding_id, agent_run_id)`; chỉ mục `(tenant_id, repo_binding_id, created_at)`, `(tenant_id, repo_id, head_commit)`, `(status, updated_at)`.
- `quality_findings`: UNIQUE `(tenant_id, run_id, ordinal)`; chỉ mục `(tenant_id, repo_id, fingerprint)`; **không** chỉ mục `file` (khoá InnoDB 3072 byte; C-DM T9).
- MySQL: `CHAR(36)`, `TIMESTAMP(6)`, `JSON`; cột `col`/`end_col` (không dùng `column`); `CHECK` cần 8.0.16. Kích thước hàng `quality_findings` (message 2048 + file 1024 ký tự, utf8mb4 nhân 4) dưới trần 65 535 byte của hàng InnoDB: **tính tay, chưa chạy migration**.
- `down`: Postgres `DROP TABLE quality_findings, quality_runs`; MySQL tương tự (không cần `CASCADE`, không FK). Phải chạy được up→down→up (`arch/05`).

### D. Domain

- `QualityFinding.Validate()`: `fingerprint` khớp `^[0-9a-f]{32}$`; `severity ∈ {error,warning,info}`; `category` thuộc đúng 10 giá trị (C-DM T9); `ruleId` khớp regex PQ-26; `file` rỗng hoặc tương đối (không `/` đầu, không `..`, không `\`, không NUL); `line,end_line,col,end_col ≥ 0`; `message` 1..2048 byte UTF-8 (cắt ở ranh giới ký tự, thêm `…` nếu agent gửi dài hơn — agent đã cắt, đây là phòng thủ); `fp_version ≥ 1`. Vi phạm → bỏ dòng, tăng `invalidDropped`, **không** làm hỏng run (đưa vào `warnings` của kết quả nạp và metric).
- Ánh xạ trạng thái agent → DB: `queued→queued`, `running|cancelling→running`, `succeeded→succeeded`, `failed→failed`, `cancelled→cancelled`, `interrupted→failed(error_code=CODEINTEL_QUALITY_RUN_INTERRUPTED)`.
- `CanTransition`: `queued→running→{succeeded|failed|cancelled}`, `queued→{cancelled|failed}`; terminal bất biến.
- Trần: 5 000 dòng/bước, 20 000/run (C-AG §5.5); backend nhận tối đa 20 000 dòng/run và đặt `findings_truncated` khi agent báo `truncated` hoặc `findings_stored < Σ total_count`.

### E. Cổng và repository (chữ ký rút gọn)

```go
type QualityRunRepository interface {
    Create(ctx context.Context, r domain.QualityRun) error                // đặt active_key; vi phạm -> domain.ErrRunActive (CODEINTEL_RUN_IN_PROGRESS)
    Get(ctx context.Context, tenantID, id string) (domain.QualityRun, error)
    GetByAgentRunID(ctx context.Context, tenantID, bindingID, agentRunID string) (domain.QualityRun, error)
    ListByBinding(ctx context.Context, tenantID, bindingID string, f RunListFilter) ([]domain.QualityRun, string, error) // keyset created_at,id
    ListByRepoCommit(ctx context.Context, tenantID, repoID, headCommit string) ([]domain.QualityRun, error)
    Touch(ctx context.Context, tenantID, id string, status domain.RunStatus) error  // progress, <= 1 lần/2 s ở use case
    Finish(ctx context.Context, f FinishRun) error                          // CAS version; active_key=NULL; cùng giao dịch ghi outbox
    MarkOrphans(ctx context.Context, olderThan time.Duration, limit int) (int, error)   // withMaintenanceTx
    PurgeExpired(ctx context.Context, before time.Time, limit int) (int, error)
}
type QualityFindingRepository interface {
    InsertBatch(ctx context.Context, tenantID, runID, repoID string, fs []domain.QualityFinding) error // idempotent
    ListByRun(ctx context.Context, tenantID, runID string, f FindingListFilter) ([]domain.QualityFinding, string, error) // keyset ordinal; severity,category,file,in_scope
    CountByRun(ctx context.Context, tenantID, runID string) (int, error)
    ListByFingerprint(ctx context.Context, tenantID, repoID, fingerprint string, limit int) ([]domain.QualityFinding, error)
    PurgeExpired(ctx context.Context, before time.Time, limit int) (int, error)
}
```

`Finish` và sự kiện outbox `orca.codeintel.quality.run_finished` (payload C-DM §5) chạy trong **cùng** `InTx` (`TxRunner`/`OutboxWriter` của SOL-010). `InsertBatch`: Postgres `INSERT … ON CONFLICT (tenant_id, run_id, ordinal) DO NOTHING`; MySQL `INSERT … ON DUPLICATE KEY UPDATE id=id`; lô ≤ 500 dòng. Mọi câu lệnh `WHERE tenant_id = ?`; Postgres đặt `set_config('app.tenant_id', $1, true)` mỗi giao dịch. Dọn: Postgres `DELETE … WHERE id IN (SELECT id … LIMIT n)`, MySQL `DELETE … ORDER BY created_at LIMIT n` (CR-CV-011 2.5 theo CR-082 2.9).

### F. `IngestQualityRun` (CR-082 2.10 + C-AG §6.4)

1. **Khởi tạo** do SOL-085 (`StartQualityRun`): `QualityRunRepository.Create` (trạng thái `queued`, `active_key`), gọi `quality.run`, lưu `agent_run_id`. Solution này cung cấp `Create` và test `CODEINTEL_RUN_IN_PROGRESS`.
2. `OnProgress(event)` (sự kiện `quality_progress` từ SOL-024): chuyển `queued→running`, `Touch` tối đa 1 lần/2 s/run; đẩy push `quality.progress` là việc của SOL-024/040.
3. `OnFinished(event)` (sự kiện `quality_finished`) hoặc `Reconcile` (sau `resync`, hoặc bảo trì phát hiện run `queued|running` quá 10 phút còn hoạt động ở agent): jitter, đọc lại run; nếu đã kết thúc thì dừng. Gọi `runStatus` rồi `results{view:findings, limit:500}` phân trang đến `nextOffset=null`, `view:steps` một lần. Với **mỗi trang**: `FindingSanitizer` → `Validate` → `InsertBatch`. Cuối cùng `Finish` bằng CAS.
4. `Finish` điền: `status`, `error_count/warning_count/info_count` (từ `summary` của agent, **trước cắt**), `findings_stored`, `findings_truncated`, `steps`, `index_commit`, `index_basis`, `work_tree_changed`, `scope_widened`, `dirty_fingerprint`, `error_code`, `finished_at`, `active_key=NULL`.
5. `CODEINTEL_RUN_NOT_FOUND` từ agent (hết TTL 1 giờ): run `failed`, `error_code='CODEINTEL_RUN_NOT_FOUND'`, giữ phần findings đã nạp.
6. Run mà agent không phản hồi (`MarkOrphans`): `failed`, `CODEINTEL_QUALITY_RUN_ORPHANED`, `active_key=NULL`.
7. Hạn mức dữ liệu: nhận tối đa 20 000 dòng/run; trang agent ≤ 1 MiB (C-AG §5.5); gRPC client tới infra-fleet đặt `MaxCallRecvMsgSize(16 MiB)` (PQ-14, SOL-021/023).

### G. `AgentQualityClient` và giải mã nghiêm ngặt

`adapter/agentquality/decode.go`: giải mã JSON bằng struct có kiểu, `DisallowUnknownFields` tắt (agent có thể thêm trường, H5) nhưng **bắt buộc** các khoá của C-AG §5.5; số đếm âm, `nextOffset` không tăng, trang quá lớn → `CODEINTEL_RESULT_INVALID` (mã Go sinh, C-AG §3.3). Không tin `file`/`message` của agent: đi qua `FindingSanitizer` (chặn đường dẫn tuyệt đối, `..`, `\`; che mẫu token/DSN, đường dẫn tuyệt đối → `<repo>`).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Nạp findings **trước**, `Finish` **sau**, trong CAS | Người đọc không bao giờ thấy run `succeeded` thiếu phát hiện (CR-082 2.10) |
| Sự kiện outbox cùng giao dịch với `Finish` | `arch/05`, `arch/08`; consumer (SOL-085, 089) idempotent |
| Mọi replica có thể nạp, UNIQUE+CAS chống trùng | Mỗi replica nhận cùng sự kiện (PQ-18); không thêm cơ chế khoá phân tán |
| Bỏ dòng sai thay vì từ chối cả run | Một dòng hỏng không làm mất run; có metric `invalidDropped` |
| Số đếm trên run là số trước cắt | Cổng không "pass nhờ cắt" (CR-082 2.5) |
| `file` không chỉ mục | Giới hạn khoá MySQL |

## 4. Tiêu chí chấp nhận

- [ ] `codeintel_quality.proto` qua `buf lint` và `buf breaking` (gọi `buf` trực tiếp trên CI, không qua `make proto-lint` có `|| true`); `QualityFinding` không có trường chứa dòng nguồn.
- [ ] Migration `0003` up/down/up sạch trên Postgres và MySQL; CHECK từ chối `severity`, `category`, `status`, `source`, `scope` sai (Postgres; MySQL < 8.0.16 từ chối ở domain).
- [ ] Hai `Create` đồng thời cùng binding: một thành công, một `CODEINTEL_RUN_IN_PROGRESS`; sau `Finish` tạo được run mới; `source='ci'` không đặt `active_key` (SOL-086).
- [ ] Gửi `quality.finished` hai lần và ngắt giữa các trang: run kết thúc một lần, không trùng `ordinal`, `findings_stored` khớp số dòng thật.
- [ ] 6 000 dòng trong một bước (agent đã cắt 5 000): `findings_truncated=true`, các số đếm trên run đúng; thứ tự `ordinal` giữ nguyên thứ tự agent.
- [ ] Dòng có `file` tuyệt đối, `..`, `ruleId` sai regex, fingerprint sai độ dài bị bỏ và đếm; `message` chứa token giả bị che; không đường dẫn tuyệt đối ra bảng.
- [ ] `interrupted` từ agent → `failed` + `CODEINTEL_QUALITY_RUN_INTERRUPTED`; run mồ côi quá 60 phút → `failed`, `active_key=NULL`.
- [ ] Tenant A không đọc/ghi được run, finding của tenant B (cả hai dialect, mọi phương thức; Postgres bằng SQL trực tiếp với role `NOSUPERUSER NOBYPASSRLS`).
- [ ] Bảo trì xoá findings > 30 ngày, runs > 180 ngày theo lô 500.
- [ ] Không tên file `helpers/utils/common/misc`; không `max-lines` disable.

## 5. Kiểm thử (chưa chạy test nào)

- **Unit:** `quality_finding_test.go` (bảng hợp lệ/đối kháng: Unicode, CRLF, đường dẫn Windows), `quality_run_transitions_test.go`, `ingest_quality_run_test.go` với `AgentQualityClient` giả (phân trang, trùng, ngắt, `RUN_NOT_FOUND`, đồng hồ giả cho throttle 2 s), `decode_test.go` (khoá thiếu, kiểu sai, trang không tiến).
- **Integration (`-tags=integration`, `dialect: [postgres, mysql]`):** bộ hợp đồng `quality_repository_contract_test.go` (một bộ kịch bản chạy cho hai dialect, khuôn `repository_contract_test.go` của SOL-011), RLS, `active_key` đồng thời, bảo trì theo lô, kiểm schema qua `information_schema`.
- **Proto:** `buf lint`, `buf breaking`; test phản chiếu trường cấm.
- **Cô lập tenant:** mẫu CR-072 cho mọi phương thức repository.
- **Thủ công:** đo thời gian nạp 20 000 dòng và kích thước bảng (chưa có số).

## 6. Rủi ro và chưa kiểm chứng

- Mọi hình dạng JSON từ agent lấy từ C-AG (đề xuất, chưa chạy); G1 (golden `testdata/agent-results/*.json`) là điều kiện trước khi viết `decode.go`.
- Kích thước bảng `quality_findings` (~0,5–0,8 KiB/dòng, **ước lượng CR**) chưa đo; chưa có phân vùng.
- N replica cùng nạp một run: số cuộc gọi agent tăng N lần ở trường hợp xấu; chưa đo, giảm bằng jitter và kiểm lại trạng thái.
- MySQL: `ON DUPLICATE KEY UPDATE id=id`, CHECK, `JSON` chưa chạy; TiDB chưa kiểm.
- Che secret ở backend là lớp thứ hai; mẫu mới có thể lọt (H8); owner gói `secretmasking`/`pathsafety` chưa chốt (O-16), tạm đặt cổng `FindingSanitizer` và adapter nội bộ.
- Go CI 1.25, `go.work` 1.26: không dùng API chỉ có ở 1.26.
- SSH/relay-ssh: không đổi ở backend; `quality.*` không có ở Part B (C-AG §5).
- GitLab/GitHub: không liên quan; `source='ci'` thuộc SOL-086.

## 7. Điểm hợp đồng thiếu/mâu thuẫn (không tự sửa)

1. `dirty_fingerprint` (CR-082) và `tree_fingerprint=20` (C-DM §2.1) trùng nghĩa (C2).
2. Mã `CODEINTEL_QUALITY_RUN_INTERRUPTED` và cách lưu `interrupted` không được hợp đồng định nghĩa (C3).
3. C-DM §4.3 không nêu `quality_runs.source='ci'` giữ bao lâu khác `local`; SOL-086 dùng 180 ngày + trần 20 SHA/binding (CR-086 2.6).
4. `quality_findings` không có cột cho `external_url` của từng phát hiện CI (SOL-086 dùng `external_url` mức run).

## 8. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-082-quality-parsers-and-fingerprint` | Sinh `QualityFinding` (fingerprint, `inScope`, che, cắt) trả qua `quality.results` |
| AG | `AG-CV-SOL-081-quality-runner-core`, `AG-CV-SOL-081-quality-profile-catalog-and-preflight` | `quality.run|runStatus|cancel|results|listProfiles`, thông báo `quality.progress|finished` |
| BE | `BE-CV-SOL-010`, `011-data-model-and-migrations` (số migration, `tenant_settings`, `repo_bindings`), `011-repositories-and-maintenance` (khung bảo trì), `012-target-resolution-and-bindings`, `013-authorization-flags-and-audit`, `021-agent-collector` (gọi `quality.*`, `MaxCallRecvMsgSize`), `023-infra-fleet-codeintel-transport` (thông báo `quality.*`), `024-event-distribution`, `080-auto-refresh-index` (`IndexBasis`), `085-quality-gate-evaluator-and-profiles` (`StartQualityRun`, `ListQualityRuns`, `ListQualityFindings`), `040-codeintel-quality-channels` | Thứ tự C-DM §7.2: `082-BE (sau 011) → 085` |
| FE | `FE-CV-SOL-087-quality-scorecard-and-state`, `FE-CV-SOL-087-quality-diff-annotations` | Tiêu thụ `QualityRun`, `QualityFinding` (C-UI §4.7) |

## 9. Câu hỏi mở

- **Q1.** Bỏ `dirty_fingerprint=15` hay `tree_fingerprint=20` (C2)?
- **Q2.** Mã và ngữ nghĩa cho run `interrupted` (C3)?
- **Q3.** Có cần khoá phân tán để chỉ một replica nạp (nếu đo thấy tốn)?
- **Q4.** `quality_findings` lưu nén theo run khi bảng lớn (Q5 của CR-082)? Mặc định: không ở v7.
- **Q5.** Đánh số migration `0003` phụ thuộc thứ tự merge của SOL-011/085 (CR-082 Q8); người triển khai đọc `ls migrations/postgres` trước.

## 10. Tham chiếu

- [CR-CV-082](../../../../../../docs/crs/v7/quality-signals/CR-CV-082-quality-finding-model-and-parsers.md) (2.1, 2.5, 2.9, 2.10), [CR-CV-081](../../../../../../docs/crs/v7/quality-signals/CR-CV-081-quality-runner-on-agent.md)
- Hợp đồng: C-DM PQ-04/05/06/07/16/17/21/26/29/33, §2.1 (#18), §4.1–§4.3 (T8, T9), §5; C-AG §3, §5.2–5.5, §6.3–6.4; C-UI §4.7
- `backend-go/common/{dbcapability,outbox,eventbus,tenant}`, `backend-go/services/scm-integration-service/migrations/postgres/0001_init.up.sql` (mẫu schema), `specs/backend-go/crs/v6/request-service-foundation/solutions/BE-REQ-SOL-001-scaffold-request-service.md` (mẫu tx/RLS)
