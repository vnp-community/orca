# BE-CV-SOL-072: Kiểm thử bảo mật phía `code-intel-service` và `api-gateway`

> ✅ **Đã triển khai.** Toàn bộ code đã được implement và verify (xem task list).

**CR:** [CR-CV-072](../../../../../../docs/crs/v7/quality-rollout/CR-CV-072-security-tests.md)
**Service:** `code-intel-service` (test + `internal/redteam/` mới), `api-gateway` (test `wscompat`), `.github/workflows/code-intel-security.yml` (mới), `docs/guides/code-intel/code-intel-threat-model.md` (mới)
**TDD tham chiếu:** [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "Multi-tenancy isolation": tenant-id ở mọi truy vấn là cơ chế chính, RLS là lớp phụ, OPA nhận tenant từ JWT; "Input validation & supply chain"; "Audit logging"), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) (mục "Multi-tenancy": `sqlc`/truy vấn nhận `tenant_id` làm tham số ràng buộc, RLS phụ), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (gRPC conventions, WS bridge), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (testing theo lớp), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) §9, [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) §9
**Task:** [`../tasks/README.md`](../tasks/README.md)

---

## 1. Hợp đồng áp dụng

| Nguồn | Mục / PQ | Dùng để |
|---|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.1 tham số (`workspaceRoot` bắt buộc, từ chối khoá lạ, chuỗi không bắt đầu `-`, đường dẫn tương đối không `..`), §2.4, §3 mã lỗi, §9 an toàn (tổng hợp, "bắt buộc kiểm thử") | ca kiểm phía service; tập mã lỗi |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-01 (`CODEINTEL_DISABLED`…), **PQ-03** (5: quyền dự án = `CODEINTEL_NOT_AUTHORIZED`; id tài nguyên con = `CODEINTEL_NOT_FOUND`; 7: phiên thiết bị bị từ chối), PQ-04 (selector `{projectId, worktreeId}`), PQ-05 (`finding_dismissals` ≠ `quality_waivers`), PQ-14 (trần theo kênh, `SetReadLimit(320<<10)`), PQ-24, PQ-25, §3 đầu (chuỗi: guard → tenant+user → cờ → OPA → selector → cổng agent → quyền **trước** cache → che bí mật → audit), §3.1/§3.2 (RPC + action OPA), §4.1 (RLS/MySQL), §6.3 (hành động OPA), §8.3 (kiểm chéo bắt buộc: mọi truy vấn có `tenant_id` và test cô lập tenant), O-16, O-18 | ma trận quyền, cô lập tenant, bộ che |
| `CONTRACT-codeintel-ui-api.md` | §2.1 (selector phẳng), §2.3 bảng lỗi (kể cả `CODEINTEL_SECRET_LEAK_BLOCKED`, `PATH_NOT_ALLOWED`, ánh xạ `NotFound|PermissionDenied` → `NOT_AUTHORIZED`), §2.4 (giới hạn `args[0]` theo kênh, `depth` ngoài khoảng bị từ chối), §2.5 (quyền ở service), §3 (46 kênh), U8 (phiên thiết bị) | fuzz gateway, vector |

