# BE-CV-SOL-037-structure-findings-and-dismissals: `ListFindings`/`DismissFinding`: vi phạm lớp, vòng, hotspot, mã chết, owner, `finding_dismissals`

> **📋 Proposed.** Chưa triển khai, chưa chạy test/CLI. P1 (đợt 5). Phía agent (method `codeintel.structuralFacts`) là solution riêng `AG-CV-SOL-037-structural-facts`. Khẳng định về code hiện có do người soạn đọc ngày 2026-10-06; chưa kiểm chứng ghi rõ.

**CR:** [CR-CV-037](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-037-structure-analysis.md)
**Service:** `code-intel-service` (mới) · `proto/orca/codeintel/v1`
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md): **PQ-04** (`selector`), **PQ-05** (`finding_dismissals` khoá `repo_id`, `disposition`, `reason`, `note`; Dismiss không miễn cổng), **PQ-06** (`Finding`, `origin` 4 giá trị, bảng `rule→kind`), **PQ-12**, **PQ-14**, **PQ-15** (`view=findings`), **PQ-21** (`structuralFacts`), **PQ-24** (cờ `hotspot_window_days`); §2.1 dòng 15, §3.1 (`ListFindings`, `DismissFinding`), §4.2 T1/T5, §6.3 (`read`, `review_write`). [`CONTRACT-codeintel-agent-rpc.md`](../../CONTRACT-codeintel-agent-rpc.md) §4.9 (`structuralFacts`), §2.5 (timeout 55 s/90 s). [`CONTRACT-codeintel-ui-api.md`](../../CONTRACT-codeintel-ui-api.md) §3.1 (`findings`, `dismissFinding`), §4.5 (`Finding`).
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (§Multi-tenancy, §Migration conventions), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (§AuthZ, §Audit logging), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md), [`services/scm-integration-service`](../../../../tdd/services/scm-integration-service.md) (CODEOWNERS hiện có).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/services/scm-integration-service/internal/usecase/codeowners.go` (`CodeownersRule{Pattern, Owners}`, `ParseCodeowners` dòng 19, `MatchOwners` dòng 39, `matchesCodeownersPattern` dòng 75; import `path/filepath`), `agent/src/relay/agent-git-handler.ts` (whitelist có `log`, `shortlog`, `show`; `SHELL_METACHARACTERS` dòng 70), `backend-go/proto/buf.yaml`, `backend-go/common/auditclient` (tồn tại; chưa đọc nội dung), `backend-go/common/dbcapability/capability.go` (tồn tại), `specs/.../CONTRACT-*.md`, đầu ra `solutions/BE-CV-SOL-030-repo-file-access-gateway.md` §B (chữ ký `RepoSourceReader` có `Log(ctx, RepoRef, LogQuery)`, `ReadFile`, `ListDir`). `code-intel-service` chưa tồn tại; bảng `finding_dismissals` mới là đặc tả (hợp đồng §4.2 T5).

### Correction relative to CR-CV-037

| # | CR nói | Hợp đồng / thực tế | Xử lý |
|---|--------|--------------------|-------|
| C1 | `finding_dismissals(repo_binding_id, …)`; `reason` 1–500 bắt buộc | PQ-05: khoá `repo_id` (kèm `repo_binding_id` xuất xứ), thêm `disposition`, `note`; `reason varchar(500) NOT NULL DEFAULT ''`; UI `reason?` tuỳ chọn | Theo hợp đồng: `reason` tuỳ chọn ở RPC (mục 4, L1) |
| C2 | `Finding.introduced: yes|touched|unknown` | PQ-06: đổi tên `origin ∈ introduced|touched|preexisting|unknown`; thêm `kind` do backend điền | Theo hợp đồng |
| C3 | `structuralFacts` "agent khử trùng theo thư mục đích" | Hợp đồng §4.9: `rows[{pair, fromFile, toFile}]` theo cặp tệp, không nói đã khử trùng | Backend **luôn** khử trùng theo `(service, package nguồn, package đích)` |
| C4 | Tham số `kind`, `pathPrefixes?`, `limit`, `offset` | §4.9: thêm `pair`; `pathPrefixes` mặc định `["backend-go/services/"]` | Theo hợp đồng |
| C5 | `ListFindings` trả `sources[]` kèm "trạng thái bộ phát hiện" | `sources[]` là `SourceInfo` (tool/version/commit), không có trường trạng thái | Thêm `repeated DetectorStatus detectors` (mới, additive; mục 4, L2) |
| C6 | `matchesCodeownersPattern` của `scm-integration-service` làm mẫu | dùng `path/filepath` (phụ thuộc hệ điều hành) | Viết lại bằng gói `path` để chạy giống nhau trên Windows/WSL |
| C7 | `git log --since=90.days --no-merges --name-only --format=%x00%H%x09%ae` | Cần email? | Dùng `%an` (tên), **không** lấy email; không `%ae` |
| C8 | Quy tắc `rule` chính thức | PQ-06 liệt kê 10 giá trị gồm `sql.*` (của CR-038) | Bộ phát hiện `sql.*` cắm vào cùng giao diện `Detector` (solution `BE-CV-SOL-038-static-tenant-filter-rule`) |

## 2. Giải pháp

### 2.A Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_findings.proto
backend-go/services/code-intel-service/
  internal/domain/structurefinding/
      finding.go                 # Finding, Evidence, Owner, kind từ rule
      finding_key.go             # "<rule>:<16 hex>" ổn định
      layer_import_rules.go      # 3 quy tắc lớp + loại trừ + gom (service, pkg nguồn, pkg đích)
      import_cycle_findings.go   # SCC fallback + finding từ cycles
      hotspot_scoring.go         # churn, complexity proxy, centrality, percentile, score
      dead_export_findings.go
      owner_resolution.go        # CODEOWNERS (path, luật cuối thắng) + lịch sử shortlog
      git_log_parsing.go         # tách NUL, giải mã đường dẫn C-quoted
  internal/domain/structurefinding/codeowners_matching.go
  internal/usecase/list_findings.go, dismiss_finding.go, finding_detector.go (interface Detector)
  internal/adapter/postgres/finding_dismissal_repository.go     # ON CONFLICT
  internal/adapter/mysql/finding_dismissal_repository.go        # ON DUPLICATE KEY
  internal/adapter/grpc/findings_handler.go
```

