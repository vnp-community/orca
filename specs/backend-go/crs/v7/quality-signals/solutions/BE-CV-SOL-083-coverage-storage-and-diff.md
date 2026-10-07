# BE-CV-SOL-083: Lưu và phục vụ coverage / diff coverage (proto, migration `0005`, nạp từ `quality.coverage`, ước lượng, `GetCoverage`)

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phần backend của CR-CV-083; thu thập coverage, parse profile Go/vitest và **tính diff coverage** là phần agent (`AG-CV-SOL-083-coverage-collection`, CR-083 K2). Backend lưu, kiểm tra, ghép, phục vụ và chỉ tính hộ phần "ước lượng".

**CR:** [CR-CV-083](../../../../../../docs/crs/v7/quality-signals/CR-CV-083-coverage-and-diff-coverage.md)
**Service:** `code-intel-service` (mới) · `proto` (`codeintel_coverage.proto`)
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (multi-tenancy, migration), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (cô lập tenant), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (gRPC: deadline mọi lời gọi ra, buf breaking), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

## 0. Hợp đồng áp dụng

| PQ / mục | Áp dụng thế nào |
|---|---|
| H7, PQ-33 | `CoverageReport.source ∈ measured\|estimated`; **không bao giờ** trộn hai nguồn trong một con số; báo cáo `estimated` không có phần trăm câu lệnh; thiếu dữ liệu là `unknown`/`null`, không `0`/`pass` |
| PQ-04 | `GetCoverage` nhận `selector`; bảng khoá theo `repo_binding_id` |
| PQ-14 | Phản hồi ≤ 2 MiB; `payload` cột ≤ 1 MiB và ≤ 2 000 tệp (C-DM T13); gRPC client tới infra-fleet nhận tới 16 MiB (SOL-021) |
| PQ-21 | `quality.coverage` nhận `workspaceRoot`; `runId` lạ → `CODEINTEL_RUN_NOT_FOUND` |
| PQ-27 | RPC `GetCoverage`, kênh `codeIntel.quality.coverage` (C-UI §3.2) |
| C-DM §2.1 #20 | `codeintel_coverage.proto`: `CoverageReport, FileCoverage, DiffCoverage`; `GetCoverage*` |
| C-DM §3.2 | `GetCoverage{run_id? \| head_commit}` → `{report}`; action OPA `quality_read` |
| C-DM §4.2 T13 | Bảng `coverage_reports` (cột, UNIQUE, TTL); migration `0005_coverage_reports` |
| C-DM §4.3 | Bảo trì: `coverage_reports` 30 ngày (hoặc 20 báo cáo/worktree theo CR-083 2.6) |
| C-AG §5.6 | Hình dạng `quality.coverage`; `ENV_NOT_READY reason=coverage_provider_missing`, `RUN_IN_PROGRESS`, `available:false` |
| C-UI §4.7 | `CoverageReport` TS theo PQ-33 (backend là chủ hình dạng) |
| §8.3 | Hai dialect; `tenant_id`; cô lập tenant; agent chỉ gọi `quality.coverage` |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `backend-go/go.work`, `backend-go/Makefile` (`test` lặp từng module; `cmd/orca-cli` bị bỏ), `backend-go/.golangci.yml` (loại `gen`), `.github/workflows/backend-go-scm-integration-service.yml` (ma trận `dialect: [postgres, mysql]`, `go-version: "1.25"`), `backend-go/common/dbcapability/capability.go`, ba file hợp đồng, và CR-083 (đã đối chiếu từng khẳng định về `go.work`: 21 mục trong CR, nhưng `go.work` hiện có **19 service + 3 module chung = 22 dòng `use`, không có `code-intel-service`**, đếm bằng đọc file). Kiểm bằng `ls`: `services/code-intel-service` và `proto/orca/codeintel` chưa tồn tại. Chưa chạy `go test -coverprofile`, chưa chạy migration.

### Correction relative to CR / hợp đồng