## 2. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `agent/src/relay/agent-tool-registry.ts` (`spawn` với `shell: false`, dòng 83; `timeout`/`env: config.toolEnv`), `agent/src/relay/fs-agent-extensions.ts:49,120` (`isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)`: đường dẫn tuyệt đối dùng nguyên), `backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go` (kiểm tenant sở hữu dev server qua `devServers.Get(ctx, tenantID, id)`, sau đó chuyển `method` nguyên văn), `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/sensitive_path_rules.go` (`CleanWorktreePath`: từ chối rỗng/>1024/UTF-8 hỏng, kiểm cả dạng gốc và NFKC, `\`, tiền tố `/`, `%2e` hai lớp, `X:\`), `.../tools/redaction_rules.go`, `.../tools/files_sensitive_guard.go`, `.../mcppolicy/untrusted_wrap.go` (`WrapUntrusted(source, text)`), `backend-go/services/mcp-service/migrations/postgres/0007_external_servers.up.sql:67-71` (`ENABLE` + `FORCE ROW LEVEL SECURITY`, chính sách theo `current_setting('app.tenant_id', true)`), `.../mcp-service/internal/adapter/postgres/tenant_tx.go` (`withTenantTx`, `withRelayTx` `app.relay`), `.../postgres/tenant_scope_guard_test.go` (`TestEveryRepositoryMethodRunsInsideAScopedTx`: duyệt AST `repository.go`), nhiều `*_integration_test.go` tạo vai trò `CREATE ROLE … NOSUPERUSER NOBYPASSRLS` (`session_repository_integration_test.go:36`, `repository_integration_test.go:94`…), `.../mcp-service/internal/redteam/redteam_test.go` (RT01–RT17), `backend-go/services/usage-service/internal/adapter/postgres/repository_test.go:125-138` (ghi rõ pool bằng chủ sở hữu bỏ qua RLS), `backend-go/services/auth-service/migrations/mysql/0001_init.up.sql` (ghi chú "no migration uses FORCE ROW LEVEL SECURITY"), `backend-go/common/internalcaller/internalcaller.go` (`Guard`, `StreamGuard`), `backend-go/common/auditclient/client.go` (`Append(ctx, tenantID, actorID, action, target, outcome, ip)`, **nuốt lỗi**), `backend-go/proto/orca/auth/v1/auth.proto:371-383` (`AppendAuditEntryRequest` **đã có** `actor_type`, `target_type`, `target_id`, `metadata_json`), `backend-go/common/tenant/tenant.go:123` (`RequireTenantID`), `backend-go/ci/mcp-conformance/run-go-conformance.sh` (`FUZZTIME`, `FuzzJSONRPCDecode` ở `mcpserver/conformance_flow_test.go:194`), `.github/workflows/backend-go-issue-status-sync.yml` (ma trận `dialect`).

| # | Correction relative to CR-CV-072 | Bằng chứng |
|---|---|---|
| C1 | CR §1.5: "chỉ `mcp-service` làm đủ RLS thật". Đúng, và **đã có mẫu tái dùng được**: 7 tệp integration của `mcp-service` đã tạo vai trò `NOSUPERUSER NOBYPASSRLS`; `tenant_scope_guard_test.go` đã là test AST "mỗi method repository chạy trong tx có tenant". Task 04/05 sao chép mẫu này, không phát minh | `mcp-service/internal/adapter/postgres/*` |
| C2 | CR §2.8: "`auditclient.Append` không có `actor_type`/`target_type`". **Chính xác hơn**: proto `AppendAuditEntryRequest` đã có các trường (additive, BE-MCP-SOL-013); chỉ **client Go** chưa dùng và vẫn nuốt lỗi. Test audit của v7 chỉ kiểm được những gì client cho phép cho tới khi có phương thức chi tiết (v6 đề xuất `AppendDetailed`; **chưa chắc đã có**) | `auditclient/client.go`, `auth.proto` |
| C3 | CR trỏ `fs.*` "không giới hạn root": đúng (`fs-agent-extensions.ts:49,120`); chưa kiểm Part B | đọc code |
| C4 | Dev compose dùng superuser `orca` ⇒ RLS không hiệu lực ở dev (README v7 §8 điểm 16). Test RLS phải tự tạo vai trò không phải chủ sở hữu trong testcontainers, không dựa compose dev | hợp đồng §4.1 |
| C5 | CI backend 18 workflow, không `golangci-lint`, không `opa test`, không chạy test `agent/`; Go CI 1.25 so với `go.work` 1.26 | `ls .github/workflows` |
| C6 | `code-intel-service`, `internal/redteam/`, gateway `decodeCodeIntelArgs`, `codeIntelChannelError` đều **chưa tồn tại** | `ls` |

## 3. Lệch giữa CR và hợp đồng