### 2.B Proto `codeintel_findings.proto`

Message (hợp đồng §2.1 dòng 15): `Finding, Evidence, Owner`, `ListFindingsRequest/Response`, `DismissFindingRequest/Response`, thêm `DetectorStatus` (mới). `Finding`: `finding_key, rule, kind, severity, title_key, params(map), subject, evidence[], metrics(map<string,double>), owner, scope, origin, confidence, dismissed{by,at,reason,disposition,note}`. `ListFindingsRequest{selector=1, rules, severities, path_prefix, include_dismissed, scope(ALL|CHANGED enum, UNSPECIFIED=0), base_ref, page_size≤200, page_token}`; response `{findings, next_page_token, total_count, dismissed_count, truncated, sources, index_freshness, detectors}`. `DismissFindingRequest{selector=1, finding_key, action(DISMISS|RESTORE), disposition, reason, note}`; response `{finding_key, dismissed, disposition}`. Số field do chủ sở hữu CR gán theo thứ tự khai báo. `severity`, `origin`, `confidence`, `kind` là chuỗi chữ thường (PQ-32).

### 2.C Bộ phát hiện

Giao diện `Detector{Rule() []string; Detect(ctx, in DetectInput) ([]Finding, error)}`; mỗi bộ chạy độc lập, timeout riêng, lỗi một bộ ⇒ `DetectorStatus{name, state: ok|failed|timeout|skipped, code}` và các bộ khác vẫn trả.