| # | CR-083 nói | Hợp đồng / mã thật | Xử lý |
|---|---|---|---|
| C1 | `go.work` có 21 mục `use` (2.2) | Đếm trong `go.work` hiện tại: `common`, `proto`, `cmd/orca-cli` + 19 service; đã chạy `ls services` (19 thư mục) | Số module do agent đọc `go.work` lúc chạy, không cố định ở backend; không dùng con số 21 trong test |
| C2 | `scope_key` = module hoặc gói (2.6) | C-AG §5.6: báo cáo agent **một khối** `totals` + `files[]`, không có totals theo module; `diff.modules[]` chỉ liệt kê tên | `scope_key = 'all'` cho báo cáo một ngôn ngữ của một run; chi tiết module nằm trong `payload.files` (theo tiền tố đường dẫn). Điểm hợp đồng chưa rõ (mục 7) |
| C3 | `language ∈ go\|ts` (T13) và `CoverageReport.language ∈ go\|ts\|mixed` (C-UI) | C-AG §5.6 trả một `language` mỗi `quality.coverage` | Một run có cả `coverage-go` và `coverage-ts` cần hai lần gọi hoặc hai báo cáo: chưa rõ, mục 7. MVP: Go; TS chỉ khi O-9/O12 duyệt |
| C4 | Lưu `estimated` (K5, T13 có `source`) | `quality_run_id uuid NOT NULL` trong T13; ước lượng không có run | Chỉ **lưu** `estimated` khi tồn tại run liên quan (run không có bước coverage hoặc bước `env_not_ready`); ngoài ra tính nóng, không lưu (2.E) |
| C5 | Response chỉ `report` (C-DM §3.2) | C-UI §3.2: `{report: CoverageReport\|null; reason?}` | Proto `GetCoverageResponse` thêm `string reason = 2` (additive); điểm lệch giữa hai hợp đồng |
| C6 | `partial:true` khi `scope:"changed"` | C-AG §5.6 `diff.partial`, `diff.modules`, `diff.noTests` | Lưu trong `payload`; `covered_stmts/total_stmts` chỉ tính phần đã đo |
| C7 | Backend "tính diff" | CR-083 K2: mọi phép tính ở agent | Backend **kiểm lại** `diffCoverage = covered/(covered+uncovered)` (null khi mẫu số 0); sai lệch → bỏ báo cáo, `CODEINTEL_RESULT_INVALID` |
| C8 | Giữ 30 ngày hoặc N=20 báo cáo/worktree | C-DM §4.3: 30 ngày | Làm cả hai (cap 20/binding); cap là bổ sung của CR, hợp đồng không cấm |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_coverage.proto
backend-go/services/code-intel-service/
  migrations/{postgres,mysql}/0005_coverage_reports.{up,down}.sql
  internal/domain/coverage_report.go           # CoverageReport, FileCoverage, DiffCoverage, CoverageSource
  internal/domain/coverage_validation.go       # kiểm tra nhất quán số liệu, tính lại diffCoverage
  internal/domain/coverage_payload_trimming.go # cắt payload theo pct thấp trước
  internal/usecase/coverage_ports.go           # CoverageRepository, AgentCoverageClient, ChangeOverlayReader
  internal/usecase/ingest_coverage.go
  internal/usecase/estimate_coverage.go
  internal/usecase/get_coverage.go
  internal/adapter/postgres/coverage_repository.go
  internal/adapter/mysql/coverage_repository.go
  internal/adapter/agentquality/coverage_decode.go   # cùng package với SOL-082 (agentquality)