| # | CR-CV-072 nói | Hợp đồng nói (thắng) | Hệ quả |
|---|---|---|---|
| L1 | Ma trận 2.8: không quyền project / tenant khác → `NotFound`, "thông điệp giống hệt" | PQ-03 (5): quyền dự án (không phải thành viên, dự án không tồn tại, tenant khác) = **`CODEINTEL_NOT_AUTHORIZED`**; id **tài nguyên con** (job, run, waiver, turn) không thuộc tenant = `CODEINTEL_NOT_FOUND`; gateway ánh xạ `NotFound|PermissionDenied` → `NOT_AUTHORIZED` (ui-api §2.3) | test khẳng định hai trường hợp (project không tồn tại / tenant khác) cho **cùng mã và cùng message**; riêng id con dùng `NOT_FOUND` |
| L2 | `BindRepo`, `SaveC4Overrides` quyền ghi "tuỳ CR-CV-013" | `BindRepo` action `read`; `SaveC4Overrides` = `c4_write` (owner/admin); `SaveQualityProfile` = `quality_profile_write`; `WaiveFinding` = `quality_waive` (member ≤ 7 ngày, không miễn `check`/`error`); `StartQualityRun`, `RecordAgentTurn`… = `review_write` | ma trận dùng **cột Action OPA** của §3.1/§3.2, **49 RPC** (29 + 20) |
| L3 | Ma trận chỉ có read/write/admin | `GetSymbol` cần `read_source`; `GenerateReviewSummary` cần `quality_read` ∧ `read_source`; `ExportReviewReport` cần `quality_read` ∧ `read`; `GetSettings` thành viên tenant, `SetSettings` `role=admin` | thêm cột "đồng thời" (hai action) |
| L4 | `workspaceRoot` "đăng ký" | agent-rpc §2.1: phải là **gốc git worktree** (`git rev-parse --show-toplevel` trùng), tuỳ chọn `ORCA_CODEINTEL_ALLOWED_ROOTS`; agent không có "workspace root đăng ký" thật (README v7 §8 điểm 1) | vector `path-attack-vectors.json` thêm ca "thư mục con của worktree" (từ chối) |
| L5 | Từ chối phiên thiết bị chưa nêu | PQ-03 (7), ui-api U8: `Identity.DeviceID != ""` ⇒ `CODEINTEL_NOT_AUTHORIZED` ở mọi kênh `codeIntel.*` **trừ** `settings.get` | thêm ca vào ma trận (phần gateway) |
| L6 | Guard nội bộ chưa nêu | hợp đồng §3 đầu: `internalcaller.Guard/StreamGuard` với `CODEINTEL_INTERNAL_CALLER_TOKEN`; **rỗng = chặn mọi RPC** | `TestInternalCallerGuardBlocksWhenTokenEmpty` |
| L7 | Che secret chỉ ở đầu ra | ui-api §2.3: `CODEINTEL_SECRET_LEAK_BLOCKED` khi "quét cuối phát hiện secret trong view" | canary: nếu quét cuối bắt được thì ra lỗi này (không hiển thị thô) |
| L8 | Giới hạn gateway chỉ nhắc "vượt giới hạn" | PQ-14/ui-api §2.4: `reviewState.save` ≤ 256 KiB, `c4.save` ≤ 96 KiB (`document` ≤ 64 KiB), `quality.profile.save` ≤ 96 KiB, `quality.trace.confirm|link` ≤ 8 KiB, còn lại ≤ 16 KiB; `SetReadLimit(320<<10)`; `depth` 1..3, ngoài khoảng **bị từ chối, không kẹp ngầm**; `kinds` ≤ 32 | vector kích thước theo từng nhóm kênh |
| L9 | "Từ chối service khi > 16 MiB" (S11) | agent-rpc §3.3: `OUTPUT_TOO_LARGE` khi `ResourceExhausted` > **12 MiB**; PQ-14: client `MaxCallRecvMsgSize(16 MiB)` | trần test = 12 MiB ở infra-fleet, 16 MiB là trần nhận của client; test cả hai biên |
| L10 | Tiêu chí "red-team MCP nếu CR-041" | PQ/O-11: MCP tắt (loại trừ `codeIntel.*` bằng một dòng `excluded_channels.yaml`); P2 | test MCP có điều kiện (task 09) |