1. **Lớp hexagonal** (`structuralFacts{kind:"layerImports"}`): cặp `usecase->adapter` (warning), `domain->usecase|adapter` (error), `adapter->adapter` (info). Áp cho `backend-go/services/<svc>/internal/**`; loại `_test.go`, `usecasetest/`, `testutil`, `cmd/`, `proto/gen`, `*.pb.go`. Gom theo `(service, package nguồn, package đích)` thành **một** finding; `evidence` = các tệp nguồn (không số dòng: cạnh `IMPORTS` không có dòng). Kết quả mong đợi trên Orca (đã đọc `ports.go:18`, `get_terminal_agent_status.go:10` ở infra-fleet; `ports.go:17`, `test_connection.go:8` ở ai-provider): 2 finding `layer.usecase-imports-adapter`, 0 `layer.domain-imports-outer`, `usecasetest/harness.go` bị loại.
2. **Vòng** (`kind:"cycles"`): mỗi phần tử `cycles[]` (bỏ tệp lặp ở cuối) là một finding `cycle.import` (warning), `evidence` = tệp sắp từ điển, nêu độ dài. Fallback tự tính Tarjan trên `ModuleGraph` chỉ khi agent không có method **và** đồ thị nhỏ (≤ 500 nút).
3. **Hotspot** (`rule hotspot.file`, info): cửa sổ `N = tenant_settings.hotspot_window_days` (mặc định 90, 30..365). `churn(f)` = số commit không-merge chạm `f`, **bỏ** commit chạm > 100 tệp và tệp nhiễu (`pnpm-lock.yaml`, `package-lock.json`, `go.sum`, `i18n/locales/*.json`, `proto/gen/**`, `**/*.md`, `docs/**`, `specs/**`); `complexity` = proxy độ dài `0,5·pr(totalLines)+0,5·pr(longestSymbol)` (từ `fileSizes`; nhãn "độ dài hàm", không gọi là cyclomatic); `centrality = pr(importInDegree)`; `score = pr(churn)·complexity·centrality`; vào danh sách khi `churn ≥ 3` và `score ≥ 0,05`; ≤ 50 tệp, giảm dần. `git log` qua `RepoSourceReader.Log` (đã có `LogQuery`), một lần cho cả cửa sổ; vượt 8 MiB hoặc 60 s ⇒ chia hai khoảng và `confidence:"low"`.
4. **Mã chết** (`dead.unused-export`, info, `confidence:"medium"`): chỉ Go, chỉ `Function`, từ `unusedExports`; loại `_test.go`, `/cmd/`, `usecasetest`, `proto/gen`; ngôn ngữ khác tắt mặc định.
5. **Owner**: có CODEOWNERS (đọc `CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS`, `.gitlab/CODEOWNERS` qua `ReadFile`) ⇒ luật cuối thắng, `source:"codeowners"`; không có (hiện trạng Orca, theo CR đã tìm) ⇒ `git shortlog -sn --since=<N>.days --no-merges HEAD -- <path>` (**bắt buộc `HEAD`**: thiếu thì `shortlog` đọc stdin; cơ chế đóng stdin chưa kiểm chứng qua `git.exec` thật), owner = tác giả ≥ 50% commit, không ai đạt ⇒ top 2; `source:"history"`, **không email**. Section GitLab `[Tên]` bỏ qua + cảnh báo.
6. **`finding_key`** = `"<rule>:<16 hex đầu sha256(canonicalSubject)>"` (≤ 128 ký tự): `layer.*` ⇒ `<service>::<pkg nguồn>::<pkg đích>`; `cycle.import` ⇒ tệp sắp từ điển nối `\n`; `hotspot.file` ⇒ đường dẫn; `dead.unused-export` ⇒ `SymbolRef.key`. Đổi cách chuẩn hoá ⇒ đổi tên `rule`.
7. **`origin`**: `scope=CHANGED` hoặc có overlay: tệp bằng chứng ∈ `changedFiles` ⇒ `introduced` nếu tệp `added` hoặc (có snapshot `findings` ở `mergeBase` và key vắng); `touched` nếu snapshot cũ có key; `unknown` nếu thiếu snapshot cũ (và tệp không `added`); ngoài `changedFiles` ⇒ `preexisting`; overlay không có ⇒ `unknown`.

### 2.D `finding_dismissals` và `DismissFinding`