```

### B. Proto `codeintel_coverage.proto`

`CoverageReport{ source, language, mode, head_commit, base_commit, dirty, totals, diff, files[], truncated, total_count, tool_versions, estimated_note }`, `FileCoverage{ path, stmts, covered, pct, functions[], uncovered_ranges[] }`, `DiffCoverage{ changed_executable, covered, uncovered, optional diff_coverage, reason, partial, modules[], no_tests[], excluded_files[], unmapped_blocks }`, `GetCoverageRequest{ selector, run_id | head_commit }`, `GetCoverageResponse{ report, reason }`. Số field do chủ sở hữu CR gán theo thứ tự khai báo (C-DM §2 quy tắc: CR-083 chưa ghi số) và **không đổi về sau**. `pct` và `diff_coverage` là `optional double` (vắng = không biết, H7). `uncovered_ranges` là `repeated LineRange{from,to}`. Dòng `rpc GetCoverage` thêm vào `service QualityGateService` ở `codeintel_quality_gate.proto` (file do SOL-085 tạo; PR này chờ hoặc phối hợp, mục 8).

### C. Migration `0005_coverage_reports`

Cột và UNIQUE đúng C-DM T13: UNIQUE `(tenant_id, repo_binding_id, head_commit, dirty, tree_hash, scope_key, source)`, chỉ mục `(tenant_id, repo_binding_id, created_at)`. Kiểm tay độ dài khoá MySQL (utf8mb4): 36·4+36·4+64·4+1+80·4+255·4+16·4 ≈ 1 949 byte < 3 072 (**tính tay, chưa chạy**). Postgres: schema `codeintel`, `ENABLE`+`FORCE` RLS, policy `tenant_isolation`, policy bảo trì hẹp. `payload` `JSONB`/`JSON`; CHECK `payload_bytes ≤ 1048576`, `source ∈ (measured, estimated)`, `language ∈ (go, ts)`, `mode ∈ (set, count, atomic, v8)` (Postgres; MySQL kiểm ở domain). Upsert: Postgres `ON CONFLICT (…) DO UPDATE`, MySQL `ON DUPLICATE KEY UPDATE`.

### D. `IngestCoverage`

Kích hoạt khi một run kết thúc (sau `Finish` của `IngestQualityRun`, qua `orca.codeintel.quality.run_finished` hoặc gọi trực tiếp cùng process; chọn gọi qua sự kiện để tách lỗi). Điều kiện: run `succeeded|failed` có bước `kind=coverage` (từ `steps` của run). Quy trình:

1. `quality.coverage{workspaceRoot, runId}` qua `AgentCoverageClient`. Kết quả `available:false, reason:no_coverage_step` → không ghi gì. `ENV_NOT_READY (coverage_provider_missing)` → ghi nhận ở run (`steps[].envReason`), **không** tạo báo cáo số 0.
2. Kiểm tra: `head_commit` của báo cáo = `quality_runs.head_commit`; `dirty` khớp `dirty_fingerprint`; số liệu nhất quán (`covered ≤ stmts`, `pct` khớp `covered/stmts` sai số 1e-6, `diffCoverage` khớp C7); đường dẫn `files[].path` tương đối, không `..`; lỗi → `CODEINTEL_RESULT_INVALID`, run giữ nguyên.
3. Cắt `payload` (domain): ≤ 2 000 tệp, ≤ 1 MiB; thứ tự cắt `pct` thấp trước; đặt `truncated`, giữ `total_count` thật; `uncovered_ranges` chỉ cho tệp đã đổi hoặc `pct < 0.5` (ngưỡng agent; Q6 CR-083).
4. `dirty=true` ⇒ lưu `tree_hash = dirty_fingerprint` của run; `dirty=false` ⇒ `tree_hash=''`.
5. Upsert; phát không sự kiện riêng (kết quả đã có trong `run_finished`). Nếu `quality_trend_points` cần `diffCoverage`, SOL-085 đọc qua `CoverageRepository.LatestByHead`.
6. Nếu `workTreeChangedDuringRun` hoặc HEAD hiện tại khác `head_commit` của run: vẫn lưu, đánh dấu qua `dirty`/`head_commit`; cổng (SOL-085) quyết `stale`.

### E. `EstimateCoverage` (ước lượng) và `GetCoverage`

- Thứ tự: (1) báo cáo `measured` mới nhất cho `run_id` (hoặc `head_commit` + khớp `dirty`) → trả; (2) không có: dựng `estimated` từ `ChangeOverlayReader.Get(selector)` (SOL-036: `uncoveredSymbols`, `ChangedSymbol.tested ∈ yes|no|unknown`) → `totals.changedSymbolsTested/Untested/Unknown`, `diff=null`, `files=[]`, `estimatedNote` là **khoá i18n** (H6) kiểu `coverage.estimated.fromCallGraph`; (3) không có overlay: `report=null`, `reason` = `no_coverage_run` hoặc `overlay_unavailable`.
- `estimated` **không** có `pct`, `stmts`, `covered` (test phản chiếu); CR-085 coi tối đa `warn`.
- Lưu `estimated` chỉ khi có `quality_run_id` (C4). Ngược lại không lưu, `from_cache=false`.
- `GetCoverage` đi qua chuỗi bảo vệ chuẩn của C-DM §3 (guard nội bộ, cờ `quality_gate_enabled`, OPA `quality_read`, phân giải `selector`, kiểm quyền **trước** cache); kết quả ≤ 2 MiB (`proto.Size`); vượt → cắt `files` theo `pct` thấp, `truncated=true`.

### F. Bảo trì

Thêm vào job `withMaintenanceTx` của SOL-011: xoá `coverage_reports` có `expires_at < now()` theo lô 500; giữ tối đa 20 báo cáo mỗi `(tenant, repo_binding)` (xoá cũ nhất). Postgres `DELETE … IN (SELECT … LIMIT n)`, MySQL `DELETE … ORDER BY created_at LIMIT n`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Agent tính, backend kiểm và lưu | CR-083 K2; tránh kéo hai phía nội dung tệp qua SSH |
| `scope_key='all'` | Agent không trả totals theo module (C2); không bịa số |
| `estimated` tách hẳn và không số phần trăm | H7, K5 |
| Chỉ lưu `estimated` khi có run | T13 buộc `quality_run_id NOT NULL` |
| Từ chối báo cáo không nhất quán | Số sai làm cổng sai; thà `unknown` |
| `reason` trong response | C-UI cần để hiển thị "không có phủ" đúng nguyên nhân |

## 4. Tiêu chí chấp nhận

- [x] Migration `0005` up/down/up trên hai dialect; CHECK/domain từ chối `source`/`language`/`mode` sai; UNIQUE chống nhân đôi khi chạy lại cùng `(head, dirty, tree_hash, scope, source)` (upsert đè, `created_at` giữ theo bảng).
- [x] Nạp báo cáo Go mẫu: `totals`, `files`, `diff` đúng; `diffCoverage` null khi `changedExecutable=0` (kèm `reason`), không 100%.
- [x] Báo cáo có `covered > stmts` hoặc `diffCoverage` sai lệch → `CODEINTEL_RESULT_INVALID`, không ghi.
- [x] `payload` > 2 000 tệp hoặc > 1 MiB: cắt theo `pct` thấp, `truncated=true`, `total_count` đúng; phản hồi gRPC ≤ 2 MiB.
- [x] `estimated`: không có trường phần trăm; chỉ ghi khi có run; không bao giờ cộng với `measured`.
- [x] `ENV_NOT_READY(coverage_provider_missing)` **không** tạo báo cáo 0%.
- [x] `GetCoverage` kiểm quyền trước đọc; tenant B không đọc được báo cáo tenant A (cả hai dialect, Postgres RLS bằng SQL trực tiếp).
- [x] Bảo trì xoá > 30 ngày và vượt 20 báo cáo/binding.
- [x] Không tên file `helpers/utils/common/misc`; không `max-lines` disable.

## 5. Kiểm thử (chưa chạy test nào)

- **Unit:** `coverage_validation_test.go` (nhất quán số), `coverage_payload_trimming_test.go` (thứ tự cắt, `truncated`), `ingest_coverage_test.go`/`estimate_coverage_test.go` với cổng giả, bảng ca `GetCoverage` (measured/estimated/null).
- **Integration:** `coverage_repository_contract_test.go` chạy hai dialect (upsert idempotent, RLS, cap 20, TTL), kiểm schema qua `information_schema`.
- **Hợp đồng:** golden JSON `quality.coverage` (G1/CR-070 mở rộng) cho giải mã; `buf lint`/`buf breaking`.
- **Phản chiếu:** `CoverageReport.estimated` không có trường `pct/stmts/covered` có giá trị.

## 6. Rủi ro và chưa kiểm chứng

- Chưa chạy `go test -coverprofile`; hình dạng `quality.coverage` là đề xuất C-AG (chưa chạy); spike S1 của CR-083 là việc agent.
- `scope_key='all'` mất khả năng so sánh theo module; chấp nhận cho tới khi agent trả totals theo module.
- Heuristic "dòng phủ nếu mọi khối phủ" (agent) làm số khác Codecov; UI phải nêu định nghĩa (CR-083 6).
- `ChangeOverlay.uncoveredSymbols` kế thừa báo nhầm của CR-036 (gọi gián tiếp, `vi.mock`).
- Go CI 1.25 vs `go.work` 1.26 (CR-083 1.3): test coverage của chính backend chạy theo CI 1.25; không dùng API chỉ có ở 1.26.
- MySQL: `ON DUPLICATE KEY UPDATE`, độ dài khoá chưa chạy; TiDB chưa kiểm.
- SSH: không đổi; agent chạy trên dev server.
- GitLab/GitHub: không liên quan.

## 7. Điểm hợp đồng thiếu/mâu thuẫn (không tự sửa)

1. C-DM T13 có `scope_key` (module/gói) và UNIQUE theo nó, nhưng C-AG §5.6 không có totals theo module (C2).
2. `CoverageReport.language='mixed'` (C-UI) và `quality.coverage` một ngôn ngữ (C-AG) (C3).
3. C-DM §3.2 `GetCoverage` response thiếu `reason` mà C-UI cần (C5).
4. T13 `quality_run_id NOT NULL` loại trừ báo cáo `estimated` không gắn run (C4).

## 8. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-083-coverage-collection` | Profile `coverage-go|coverage-ts`, parse, **diff coverage**, `quality.coverage` |
| AG | `AG-CV-SOL-081-quality-runner-core`, `...-profile-catalog-and-preflight` | `quality.run`, tên profile, `ENV_NOT_READY` |
| BE | `BE-CV-SOL-082-quality-run-storage-and-ingest` (bảng `quality_runs`, `run_finished`, package `agentquality`), `BE-CV-SOL-036-change-overlay-pipeline` (ước lượng; cổng `ChangeOverlayReader`), `BE-CV-SOL-085-quality-gate-evaluator-and-profiles` (**tạo** `codeintel_quality_gate.proto`; dùng `diffCoverage` cho cổng và `quality_trend_points`), `BE-CV-SOL-040-codeintel-quality-channels` (kênh `codeIntel.quality.coverage`), `BE-CV-SOL-011-repositories-and-maintenance` | Thứ tự C-DM §7.2: `083-BE (sau 082)`; dòng `rpc GetCoverage` cần `codeintel_quality_gate.proto` của SOL-085 |
| FE | `FE-CV-SOL-087-quality-trend-coverage-hotspot` | Tiêu thụ `CoverageReport` (C-UI §4.7, PQ-33); phân biệt `measured`/`estimated` |