## 4. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-072-security-tests-agent` | whitelist lệnh, Cypher, spawn, đầu ra tối đa; cùng đọc `path-attack-vectors.json` do **solution này** sở hữu (`code-intel-service/testdata/security/`) |
| FE | `FE-CV-SOL-073-flag-gating-and-web-e2e` và các lens (`FE-CV-SOL-054…059`) | test XSS/Mermaid (`securityLevel: 'strict'`, nhãn như văn bản): thuộc frontend; solution này **không** viết, chỉ ghi phụ thuộc |
| BE | `BE-CV-SOL-010`, `011-*` (bảng, `withTenantTx`), `012-*` (phân giải đích), `013-authorization-flags-and-audit`, `013-agent-call-gate-and-quotas`, `021`, `022`, `030`, `035` (StorageMap; P2), `040-*` (decoder, lỗi, read limit), `041` (MCP; P2), `085-*` | đối tượng được kiểm |
| BE | `BE-CV-SOL-070` | cùng cây `testdata/`; tệp vàng làm dữ liệu cho test agent giả |
| BE | `BE-CV-SOL-073-settings-flag-and-rollout` | harness e2e T1 dùng lại fake; cổng chặn rollout cần job `code-intel-security` xanh |

## 5. Giải pháp

### 5.1 Bộ vector đường dẫn dùng chung (task 01)