Bảng theo hợp đồng T5 (do `BE-CV-SOL-011-data-model-and-migrations` tạo; solution này **không** thêm migration): `UNIQUE (tenant_id, repo_id, finding_key)`. Repository `FindingDismissalRepository` (port ở `usecase`): `Upsert`, `Delete`, `ListByKeys(tenant, repo, keys[≤200])`, `CountByRepo`. SQL mọi câu có `tenant_id`; Postgres `INSERT … ON CONFLICT (tenant_id, repo_id, finding_key) DO UPDATE SET disposition, reason, note, dismissed_by, at, repo_binding_id`; MySQL `INSERT … ON DUPLICATE KEY UPDATE …`; Postgres chạy trong `withTenantTx` (`set_config('app.tenant_id')`, mẫu `mcp-service/internal/adapter/postgres/tenant_tx.go`). `DISMISS` = upsert (idempotent), `RESTORE` = xoá dòng (không có dòng vẫn thành công). Quyền `review_write`; dismiss áp cho **cả repo** (khoá `repo_id` từ `selector`→binding). Audit qua `auditclient` (CR-013) kèm `finding_key`, `disposition`, không mã nguồn. Validate: `finding_key` khớp `^[a-z][a-z0-9.\-]{0,63}:[0-9a-f]{16}$`, `reason`/`note` ≤ 500, `disposition ∈ ignored|resolved`. Dismiss **không bao giờ** miễn cổng chất lượng (PQ-05; việc của `BE-CV-SOL-085-*`).

### 2.E `ListFindings`

Tính theo bộ phát hiện (song song có hạn mức CR-013), kết quả hợp nhất lưu `graph_snapshots(view="findings", head_commit=indexedCommit, params_hash)` (PQ-15) — **không** chứa dismiss (ghép lúc đọc, `ListByKeys` theo trang). Sắp `severity` giảm dần, `rule`, `subject`; `page_size ≤ 200`, ≤ 2 000 finding/lần tính; `total_count`, `dismissed_count` (đếm trong tập đã lọc, không phải số dòng bảng), `truncated`. `include_dismissed=false` mặc định. Index cũ ⇒ hạ `confidence` một bậc, trả `index_freshness`. Quyền `read` kiểm trước cache.

Mã lỗi: `CODEINTEL_DISABLED`, `CODEINTEL_NOT_AUTHORIZED`, `CODEINTEL_INVALID_PARAMS`, `CODEINTEL_TIMEOUT` (nền hoàn tất), `CODEINTEL_AGENT_UNSUPPORTED` (agent chưa có `structuralFacts`: bộ lớp/vòng/hotspot-trung-tâm/mã chết `skipped`, bộ còn lại vẫn chạy), `CODEINTEL_TOOL_UNAVAILABLE`, `CODEINTEL_INDEX_MISSING`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Khử trùng ở backend dù agent có thể đã làm | Hợp đồng không bảo đảm (C3); khử trùng idempotent |
| Chế độ lỗi từng phần (`DetectorStatus`) | Một bộ chậm (`cycles` ~ giây) không làm mất bộ khác |
| Dismiss không nằm trong cache | Dismiss đổi liên tục; snapshot theo commit |
| Khoá dismiss theo `repo_id` | PQ-05: mọi worktree của repo dùng chung phân loại |
| Hotspot ghi rõ proxy độ dài | Không có chỉ số phức tạp thật ở GitNexus/CodeGraph |
| Mã chết chỉ Go/`Function` | Tránh báo nhầm qua interface/JSX/reflection |
| Không email, chỉ tên | Quyền riêng tư |
| Viết lại CODEOWNERS bằng `path`, không import `scm-integration-service/internal` | Không import chéo `internal` (arch/03: không chia sẻ domain); tránh phụ thuộc OS |
| Git: chỉ `log`, `shortlog` (≤ Git 2.25; `--since`, `--no-merges`, `--name-only`, `-sn` đều cũ) | Baseline AGENTS.md; không cần `GitCapabilityCache`; không `-c` trước subcommand (agent cấm), nên đường dẫn có thể bị C-quote ⇒ giải mã |
| GitLab/provider | CODEOWNERS đọc cả `.gitlab/CODEOWNERS`; không có nhánh GitHub-only |

## 4. Lệch giữa CR và hợp đồng