## 9. Câu hỏi mở

- **Q1.** Duyệt `@vitest/coverage-v8` (O-9/O12) trước khi có `language=ts`?
- **Q2.** Agent trả totals theo module để `scope_key` có nghĩa, hay bỏ cột khỏi UNIQUE?
- **Q3.** Dòng `rpc GetCoverage`: SOL-083 chờ SOL-085 tạo `codeintel_quality_gate.proto`, hay SOL-085 tạo khung service trước (đề xuất: khung trước, theo G0)?
- **Q4.** Có lưu `estimated` không gắn run bằng một run giả không? Đề xuất: không.
- **Q5.** Khoá lưu cho worktree bẩn: `head_commit + dirty + tree_hash` (đang dùng) hay chỉ đo khi sạch (Q5 CR-083)?

## 10. Tham chiếu

- [CR-CV-083](../../../../../../docs/crs/v7/quality-signals/CR-CV-083-coverage-and-diff-coverage.md) (2.5, 2.6, 2.7, 2.8), [README quality-signals](../../../../../../docs/crs/v7/quality-signals/README.md) mục 6 điểm 4, 9
- Hợp đồng: C-DM PQ-04/14/21/27/33, §2.1 (#20), §3.2, §4.2 (T13), §4.3; C-AG §5.6; C-UI §3.2, §4.7
- `backend-go/go.work`, `backend-go/Makefile`, `.github/workflows/backend-go-scm-integration-service.yml`
- `/opt/repos/orca/AGENTS.md`