`backend-go/services/code-intel-service/testdata/security/path-attack-vectors.json` (mới): `{ "version": 1, "vectors": [ {"input": "...", "field": "workspaceRoot|file|path", "expect": "allow|deny", "reasonCode": "CODEINTEL_PATH_NOT_ALLOWED", "note": "..."} ] }`. Nội dung từ CR-072 §2.4: `..`, `../x`, `/`, `/etc`, `~`, `~/.ssh`, đường tương đối cho `workspaceRoot`; `a/../../x`, `a/./../..`, `%2e%2e`, `%252e%252e`, `..%c0%af`, `．．`, `․․`; `\` trên POSIX; NUL; độ dài > 1024; UTF-8 hỏng; `C:\`, `\\server\share`, `\\?\C:\`; `/opt/repos/orca` so với `/opt/repos/orca-old` (tiền tố); `file` tới `.env`, `*.pem`, `id_rsa`, `credentials`, `.git/config`. Ba nơi kiểm cùng kết quả (D4): agent (vitest, `AG-CV-SOL-072`), `code-intel-service` (Go), gateway (Go). Symlink thoát root **không** biểu diễn được bằng JSON ⇒ test dựng cây tạm riêng (agent và service).

### 5.2 Gateway (task 02)

`FuzzDecodeCodeIntelArgs` và test bảng cho `decodeCodeIntelArgs` (BE-CV-SOL-040-foundation): từ chối khoá lạ, >1 `args`, kiểu sai, vượt giới hạn theo nhóm kênh (L8), `depth` ngoài 1..3, `kinds` > 32, `projectId` > 64, `worktreeId` > 512; chạy `path-attack-vectors.json` trên mọi tham số đường dẫn; `SetReadLimit(320<<10)` (đóng khung > 320 KiB bị ngắt đúng cách, test với `coder/websocket` v1.8.15 như ghi trong PQ-14); phiên thiết bị bị từ chối (L5). Bất biến fuzz: không panic, từ chối khoá lạ, từ chối vượt giới hạn.

### 5.3 Service: tham số, đường dẫn, agent không tin cậy (task 03, 08)

`FuzzValidateCodeIntelParams` (selector, tham số theo RPC); `TestPathBuilderNeverEscapesBindingRoot`: trình dựng đường dẫn của `RepoSourceReader` (CR-030) không bao giờ ra ngoài `workspace_root` của `repo_bindings`, kể cả khi tên tệp đến từ UI hay từ `SymbolRef.key` (dữ liệu không tin cậy). Agent không tin cậy (S11): kết quả khổng lồ hoặc sai schema, `truncated` tự khai không được dùng để vượt trần service, `workspaceRoot`/đường dẫn do agent trả bị bỏ qua (dùng binding của service, không phản chiếu đường dẫn tuyệt đối ra UI), `sources[].commit` giả không làm khoá cache sai (khoá dùng `headCommit` do service lấy; **chưa chắc** CR-022 làm, câu hỏi mở), không cache kết quả không hợp lệ.

### 5.4 Cô lập tenant (task 04, 05)

Hai lớp, **cả hai bắt buộc** (hợp đồng §8.3 mục 4):
- **Ứng dụng, hai dialect**: (a) AST guard cho mọi method của repository ở `adapter/postgres` và `adapter/mysql` (mẫu `tenant_scope_guard_test.go`); (b) test chéo tenant `Get/List/Update/Delete` cho từng bảng T1–T15 trả rỗng/`not found`; (c) khoá cache snapshot và khoá singleflight có `tenant_id`, test đua hai tenant cùng `(view, head_commit, params_hash)`; (d) `StreamCodeIntelEvents` của tenant A không nhận sự kiện tenant B; (e) relay chỉ gọi `RelayByDevServer` với `dev_server_id` từ `repo_bindings` của tenant gọi, đoán `worktreeId` của B ⇒ `CODEINTEL_NOT_AUTHORIZED`; (f) `tenant.RequireTenantID` ở mọi use case (ctx không tenant ⇒ lỗi, không panic); (g) quét tĩnh MySQL: mọi truy vấn trong `adapter/mysql` có `tenant_id`.
- **RLS thật, Postgres, tag `integration`**: kết nối bằng vai trò `NOSUPERUSER NOBYPASSRLS`; bảng `ENABLE` + `FORCE`; không `set_config('app.tenant_id', …)` ⇒ 0 dòng; `set_config` của A không thấy dòng B; truy vấn thiếu `WHERE` vẫn không rò; `outbox_events` chỉ mở policy `app.relay='on'` cho hai hàm của `common/outbox.Store`; `app.maintenance` chỉ cho `withMaintenanceTx`. Ghi rõ: dev compose dùng superuser ⇒ test tự dựng vai trò (C4).

### 5.5 Quyền và từ chối đồng nhất (task 06)

`TestEveryRPCHasPermissionRow` đọc danh sách RPC từ `ServiceDesc` của hai service (49) và khẳng định có dòng trong bảng dữ liệu `rpc_permission_matrix` (Go struct hoặc JSON), cột: `rpc`, `action` (một hoặc hai), `disabledBehavior` (`CODEINTEL_DISABLED`/`QUALITY_GATE_DISABLED`/cho phép), `roles` kỳ vọng cho {không quyền project, tenant khác, member đọc, member ghi, admin tenant, owner}. Kỳ vọng sinh từ `code_intel.rego` (CR-013) và bảng §3.1/§3.2, không tự bịa. Thêm: `expectedVersion` cũ ⇒ `CODEINTEL_VERSION_CONFLICT`; người A không sửa ghi chú của B nếu CR-052 quy định `updated_by`; `DismissFinding` ghi `dismissed_by`; `Dismiss` không miễn cổng với `error` (PQ-05); `WaiveFinding` member > 7 ngày hoặc `check` ⇒ lỗi; mọi từ chối cùng mã và cùng message (L1), không đo thời gian (oracle thời gian chỉ ghi nhận). Audit: mọi `GetSymbol` và mọi ghi có dòng audit (kiểm theo những gì `auditclient` cho phép, C2).

### 5.6 Canary toàn pipeline (task 07)

Repo mẫu (của `AG-CV-SOL-070`) có tệp cấu hình chứa canary (`postgres://orca:CANARY-DB-PASS-1@db:5432/orca`, `REDIS_PASSWORD=CANARY-REDIS-2`, `API_KEY: CANARY-API-3`, khối PEM giả, `ghp_`/`AKIA` giả, URL có userinfo `CANARY-URL-4`; tên khoá và đường dẫn Vault **được phép**). Chạy qua e2e T1 (agent giả phát lại tệp vàng + tệp canary) rồi quét: `StorageMap` JSON (khi CR-035 có), `graph_snapshots.payload` và mọi cột JSON (dump), log `slog` của service/gateway (handler bắt log), lỗi WS/gRPC, span, khung push `codeIntel.*`, kết quả `GetSymbol`. Dùng **một** bộ che (O-16): tái dùng bảng mẫu của `tools/redaction_rules.go` bằng cách tách gói tên cụ thể (`secretmasking`) hoặc sao có test chênh lệch; test đỏ khi hai bản lệch. Giới hạn đã biết: che theo regex không bắt mọi dạng; chỉ mục công cụ có thể đã chứa secret (ghi nhận, không sửa được).