| # | CR-CV-037 | Hợp đồng | Theo |
|---|-----------|----------|------|
| L1 | `reason` bắt buộc 1–500 | PQ-05: `reason varchar(500) NOT NULL DEFAULT ''`, UI `reason?` | Hợp đồng; **báo chủ hợp đồng**: CR-038 §3.7 và CR-037 cùng nói "lý do bắt buộc" — nếu muốn bắt buộc cần sửa hợp đồng (UI `reason?`, DB default) |
| L2 | `sources[]` mang trạng thái bộ phát hiện | `sources[]` là `SourceInfo` | Thêm `detectors[]` (đề nghị bổ sung hợp đồng §3.1 và UI §4.5) |
| L3 | `introduced` | `origin` 4 giá trị | Hợp đồng |
| L4 | Request `repo_binding_id` | `selector` | Hợp đồng |
| L5 | `Finding.dismissed{by,at,reason}` | thêm `disposition`, `note` | Hợp đồng |
| L6 | `scope=CHANGED` mặc định không nói | UI: `scope?` mặc định `changed` | Hợp đồng (UI) |
| L7 | `totalCount` | `total_count`, `dismissed_count` (§3.1) | Hợp đồng |

## 5. Phụ thuộc chéo khu vực và thứ tự

| Cần | Từ | Dạng |
|-----|----|------|
| Method `codeintel.structuralFacts` (5 `kind`, `pair`, `pathPrefixes`, `limit/offset`; whitelist `gitnexus check --cycles`) | **`AG-CV-SOL-037-structural-facts`** (đối ứng; hợp đồng agent §4.9) | cứng cho 4 bộ; backend phát triển trên tệp vàng `testdata/agent-results/structuralFacts-*.json` (G1) |
| Collector typed client `StructuralFacts` | `BE-CV-SOL-021-agent-collector` | cứng |
| Bảng `finding_dismissals` + tenant_settings (`hotspot_window_days`) | `BE-CV-SOL-011-data-model-and-migrations` | cứng. **Ranh giới sở hữu**: 011 tạo bảng; solution này sở hữu **repository + use case** (CR-037 "Tác động"); nếu `BE-CV-SOL-011-repositories-and-maintenance` đã tạo repository cùng tên, task 037-07 chỉ bổ sung |
| Quyền, audit, hạn mức đồng thời | `BE-CV-SOL-013-*` | cứng |
| `RepoSourceReader` (`Log`, `ReadFile`) | `BE-CV-SOL-030-repo-file-access-gateway` | cứng |
| `changedFiles`, `mergeBase` | `BE-CV-SOL-036-change-overlay-pipeline` | mềm (`origin`, `scope=CHANGED`) |
| `ModuleGraph` (fallback vòng) | `BE-CV-SOL-020/021` | mềm |
| Tiêu thụ | `BE-CV-SOL-036-reading-order-and-risk` (`VIOLATION`), `BE-CV-SOL-038-static-tenant-filter-rule` (cắm `Detector`), `BE-CV-SOL-085-*` (cổng), kênh `BE-CV-SOL-040-codeintel-write-and-stream-channels` (`dismissFinding`) và `…-view-channels` (`findings`), `FE-CV-SOL-059-contract-lens-and-findings` | sau |

Thứ tự theo hợp đồng §7.2: 036 → **037** → 038.

## 6. Kiểm thử

- **Unit thuần**: chuẩn hoá và băm `finding_key` (đổi dòng/tệp cùng package không đổi khoá); khử trùng; luật lớp (test, `usecasetest`, adapter↔adapter); Tarjan; `pr` hạng phần trăm và biên; CODEOWNERS (luật cuối thắng, `**`, thư mục, dòng không owner); parse `git log` NUL, tên tệp có dấu cách/Unicode/C-quote (đối chiếu `agent/src/shared/git-cquoted-path.ts` `decodeGitCQuotedPath`, chưa đọc nội dung). `go test ./services/code-intel-service/internal/domain/structurefinding/...`.
- **Use case**: fake agent (tệp vàng), fake `RepoSourceReader` (log bản vàng), bộ lỗi từng phần (`cycles` timeout), `scope=CHANGED`/`origin`.
- **Repository hai dialect**: ma trận CI `dialect:[postgres, mysql]` (testcontainers, `-tags=integration`): upsert, idempotent, **hai người dismiss đồng thời cùng khoá**, restore, unique; Postgres với role `NOSUPERUSER NOBYPASSRLS`.
- **Cô lập tenant**: tenant B không thấy/không xoá được dismissal của A (cả `repo_id` trùng); mọi câu SQL có `tenant_id` (test AST/`sqlmock` ở repository).
- **Git thật**: repo tạm có commit hàng loạt > 100 tệp và tệp nhiễu; chạy với Git 2.25 và bản mới (`guides/reference/git-compatibility.md`).
- **Bảo mật**: finding không chứa nội dung mã nguồn ngoài `SymbolRef`; không lộ email.
- **Golden Orca** (thủ công/nightly): 6 tệp/3 service ở CR §1; 13 hàm chết ± biến động.
- Chưa chạy bất kỳ test nào.