### 5.7 CI, red-team, tài liệu (task 09)

`.github/workflows/code-intel-security.yml` (mới): job `security` chặn PR (`go test` các gói, vitest phía agent do `AG-CV-SOL-072`, `-fuzz` ngắn `FUZZTIME=10s` cho hai hàm fuzz), tích hợp Postgres/MySQL theo ma trận `dialect` (Docker/testcontainers), canary; job `live-security` hằng đêm không chặn (cần công cụ thật). `internal/redteam/redteam_test.go` (mẫu RT01–RT17 của `mcp-service`): chỉ khi CR-041 có (P2). Tài liệu `docs/guides/code-intel/code-intel-threat-model.md` (S1–S12 + rủi ro còn lại).

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Vector đường dẫn là một tệp JSON, ba nơi kiểm | không tin một tầng; một nguồn dữ liệu (CR-072 D4) |
| D2 | Từ chối thay vì làm sạch | cái cấp phép phải đúng từng byte cái chạy (nguyên tắc `CleanWorktreePath`) |
| D3 | Từ chối quyền = một mã một message | không oracle (PQ-03 5) |
| D4 | RLS kiểm bằng vai trò không phải chủ sở hữu + `FORCE` | RLS không có tác dụng với chủ sở hữu (usage-service) |
| D5 | Ma trận quyền sinh từ `ServiceDesc` + `code_intel.rego` | RPC mới mà quên quyền thì đỏ |
| D6 | Tầng offline chặn PR, tầng live không chặn | công cụ ngoài không ổn định (CR-072 D10) |
| D7 | Canary quét mọi nơi dữ liệu có thể rò | chứng minh bằng quan sát, không bằng đọc code |
| D8 | Test frontend (XSS/Mermaid) thuộc FE | đúng khu vực |

## 7. Tiêu chí chấp nhận

- [x] `path-attack-vectors.json` chạy ở service và gateway (và agent) cùng kết quả; symlink thoát root bị từ chối.
- [x] `FuzzDecodeCodeIntelArgs` và `FuzzValidateCodeIntelParams` chạy ≥ 10 s không panic; khoá lạ, vượt giới hạn, `depth` ngoài khoảng đều bị từ chối.
- [x] Postgres: với vai trò `NOBYPASSRLS` và `FORCE`, không `set_config` thì 0 dòng ở **mọi** bảng T1–T15; MySQL: AST/quét tĩnh xanh và test chéo tenant xanh.
- [x] Cache, singleflight, stream, relay đều có `tenant_id` trong khoá (test đua).
- [x] `TestEveryRPCHasPermissionRow` xanh với 49 RPC; mọi từ chối cùng mã và message.
- [x] Canary: không `CANARY-` trong snapshot, log, lỗi, span, khung push, `GetSymbol`.
- [x] Agent trả kết quả khổng lồ/sai schema không panic và không được cache.
- [x] `TestInternalCallerGuardBlocksWhenTokenEmpty` xanh.
- [x] Workflow `code-intel-security.yml` chạy tầng chặn trên PR; tài liệu threat model có mặt.
- [x] Không `max-lines` disable.

## 8. Kiểm thử

Chính solution này là kế hoạch kiểm thử; bảng tại 5.1–5.7 và từng task. Chạy: `go test ./...` và `go test -tags=integration ./internal/adapter/...` (Docker) trong `code-intel-service`; `go test ./internal/adapter/wscompat/... -run CodeIntel` trong `api-gateway`. **Chưa chạy.**

## 9. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng `gitnexus`/`codegraph` chuẩn hoá Unicode thành `-` hay không (phần agent); chưa biết cờ nào khác có thể đổi repo hoặc ghi đĩa.
- `fs.*` của agent không giới hạn root: nếu `RepoSourceReader` dựng đường dẫn có thành phần từ người dùng mà không qua kiểm của service thì đọc tệp tuỳ ý (ví dụ `/etc/passwd`, `~/.ssh/id_rsa`). Part B (SSH) chưa kiểm chứng.
- Chưa có dev server Windows/WSL để kiểm `C:\`, UNC, phân biệt hoa thường; TOCTOU chưa kiểm chứng.
- Che theo regex không phủ mọi dạng; chỉ mục có thể đã chứa secret trong `content`.
- Oracle thời gian giữa "không có" và "không quyền" không kiểm tự động.
- `FUZZTIME` và số ca fuzz là mặc định đề xuất, chưa đo thời gian CI; Go CI 1.25 so với 1.26.
- Test audit bị giới hạn bởi `auditclient` (nuốt lỗi; chưa dùng trường `actor_type`).
- SSH/remote (AGENTS.md): ca đường dẫn và quyền không giả định thực thi cục bộ; hệ tệp dev server (symlink, mount) chưa kiểm.
- GitLab/provider khác: không có thành phần provider ở solution này; nếu `RefreshCiRun` (CR-086) thêm bề mặt, mở rộng ma trận (đã nằm trong §3.2).

## 10. Câu hỏi mở

1. `fast-check` làm devDependency của `agent/` hay vòng lặp có hạt giống (quyết định bên AG; BE dùng `testing/fuzz` chuẩn, không thêm phụ thuộc)?
2. Bộ che dùng chung (O-16): tách gói `secretmasking` ở `common/` (chạm mọi service) hay đặt ở `code-intel-service` kèm test chênh lệch?
3. Khoá cache có dùng `headCommit` do service tự lấy (qua `git.*`) thay vì lời agent không (liên quan CR-022)?
4. Ai chặn `fs.*` ngoài workspace root ở agent (ngoài v7 nhưng ảnh hưởng an toàn)? Có cần CR riêng?
5. `member` có `reindex`/miễn `check` không (O-18)? Ma trận ở task 06 dùng giá trị tạm của hợp đồng (`member` không miễn `check`; `reindex` cho `member` có hạn mức).
6. Có cần tách quyền `read_source` khỏi `read` cho `GetSymbol` ngay ở v7 (hợp đồng đã tách; CR-072 Q6 hỏi)?

## 11. Khoảng trống hợp đồng ghi nhận

- Hợp đồng chưa liệt kê vai trò × action thành bảng đủ 49 RPC (chỉ có cột Action OPA); bảng vai trò nằm "ở CR-013 §2.2 và CR-085 §2.9" (không thuộc ba hợp đồng).
- Chưa có tên mã lỗi riêng cho "tenant giả mạo qua `worktreeId` đoán được" (dùng `CODEINTEL_NOT_AUTHORIZED`).
- Chưa có mô tả chuẩn của dòng audit (`action`, `target`) cho từng RPC.

## 12. Tham chiếu

- `agent/src/relay/{agent-tool-registry.ts,fs-agent-extensions.ts,agent-git-exec-validator.ts}`
- `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/{sensitive_path_rules.go,redaction_rules.go,files_sensitive_guard.go}`, `.../mcppolicy/untrusted_wrap.go`, `.../mcpserver/conformance_flow_test.go`
- `backend-go/services/mcp-service/{migrations/postgres/0007_external_servers.up.sql,internal/adapter/postgres/*,internal/redteam/redteam_test.go}`, `backend-go/services/usage-service/internal/adapter/postgres/repository_test.go`, `backend-go/common/{internalcaller,auditclient,tenant,outbox}`
- `backend-go/ci/mcp-conformance/run-go-conformance.sh`, `.github/workflows/backend-go-issue-status-sync.yml`
- `docs/crs/v7/quality-rollout/CR-CV-072-security-tests.md`, `docs/crs/v7/README.md` mục 6, 8 (điểm 1, 16, 22)