## 7. Rủi ro và điểm chưa kiểm chứng

- Số liệu (6 tệp, 93 vòng, 13/1 231, 886 commit) do CR đo bằng CLI trực tiếp, chưa qua agent/WS; `indexedCommit` so với HEAD lúc đo chưa kiểm.
- `structuralFacts`/`check --cycles`: giới hạn độ dài/số vòng, thời gian chạy chưa biết.
- `shortlog` + stdin đóng chưa thử qua `git.exec` thật.
- Đổi tên/di chuyển tệp làm mất churn (git bỏ dò đổi tên khi quá nhiều tệp) và mất dismiss; chấp nhận, ghi ở UI.
- Cách ghép dismiss: nếu `finding_key` đổi do đổi tên, trạng thái mất.
- Tỷ lệ báo nhầm mã chết chưa đo (1 ca xác nhận/13).
- `relay-ssh` (Part B) ngoài MVP (O-5).
- Hợp đồng chưa có cột `detectors`; nếu không được thêm thì lỗi bộ phát hiện chỉ có ở log/metric.

## 8. Câu hỏi mở

1. `reason` bắt buộc hay tuỳ chọn (L1)?
2. Dismiss có hết hạn/tự bật lại khi finding đổi mức độ?
3. Tuỳ biến quy tắc lớp ở `c4.yaml` (O-12) hay chỉ mặc định? Tắt từng quy tắc theo repo?
4. Có xếp `layer.adapter-imports-adapter` thành finding (nhiều khả năng chủ ý)?
5. Tạo `CODEOWNERS` cho Orca (E12)?
6. Mở mã chết sang TypeScript khi nào?

## 9. Tiêu chí chấp nhận

- [ ] Trên fixture Orca: `layer.usecase-imports-adapter` = 2 finding (infra-fleet, ai-provider; khử trùng, không `usecasetest`), `layer.domain-imports-outer` = 0.
- [ ] `finding_key` ổn định 100 lần; không chứa số dòng.
- [ ] `cycle.import`: số finding = `cycleCount` sau khử trùng; xác định.
- [ ] Hotspot: bỏ commit > 100 tệp và tệp nhiễu; `metrics` đủ 3 thành phần + `churn`; ≤ 50; giảm dần.
- [ ] Mã chết: không trả `_test.go`, `/cmd/`, `usecasetest`; `NewFleetDefinitionStore` (hai bản) có mặt; `confidence:"medium"`.
- [ ] Owner: không CODEOWNERS ⇒ `history`, không email, `HEAD` rõ ràng; có CODEOWNERS ⇒ luật cuối thắng.
- [ ] `DismissFinding` idempotent, `RESTORE` không lỗi khi vắng; `ListFindings` ẩn/hiện đúng; `dismissed_count` đúng; quyền/tenant/audit.
- [ ] Một bộ lỗi không làm mất bộ khác; `detectors` nêu trạng thái.
- [ ] `scope=CHANGED` gắn `origin` đúng (`introduced`, `touched`, `unknown`, `preexisting`).
- [ ] Hai dialect xanh; mọi truy vấn có `tenant_id`; không `max-lines` disable.
- [ ] Mọi lệnh git thuộc whitelist Part A, không ký tự bị chặn, không `-c` trước subcommand.

## 10. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-037-structure-analysis.md`
- `/opt/repos/orca/backend-go/services/scm-integration-service/internal/usecase/codeowners.go`, `/opt/repos/orca/agent/src/relay/agent-git-handler.ts`, `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/tenant_tx.go`, `/opt/repos/orca/guides/reference/git-compatibility.md`, `/opt/repos/orca/AGENTS.md`
- Series: `AG-CV-SOL-037-structural-facts`, `BE-CV-SOL-011-*`, `013-*`, `021`, `030`, `036-*`, `038-*`, `040-*`, `085-*`, `FE-CV-SOL-059`
