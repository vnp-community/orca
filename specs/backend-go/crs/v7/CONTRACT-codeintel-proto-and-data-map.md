# CONTRACT: Bản đồ proto, RPC nội bộ và dữ liệu cho series v7 "Xem code và kiểm soát chất lượng"

> **Nguồn sự thật cho mọi thứ nằm giữa `agent`, `infra-fleet-service`, `code-intel-service`, `api-gateway` và `scm-integration-service` trong series v7.** Các solution `BE-CV-SOL-*`, `FE-CV-SOL-*`, `AG-CV-SOL-*` phải hiện thực đúng ba tài liệu hợp đồng; cần đổi thì sửa tài liệu trước, rồi cập nhật các solution.
> Bộ ba: [`CONTRACT-codeintel-agent-rpc.md`](./CONTRACT-codeintel-agent-rpc.md) (backend ↔ agent), [`CONTRACT-codeintel-ui-api.md`](./CONTRACT-codeintel-ui-api.md) (gateway ↔ frontend/mobile), file này (proto, RPC nội bộ, dữ liệu, thứ tự, ID). **Mục "Phán quyết" ở file này là nơi tập trung mọi mâu thuẫn đã xử lý; hai file kia chỉ tham chiếu `PQ-xx`.**
> CR gốc: [`docs/crs/v7/`](../../../../docs/crs/v7/README.md) (59 CR; README mục 8 thắng mục 3; **bản phán quyết dưới đây thắng cả CR khi hai CR mâu thuẫn nhau**). Mẫu hợp đồng: [`../v6/gateway-and-mcp/CONTRACT-request-ui-api.md`](../v6/gateway-and-mcp/CONTRACT-request-ui-api.md).
> **Trạng thái: 📋 Proposed.** Chưa có dòng code nào của series v7 được hiện thực; chưa chạy build, test, migration, công cụ GitNexus/CodeGraph nào. Mọi thứ ghi "(mới)" là đề xuất của hợp đồng này. Các khẳng định về code hiện có kèm đường dẫn là do người soạn **đọc code thật** trong phiên soạn (2026-10-06); khẳng định lấy từ CR mà chưa tự kiểm ghi "theo CR, chưa kiểm chứng".

## 0. Nguyên tắc tương thích (áp dụng cho cả ba tài liệu)

| # | Nguyên tắc | Bằng chứng / lý do |
|---|---|---|
| H1 | Mọi JSON trên dây UI và JSON-RPC agent là **camelCase**; thời gian RFC 3339 UTC (`string`); id là `string`; mảng rỗng là `[]` (không `null`) | `registry.go` `normalizeNilSlices` chỉ sửa một cấp; view struct phải tự bảo đảm (`backend-go/services/api-gateway/internal/adapter/wscompat/registry.go`) |
| H2 | Danh tính (`tenantId`, `userId`, `role`, `deviceId`) chỉ lấy từ session ở gateway, **không bao giờ** từ tham số | `registry.go` `Identity`; CR-CV-040 G1 |
| H3 | Agent không nhận lệnh, `args`, tên repo, Cypher, `env`, `cwd`, `timeout` từ backend hay UI; chỉ tham số kiểu hẹp và **tên profile** | README v7 D5, O11; CR-CV-001, 081 |
| H4 | Mã lỗi là chuỗi `CODEINTEL_*` đứng đầu thông điệp (`CODE: message`); chi tiết máy đọc nằm ở hậu tố `" \| {json}"` | `session_dialect.go` chỉ gửi `err.Error()` với `code:"internal"`; PQ-02 |
| H5 | Chỉ **thêm** field/enum/kênh (additive); client bỏ qua field lạ; enum lạ rơi về `'unknown'` ở client | `buf breaking` FILE (`backend-go/proto/buf.yaml`); CR-CV-050 §2.1 |
| H6 | Backend trả **mô hình có cấu trúc + khoá i18n**; câu chữ hiển thị do frontend dựng bằng `translate()` | CR-CV-085 F6, CR-CV-090 |
| H7 | Số liệu "kết luận" không bao giờ suy từ thiếu dữ liệu: `unknown` là trạng thái thật, không thành `pass` | README v7 §3.10; CR-CV-085 F2 |
| H8 | Không secret, không mã nguồn trong lỗi/log/cache/span/push; mã nguồn chỉ có ở `GetSymbol` | README v7 §6; CR-CV-072 |
| H9 | Mọi thao tác phải nghĩ tới SSH/dev server từ xa (độ trễ 50–200 ms) và GitLab/provider khác, không chỉ GitHub | `AGENTS.md` |
| H10 | Mọi bảng có `tenant_id`; không FK chéo bảng/chéo service; hai dialect Postgres + MySQL qua `common/dbcapability` | README v7 §6, O1 |

---

## 1. Phán quyết

Mỗi dòng: **vấn đề**, **CR liên quan**, **quyết định**, **lý do**. Các nhóm sau phải theo cột "Quyết định". Mã `PQ-xx` được các hợp đồng khác trích dẫn.

### PQ-01 Mã lỗi khi cờ tính năng tắt
- **Vấn đề.** `CODEINTEL_FEATURE_DISABLED` (CR-CV-013, 010, 012) so với `CODEINTEL_DISABLED` (CR-CV-073, 040, 041, 050, 091; README v7 mục 8 điểm 25 đòi chọn một). Thêm `CODEINTEL_QUALITY_GATE_DISABLED` (CR-CV-085), `CODEINTEL_AI_REVIEW_DISABLED` (093); CR-CV-091 dùng `CODEINTEL_DISABLED` cho cờ quét bảo mật.
- **Quyết định.** (1) Cờ `code_intel_enabled` (hiệu lực = công tắc env AND cờ tenant) tắt: **`CODEINTEL_DISABLED`** (gRPC `FailedPrecondition`). `CODEINTEL_FEATURE_DISABLED` **bị bỏ**; CR-CV-013 phải đổi tên. (2) `code_intel_enabled` bật nhưng `quality_gate_enabled` tắt: mọi RPC `QualityGateService` trả **`CODEINTEL_QUALITY_GATE_DISABLED`** (`FailedPrecondition`). (3) AI tắt (`ai_review_level=off` hoặc công tắc env): `CODEINTEL_AI_REVIEW_DISABLED`. (4) `quality_security_scan_enabled` tắt: profile `security-*`/`dependency-diff` **không có** với tenant (không liệt kê trong `runnableProfiles`), `StartQualityRun` với tên đó trả `CODEINTEL_PROFILE_UNKNOWN` (không dùng `DISABLED`, để frontend không ẩn cả tính năng).
- **Lý do.** Frontend (CR-CV-050 `kind:'disabled'`) ẩn **toàn bộ** tính năng khi gặp `CODEINTEL_DISABLED`; hai cờ phụ chỉ ẩn phần chất lượng/AI nên cần mã riêng. Nhiều CR hơn dùng `CODEINTEL_DISABLED`.

### PQ-02 Đường đi và vị trí của mã lỗi
- **Vấn đề.** README v7 nói `error.data.code`; thực tế `RelayByDevServer` làm mất `error.data` (`usecase/relay_by_dev_server.go`: mọi lỗi `Exec` thành `INFRA_AGENT_EXEC_FAILED`, `apperrors.ToGRPCStatus` chỉ giữ `Code: Message`); kênh WS chỉ có `message` (`envelope.go` `ErrorMessage`) và session-client luôn `code:"internal"` (`session_dialect.go` `writeDialectError`).
- **CR liên quan.** 001 §2.8, 021 §2.3, 023 §2.1, 040 §2.8, 050 §2.3.
- **Quyết định.** Bốn chặng, mỗi chặng một quy tắc cố định:
  1. **Agent → infra-fleet**: JSON-RPC `error: {code: <số AgentErrorCode>, message, data: {code: "CODEINTEL_X", ...}}` (`agent/src/relay/agent-rpc-dispatch.ts` `makeError` giữ `data`).
  2. **infra-fleet → code-intel-service** (CR-CV-023): `domain.AgentRPCError` (mới) → `apperrors.New(kind, "CODEINTEL_X", message≤300, err)` → `status.Message = "CODEINTEL_X: message"`; `error.data` đi qua **trailer gRPC `x-orca-agent-error-data-bin`** (JSON ≤ 4 KiB, hợp lệ hoá bằng cách bỏ phần tử, không cắt giữa chuỗi).
  3. **code-intel-service → api-gateway**: `status.Message` luôn bắt đầu bằng `CODEINTEL_X: `; dữ liệu máy đọc được (candidates, jobId, version…) nằm **trong chính message** dưới dạng hậu tố `" | " + JSON object` (≤ 2 KiB). Không dùng `status.Details`.
  4. **api-gateway → client**: `codeIntelChannelError` (mới, mẫu `mcpChannelError` ở `channels_mcp.go`) bóc `rpc error: code = … desc = `, giữ nguyên chuỗi khi khớp `^CODEINTEL_[A-Z0-9_]+: `, ánh xạ theo `status.Code` khi không khớp (bảng ở `CONTRACT-codeintel-ui-api.md` §2.3). Client đọc **tiền tố của `message`** (`error.message` ở session-client, `message` ở native); `error.code` luôn `"internal"` hoặc vắng và **không dùng**.
- **Cú pháp message trên WS** (chuẩn cho client): `^(CODEINTEL_[A-Z0-9_]+): (.*?)(?: \| (\{.*\}))?$`; phần người đọc là một dòng ≤ 200 ký tự, không chứa `" | {"`; nhóm 3 là JSON `data` (≤ 2 KiB).
- **Lý do.** Đây là cách duy nhất không đổi `wscompat` envelope (dùng chung toàn hệ thống) và đã có tiền lệ (`MCP_*`, v6 `REQUEST_*`). Lấy `data` qua hậu tố message tránh phụ thuộc `status.Details` chưa dùng ở backend-go.

### PQ-03 Tập mã lỗi hợp nhất
- **Vấn đề.** Ba CR đặt mã trùng nghĩa khác tên (`INVALID_ARGUMENT` ở 011/012/085 so với `INVALID_PARAMS` ở agent/gateway; `DEV_SERVER_OFFLINE` là `FailedPrecondition` ở 012 nhưng `Unavailable` ở 021; `OUTPUT_TOO_LARGE` so với `RESPONSE_TOO_LARGE`; `NOT_AUTHORIZED` ở 013 so với `NotFound` che ở 072/040); `common/apperrors` chỉ có 8 Kind, thiếu `ResourceExhausted`, `Unavailable` (`backend-go/common/apperrors/apperrors.go`).
- **Quyết định.** (1) Bảng hợp nhất ở `CONTRACT-codeintel-agent-rpc.md` §3 (lỗi do agent sinh) và `CONTRACT-codeintel-ui-api.md` §2.3 (lỗi tới client); **mỗi mã một nghĩa**. (2) `CODEINTEL_INVALID_ARGUMENT` bị bỏ, dùng `CODEINTEL_INVALID_PARAMS` ở mọi tầng. (3) `CODEINTEL_DEV_SERVER_OFFLINE` = `Unavailable` ở mọi nơi. (4) `CODEINTEL_OUTPUT_TOO_LARGE` = đầu ra công cụ/agent/collector vượt trần (`FailedPrecondition`); `CODEINTEL_RESPONSE_TOO_LARGE` = phản hồi tới UI vượt trần của gateway (gateway sinh). (5) Quyền dự án (không phải thành viên, dự án không tồn tại, tenant khác) = `CODEINTEL_NOT_AUTHORIZED`; id tài nguyên con (job, run, waiver, turn) không thuộc tenant = `CODEINTEL_NOT_FOUND`. (6) `CODEINTEL_SYMBOL_NOT_FOUND` được thêm (symbol/flow/process không có trong chỉ mục; thay `INVALID_PARAMS reason=not_found`). (7) Từ chối phiên thiết bị (`Identity.DeviceID != ""`): `CODEINTEL_NOT_AUTHORIZED`. (8) `common/apperrors` được **thêm** `KindResourceExhausted` và `KindUnavailable` (additive, ánh xạ `codes.ResourceExhausted`, `codes.Unavailable`); việc này thuộc CR-CV-010 và là thay đổi duy nhất được phép ở `common/apperrors`.
- **Lý do.** Giữ test "đúng N mã" của frontend (CR-CV-050) kiểm chứng được; tránh hai mã cùng nghĩa.

### PQ-04 Định danh worktree trên mọi kênh và RPC
- **Vấn đề.** CR-CV-040/050 chỉ truyền `worktreeId`; CR-CV-012 yêu cầu `project_id` + `worktree_ref` để kiểm quyền (`ResolveTarget` chạy sau `GetProject`) và cấm dùng `GetWorktree` theo id (không lọc tenant); CR-CV-031…038/085 dùng `repo_binding_id` mà frontend không có.
- **Quyết định.** Mọi kênh/RPC gắn với worktree nhận **`{ projectId, worktreeId }`** (proto: `WorktreeSelector selector = 1 { string project_id = 1; string worktree_ref = 2; }`, định nghĩa ở `codeintel_common.proto`). `worktreeId` trên dây = **chuỗi `worktree_ref`** đã chuẩn hoá (bỏ tiền tố `id:`/`repo:`; ba dạng: UUID worktree, `<repoId>::<path>`, repo id trần; dạng `::workspace:` bị từ chối `CODEINTEL_WORKTREE_REF_UNSUPPORTED`). Service tự phân giải sang `repo_binding` (cache 30 s sau kiểm quyền). **Mọi trường `repo_binding_id` trong request của CR-CV-031…038, 080…095 được thay bằng `selector`**; `repo_binding_id`/`repo_id` chỉ tồn tại trong bảng, trong response và trong payload sự kiện nội bộ.
- **Lý do.** Gateway không có DB; quyền phải do service thi hành (CR-CV-040 G2); `project_id` bắt buộc để gọi `GetProject`. **Rủi ro đã biết:** frontend phải biết `projectId` của worktree (xem mục 9, điểm mở O-1).

### PQ-05 `finding_dismissals`, `quality_waivers` và khoá
- **Vấn đề.** `repo_binding_id` (README v7 3.5, CR-037, 059) so với `repo_id` (CR-011, README v7 mục 8 điểm 10/25); thiếu `disposition`; UI gửi thêm `note` mà bảng không có.
- **CR liên quan.** 011, 037, 059, 085, 089.
- **Quyết định.** Khoá nghiệp vụ là **`repo_id`**. Bảng `finding_dismissals`: `id, tenant_id, repo_id, repo_binding_id (xuất xứ), finding_key, disposition ('ignored'|'resolved'), reason varchar(500) NOT NULL DEFAULT '' , note varchar(500) NOT NULL DEFAULT '', dismissed_by, at`; UNIQUE `(tenant_id, repo_id, finding_key)`. `RESTORE` = xoá dòng. `finding_key` không chứa số dòng, ≤ 128 ký tự. `quality_waivers` **tách hẳn** (có người, lý do, hạn tối đa 30 ngày, khoá `(repo_id, subject_kind, subject_key, scope)`); `Dismiss` không bao giờ miễn cổng với phát hiện `error` hoặc `blockingSeverities`.
- **Lý do.** Một repo có nhiều worktree/binding; phân loại của người review phải áp cho cả repo.

### PQ-06 `Finding` và `QualityFinding`; enum của `Finding`
- **Vấn đề.** Hai khái niệm phát hiện (README v7 mục 8 điểm 25); 059 dùng `severity: high|medium|low|info`, `origin: introduced_in_scope|preexisting|unknown`, `kind` gạch dưới, `title/summary` chuỗi; 037 dùng `error|warning|info`, `introduced: yes|touched|unknown`, `rule`, `titleKey/params`.
- **Quyết định.** (1) **Giữ hai kiểu tách biệt**: `Finding` (cấu trúc/tĩnh: CR-037, 038) và `QualityFinding` (công cụ: CR-082). Một dock UI, hai nguồn, không trộn danh sách (như README v7 điểm 25). (2) Dây theo CR-037 làm chuẩn: `findingKey`, `rule`, `severity ∈ error|warning|info`, `titleKey`, `params`, `subject`, `evidence[]`, `metrics`, `owner?`, `scope`, `confidence`, `indexFreshness`, `dismissed?`. (3) Trường `introduced` của 037 **đổi tên thành `origin`** với 4 giá trị: `introduced` (mới trong thay đổi đang review), `touched` (có từ trước nhưng nằm trong tệp bị đổi), `preexisting` (ngoài tệp bị đổi), `unknown`. `ViolationRef.status` của 036 dùng đúng hai giá trị `introduced|touched`. (4) `rule` là chuỗi mở (không enum đóng): `layer.domain-imports-outer`, `layer.usecase-imports-adapter`, `layer.adapter-imports-adapter`, `cycle.import`, `hotspot.file`, `dead.unused-export`, `sql.missing-tenant-filter`, `sql.insert-missing-tenant`, `sql.drop-policy`, `sql.disable-rls`. (5) `kind` do **backend điền** từ `rule` theo bảng cố định (`layer.*`→`layer_violation`, `cycle.import`→`dependency_cycle`, `hotspot.file`→`hotspot`, `dead.unused-export`→`dead_code`, `sql.missing-tenant-filter|sql.insert-missing-tenant`→`missing_tenant_id`, `sql.drop-policy|sql.disable-rls`→`rls_removed`); frontend dựng `title/summary` từ `titleKey`+`params`, `locations` từ `evidence`.
- **Lý do.** Backend chịu trách nhiệm về enum; frontend không phải sinh lại ngữ nghĩa; `origin` phân biệt đủ cho lọc "chỉ do thay đổi này".

### PQ-07 Tên và sở hữu file proto
- **Vấn đề.** Ba hệ tên: `codeintel_*.proto` (CR-010), `graph_common/graph_code/graph_sources/code_intel_service.proto` (CR-020), `erd/contract/c4/dataflow.proto` (CR-031…034), thêm `quality_gate/agent_turn/requirement_trace/review_report/ai_review.proto` (CR-085…093). Hai bản `IndexStatus`.
- **Quyết định.** **Một quy ước**: `backend-go/proto/orca/codeintel/v1/codeintel_<chủ đề>.proto`, một `service CodeIntelService` duy nhất trong `codeintel.proto` (mọi RPC thêm vào đây) và một `service QualityGateService` trong `codeintel_quality_gate.proto`. Danh sách cuối cùng và chủ sở hữu ở mục 2. Các tên `graph_*.proto`, `code_intel_service.proto`, `erd.proto`, `contract.proto`, `c4.proto`, `dataflow.proto`, `quality_gate.proto`, `agent_turn.proto`… **không dùng**.
- **Lý do.** Một quy ước dễ lint (`buf lint` STANDARD) và tránh hai file cùng định nghĩa một message.

### PQ-08 `IndexStatus`, `ToolIndexStatus`, `indexScope`, `overall`
- **Vấn đề.** CR-020 định nghĩa `IndexStatus` mỗi công cụ; CR-012 định nghĩa `IndexStatus.overall` tổng hợp; frontend (050) giả định `codeIntel.status` trả `IndexStatus[]`. Ba tập `indexScope` khác nhau (agent `exact|repo_root|none`; CR-080 `exact|repo_root|stale|none`; DB `exact|repo_root|unresolved`); `OVERLAY` chỉ có ở README v7/080.
- **Quyết định.** (1) `ToolIndexStatus` (mỗi công cụ, thay `IndexStatus` của CR-020) và `IndexStatus` (tổng hợp, CR-012) là hai message khác nhau. (2) `codeIntel.status` trả `IndexStatus` **một object** có `tools: ToolIndexStatus[]`; frontend CR-050 phải đổi từ `IndexStatus[]`. (3) Trên agent và UI: `indexScope ∈ exact|repo_root|stale|none` và `freshness ∈ fresh|fresh_base|stale|unknown` (CR-080); `overall ∈ OFFLINE|UNKNOWN|NOT_INSTALLED|BUILDING|MISSING|DEGRADED|OVERLAY|STALE|READY` (thêm `OVERLAY` giữa `DEGRADED` và `STALE`, điều kiện `indexScope=repo_root` và `freshness=fresh_base`). (4) Cột DB `repo_bindings.index_scope` vẫn `exact|repo_root|unresolved` (vị trí chỉ mục): `exact` khi gốc chỉ mục trùng `workspaceRoot`, `repo_root` khi không, `unresolved` ứng với `none`; độ tươi nằm trong `last_status`. (5) `IndexScope` enum proto có `INDEX_SCOPE_UNSPECIFIED = 0` (buf STANDARD).
- **Lý do.** Giữ cả hai nhu cầu (chip tổng, chi tiết công cụ) mà không đụng CHECK của CR-011.

### PQ-09 Chủ sở hữu `ChangeOverlay` và kiểu rủi ro
- **Vấn đề.** CR-020 định nghĩa `ChangeOverlay` gốc (7 trường + dải 10–39), CR-036 định nghĩa mô hình đầy đủ khác nhiều (`scope`, `changedFiles[ChangedFile]`, `components`, `risk: RiskAssessment`, `indexFreshness`, `limits`…); `Risk` enum có `UNKNOWN` ở CR-020 nhưng `RiskAssessment.level` chỉ 4 mức ở 036.
- **Quyết định.** `ChangeOverlay` do **CR-CV-036** sở hữu hoàn toàn (`codeintel_change_overlay.proto`); CR-020 **không** khai báo `ChangeOverlay` (chỉ các kiểu chung). `RiskAssessment.level ∈ LOW|MEDIUM|HIGH|CRITICAL` + `incomplete: bool` + `confidence`; `Risk` enum 5 giá trị chỉ dùng cho `ImpactGraph.risk` (kết quả công cụ). CR-037 và 038 đưa dữ liệu vào overlay qua `ViolationRef` và `TouchedContract` (PQ-30).
- **Lý do.** Một chủ, không dải số dự phòng chia nhiều CR.

### PQ-10 `GetArchitecture` là C4; đồ thị cụm có RPC riêng
- **Vấn đề.** README v7 `GetArchitecture` mơ hồ (C4 hay `ArchitectureGraph` cụm); CR-033 và frontend 055 coi là C4.
- **Quyết định.** `GetArchitecture` → `C4ComponentView` kèm danh sách container (kênh `codeIntel.architecture`). Đồ thị cụm GitNexus (`ArchitectureGraph`) dùng RPC **`GetClusterOverview` (mới)**, **chưa có kênh WS** ở v7 (dành cho MCP/tương lai).
- **Lý do.** UI không cần cụm ở MVP; tránh nhồi hai hình dạng vào một kênh.

### PQ-11 Đăng ký push và payload
- **Vấn đề.** `codeIntel.events.subscribe` (CR-050) so với `codeIntel.subscribe` (CR-040, 073); `worktreeId` đơn tuỳ chọn (040) so với `worktree_ids` 1..50 bắt buộc (024); session-client mất tên kênh push (`push_bridge.go` `pushEventResult`); payload `changed` ba bản (040, 024, 080).
- **Quyết định.** Kênh `codeIntel.subscribe` (stream, `RegisterStream`), tham số `{ selectors?: {projectId, worktreeId}[] }` (0..50); rỗng = mọi worktree người dùng đọc được (service lọc từng sự kiện bằng quyền `read` có cache 10 s). Mỗi khung push là **một object có trường `event`** (`"changed" | "reindexProgress" | "quality.progress" | "quality.finished" | "quality.gateChanged"`), kèm `worktreeId` và `projectId`. Một subscribe/kết nối WS; gRPC `StreamCodeIntelEventsRequest{ repeated WorktreeSelector selectors = 1 }` (0..50). Payload hợp nhất ở `CONTRACT-codeintel-ui-api.md` §5.
- **Lý do.** Frontend mở một luồng theo môi trường (ref-count); `event` thay tên kênh cho session-client.

### PQ-12 Phong bì kết quả phẳng và ETag
- **Vấn đề.** README v7 3.2 phong bì (`sources, headCommit, stale, truncated, totalCount, data`) so với 05 §1 (`repo, worktreeId, devServerId, view`); CR-022 thêm `etag`, `not_modified`; frontend không xử lý ETag.
- **Quyết định.** Kết quả mọi kênh view là **object phẳng**: các trường `ResultMeta` (trừ `devServerId`) nằm cùng cấp với `data`; có `etag`, `notModified`, `fromCache`, `generatedAt`. Tham số tuỳ chọn `ifNoneMatch` (≤ 80 ký tự); khớp thì trả `{ ...meta, notModified: true }` **không có `data`**. Frontend **có thể** không bao giờ gửi `ifNoneMatch` (additive). `totalCount` chỉ có nghĩa khi `> 0`.
- **Lý do.** Một dạng khớp agent (`CodeIntelResult<T>`), không buộc frontend đổi.

### PQ-13 Timeout nhiều tầng
- **Vấn đề.** Go mặc định 30 s (`session.go`/`config.go:57`), chỉ `agent.execPrompt` ngoại lệ (`client.go:412`); `detectChanges` agent 55 s (CR-005) so với README "≥ 60 s"; CR-023 chọn 90 s; WS `invokeTimeout` 25 s (`handler.go:247`); gateway 8/20 s.
- **Quyết định.** Bảng cuối cùng ở `CONTRACT-codeintel-agent-rpc.md` §2.5 (Go: `codeintel.*` trừ `status|reindex|reindexStatus|reindexCancel|watch` = **90 s**; `quality.listProfiles` = 45 s; các `quality.*` khác = 30 s; `ai.complete` = 120 s; agent: mặc định 25 s, `detectChanges`/`structuralFacts` 55 s). Agent **luôn tự hết hạn trước** Go. Gateway: đọc 20 s, ghi/trạng thái 8 s (< 25 s WS). Khi service vượt 20 s: trả `CODEINTEL_TIMEOUT` có hậu tố `{"retryAfterMs":3000,"inProgress":true}` và **tiếp tục thu thập nền** (singleflight, 100 s) để lần gọi sau trúng cache; frontend thử lại tự động (tối đa 90 s).
- **Lý do.** Không đổi `invokeTimeout` toàn cục; không để lệnh nặng bị Go cắt giữa chừng.

### PQ-14 Giới hạn kích thước, đọc WS, TTL cache
- **Vấn đề.** Phản hồi gateway 2 MiB (040) so với proto 3 MiB (020) và snapshot 8 MiB (011), 3 MiB (022); `reviewState.save` và `c4.save` chứa tới 256 KiB/128 KiB nhưng gateway giới hạn args 28 KiB; tài liệu C4 64 KiB (033/UI) vs 24 KiB (040) vs 128 KiB (011); TTL snapshot 24 h (011) vs 7 ngày (022). Đã kiểm: `api-gateway` **không** gọi `SetReadLimit` (grep), `github.com/coder/websocket v1.8.15`; giới hạn đọc mặc định của thư viện (32 KiB theo tài liệu thư viện) **chưa kiểm chứng**.
- **Quyết định.** (1) Mọi response của `CodeIntelService` ≤ **2 MiB** (`proto.Size`; `GetSymbol` ≤ 320 KiB); các ngân sách 3 MiB của CR-020 hạ về 2 MiB. (2) Payload snapshot lưu DB mặc định ≤ 3 MiB (`CODEINTEL_SNAPSHOT_MAX_BYTES`, tối đa 8 MiB; CHECK `payload_bytes ≤ 16 MiB`). (3) TTL snapshot 7 ngày, giữ 3 commit mỗi `(binding, view)` (CR-022 thắng CR-011). (4) **api-gateway phải đặt `conn.SetReadLimit(320 << 10)` cho `/ws`** (việc của CR-CV-040) và tự kiểm giới hạn `args[0]` theo kênh: `reviewState.save` ≤ 256 KiB, `c4.save` ≤ 96 KiB (`document` ≤ 64 KiB), `quality.profile.save` ≤ 96 KiB, `quality.trace.confirm|link` ≤ 8 KiB, còn lại ≤ 16 KiB. (5) Tài liệu `c4.yaml` ≤ 64 KiB ở mọi tầng (UI, gateway, service); DB `document` ≤ 128 KiB. (6) `gRPC MaxCallRecvMsgSize(4 MiB)` đặt tường minh ở gateway; `MaxCallRecvMsgSize(16 MiB)` ở client `code-intel-service → infra-fleet` (đã kiểm: không nơi nào trong backend-go đặt `MaxCallRecvMsgSize`).
- **Lý do.** Một trần dưới 4 MiB; ghi ghi chú/review không bị cắt thầm. Tăng read limit là thay đổi chạm toàn `/ws` nên cần người duyệt (mục 9, O-2).

### PQ-15 Cột của `graph_snapshots`
- **Quyết định.** Cột `commit` được đổi tên **`head_commit`** (từ khoá SQL); thêm `etag char(32)`, `total_count bigint`, `schema_version int`; UNIQUE `(tenant_id, repo_binding_id, view, head_commit, params_hash)`. `view` là chuỗi, tập giá trị ở mục 4. Không CHECK trên `view`.
- **Lý do.** CR-CV-022 §2.8 khuyến nghị; CR-CV-011 sở hữu migration.

### PQ-16 Hợp đồng reindex
- **Vấn đề.** CR-004 `mode: incremental|full` + `tools`; CR-080 thêm `trigger, ifStale, expectHead, tiers` (và nói `tiers` thay `mode/tools`); trạng thái `canceled` (agent) vs `cancelled` (DB); `percent NOT NULL DEFAULT 0` vs `null`; hai bản trạng thái `queued|running|…`.
- **Quyết định.** Giữ `mode`, `tools`; thêm `trigger`, `ifStale`, `expectHead`; **bỏ `tiers`**. Trạng thái job ở mọi tầng: `queued|running|cancelling|succeeded|failed|cancelled|interrupted` (agent), DB/gRPC chỉ `queued|running|succeeded|failed|cancelled` (`cancelling` hiển thị là `running`, `interrupted` lưu `failed` với `error_code='CODEINTEL_REINDEX_INTERRUPTED'`). `percent` là số hoặc `null` (DB: `smallint NULL`). `outcome ∈ already_up_to_date|superseded|skipped_scope_repo_root|""`. UI chỉ gửi `mode`; `trigger='manual'` do service đặt.
- **Lý do.** Một cách diễn đạt; `null` giữ đúng nghĩa "chưa biết".

### PQ-17 Thông báo agent và vận chuyển qua infra-fleet
- **Vấn đề.** `quality.progress|finished` không có `workspaceRoot`/`worktreeId` nên không định tuyến được; CR-023 chỉ chuyển `codeintel.indexChanged|reindexProgress`; `CodeIntelEvent.percent` kiểu `int32` mất `null`; `indexChanged` cần `reason, headCommit, stale, tool:"git"`.
- **Quyết định.** (1) **Mọi** thông báo agent có `workspaceRoot` (kể cả `quality.*`). (2) infra-fleet chuyển thêm `quality.progress`, `quality.finished`; `CodeIntelEvent` đổi: `percent` thành `optional int32`, thêm `reason, head_commit, stale, index_scope, merge_base, trigger, outcome, error_code, run_id`, thêm **`payload_json`** (params gốc ≤ 64 KiB) và `kind ∈ index_changed|reindex_progress|quality_progress|quality_finished|resync|overflow`. (3) code-intel-service gắn `worktreeId` bằng `(tenant, dev_server_id, path_hash)`; chi tiết ở agent contract §6.
- **Lý do.** Một đường vận chuyển chung, ít thay đổi proto về sau.

### PQ-18 infra-fleet: luồng, tools, năng lực
- **Quyết định.** `InfraFleetService.StreamCodeIntelEvents` giới hạn **16** luồng mỗi `(tenant, dev_server)` (không phải 4 của CR-023: mỗi replica code-intel-service mở một luồng); `GetAgentCapabilities` (mới) trả `capabilities[]`, `tools[]`, `platform`, `arch`, `node_version`, `agent_version`, `session_id`, `connected`; `inboundHandshakeParams` của `agentwsserver` và `HandshakeInfo` thêm `Tools []string` (đã kiểm: hiện **không** có, `adapter/agentwsserver/server.go`).
- **Lý do.** `4` sẽ chặn triển khai nhiều bản sao.

### PQ-19 Hình dạng `codeintel.status` và `detectChanges`
- **Quyết định.** (1) Hình dạng `codeintel.status` của **CR-CV-001 + mở rộng CR-CV-080** là chuẩn trên dây agent; CR-CV-012 ánh xạ từ đó (không dùng mảng `tools[]` giả định). (2) `worktreeMismatch` ở `binding` là `boolean`; ở `indexes.<tool>` đổi tên **`rootMismatch: null | {worktreeRoot, indexRoot}`**. (3) `detectChanges`: `warnings` chỉ ở **cấp phong bì**; `data.index = {commit, stale, driftedFileCount, mappingConfidence}` giữ lại; `data.warnings` bỏ. (4) `impact` trả `affectedModules[{name,hits,impact}]` (đúng GitNexus) và backend ánh xạ thành `affectedClusters[{id|null,label,hits,impact}]`; `detectChanges.affectedClusters[{id,label,changedSymbols}]` giữ nguyên tên vì cùng khái niệm cụm. (5) `ImpactGraph` **không có cạnh** ở v7 (hạn chế đã biết).
- **Lý do.** Gỡ hai nghĩa cho một tên trường.

### PQ-20 Chuẩn hoá `SymbolRef` ở agent, tránh cộng dòng hai lần
- **Vấn đề.** CR-002 chuẩn hoá ở agent (dòng +1, `key`, `lineBase:1`); CR-020 giả định agent trả dạng gốc và backend +1; cộng hai lần.
- **Quyết định.** **Agent chuẩn hoá**: mọi `SymbolRef` do agent trả đã có `key`, dòng 1-based, `qualifiedName` chuẩn hoá; `sources[].lineBase = 1` bắt buộc. Backend **không cộng thêm** khi `lineBase===1`; thiếu `lineBase` thì coi GitNexus là 0-based (tương thích ngược). Khi va chạm khoá: `#<arity>` rồi `#L<startLine>` (CR-020), cùng hàm ở agent và backend, cùng vector kiểm thử (CR-CV-070). Backend vẫn **kiểm lại** `key`/đường dẫn (`NormalizeRepoPath`) và hợp nhất hai nguồn.
- **Lý do.** Một nơi chịu trách nhiệm chuẩn hoá, backend chỉ kiểm định và hợp nhất.

### PQ-21 Bộ method agent và quy tắc `workspaceRoot`
- **Quyết định.** Bộ method cuối ở `CONTRACT-codeintel-agent-rpc.md` §4–5: `codeintel.status|overview|processes|process|subgraph|impact|symbol|routes|detectChanges|reindex|reindexStatus|reindexCancel|watch|structuralFacts|codegraphSearch|files`, `quality.listProfiles|run|runStatus|cancel|results|coverage`. `codeintel.node` **không công khai**. **Mọi** method `quality.*` nhận `workspaceRoot` (kể cả `runStatus`, `cancel`, `results`, `coverage`); `runId` không thuộc worktree đó → `CODEINTEL_RUN_NOT_FOUND`. `codeintel.symbol` thêm `includeTrail` chỉ khi CR-003 phát hành (thử nghiệm).
- **Lý do.** Cô lập theo worktree, giảm bề mặt tấn công.

### PQ-22 `review_states`, lượt agent và dấu vết
- **Vấn đề.** README v7 mục 8 điểm 10: `notes`, `turns[]`, `sentBatches[]` riêng; CR-060 lồng `notes.{anchors,sentBatches}` và `reading_progress.turns[]`; CR-089 thêm bảng `agent_turns`; `reviewState.get` khi chưa có bản ghi chưa chốt.
- **Quyết định.** `review_states` khoá `(tenant_id, repo_binding_id, base_commit, head_commit)`; cột JSON: `reading_progress` (`ReadingProgress` v1 ≤ 64 KiB), `notes` = `{anchors, sentBatches}` (≤ 256 KiB, ≤ 500 mục), `turn_markers` (`ReviewTurnMarker[]` ≤ 5, ≤ 256 KiB) chỉ dùng ở **dòng mức worktree** (`base_commit=''` và `head_commit=''`); `status ∈ open|reviewed`. `agent_turns` (CR-089) là kho **provenance** phía backend; `ReviewTurnMarker.turnId` = `agent_turns.client_turn_id` = `"${paneKey}:${doneAt}"`. `reviewState.get` khi chưa có: trả bản mặc định với `version: 0` (không lỗi); `save` với `expectedVersion: 0` = tạo.
- **Lý do.** Tránh hai kho lượt xung đột; marker từng tệp (≈ 30 KB) không đưa vào `agent_turns`.

### PQ-23 Tên biến môi trường và compose
- **Quyết định.** Biến của **code-intel-service**: tiền tố `CODEINTEL_` (`CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_ENABLED`, …); CR-073 đổi `CODE_INTEL_ENABLED` → `CODEINTEL_ENABLED` và tương tự hai biến kia. Biến của **api-gateway**: `CODE_INTEL_SERVICE_ADDR`, `CODE_INTEL_MAX_RESPONSE_BYTES`, `CODE_INTEL_MAX_STREAMS`. Job migrate: `migrate-codeintel` (không `migrate-code-intel`). Container: `orca-go-code-intel`. DB/schema: `codeintel`.
- **Lý do.** Khớp CR-010/013 (nhiều biến hơn) và mẫu `MCP_SERVICE_ADDR` của gateway.

### PQ-24 Cờ tenant: cache, ngoại lệ, cột
- **Quyết định.** Cache cờ **5 s** (CR-073; CR-013 hạ từ 30 s). Khi `CODEINTEL_DISABLED`, các RPC **được phép** vẫn chạy: `GetSettings`, `SetSettings` (admin), `GetReindexJob`. `tenant_settings` có **đủ cột** ngay từ migration `0002` (mục 4.2), không thêm cột sau. Hiệu lực: `code_intel_enabled_effective = CODEINTEL_ENABLED ∧ tenant.code_intel_enabled`; chất lượng thêm `∧ quality_gate_enabled`; AI thêm `∧ ai_review_level ≠ off ∧ CODEINTEL_AI_REVIEW_ENABLED`; quét bảo mật thêm `∧ quality_security_scan_enabled`. Lỗi đọc cờ = tắt (fail closed).
- **Lý do.** Hiệu lực tắt ≤ 5 s; không thể bật lại nếu `SetSettings` bị chặn.

### PQ-25 Run CI và `RefreshCiRun`
- **Quyết định.** Chấp nhận `RefreshCiRun`, kênh `codeIntel.quality.ci`, `scmintegration.ListCommitChecks`, `CiComparison` (ngoài README v7 3.10). `QualityRun.source='ci'`: `status` ← `overall`: `PENDING→running`, `SUCCESS|NEUTRAL→succeeded`, `FAILURE→failed`, `UNKNOWN→failed` với `error_code='CODEINTEL_CI_RESULT_UNKNOWN'` (cổng coi là `unknown`). `CiComparison.relation` 9 giá trị như CR-086. `local_pass_ci_fail` luôn có `reasonsHint[]` và không bao giờ `pass`.
- **Lý do.** Cần một bảng ánh xạ cố định; CR-086 chưa có.

### PQ-26 Không gian tên `ruleId`
- **Quyết định.** `ruleId` của `QualityFinding`: `<tool>/<rule>` (CR-082: `oxlint/...`, `tsc/TS2322`, `go-vet/...`, `orca-check/...`); luật convention `ORCA-NNN` (tool `orca-rules`); CI `CI/<group>/<name>`; bảo mật `SEC-GOVULN/<id>`, `SEC-OSV/<id>`, `SEC-SECRET/<loại>`; phụ thuộc `DEP-*`. Regex hợp lệ: `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`. Luật 084 "không chạy được" **không** sinh finding `ORCA-0xx-unavailable` mà báo trong `ruleResults[]` (`status ∈ ran|skipped_scope|script_not_found|env_not_ready|disabled`) và làm cổng `unknown`. ID `ORCA-008`, `ORCA-009`, `ORCA-016`, `ORCA-017` được **dành riêng, không triển khai**.
- **Lý do.** Gỡ mâu thuẫn regex `^ORCA-\d{3}$` với `ORCA-0xx-unavailable`.

### PQ-27 Kênh `codeIntel.quality.*` và RPC bổ sung
- **Quyết định.** Danh sách kênh cuối ở `CONTRACT-codeintel-ui-api.md` §3.2 (20 kênh unary): thêm `cancel`, `trace.confirm`, `trace.link`, `ci`, `turn.record`, `turns`, `turn` ngoài 13 kênh README; thêm RPC `CancelQualityRun`. Push `codeIntel.quality.gateChanged`. `quality.profile.get` không tên trả `runnableProfiles[]` (không thêm kênh liệt kê).
- **Lý do.** CR-087 §1.2 yêu cầu; không thêm kênh trùng nghĩa.

### PQ-28 `erd-links.yaml`
- **Quyết định.** MVP lưu **tệp trong repo** `docs/code-intel/erd-links.yaml` đọc qua `RepoSourceReader`; chưa có bảng DB. Chuyển sang bảng DB là CR riêng.
- **Lý do.** Không có UI sửa; tránh thêm bảng thứ 15.

### PQ-29 Tên message proto dùng chung
- **Quyết định.** Không trùng tên trong `orca.codeintel.v1`. Chủ sở hữu và tên: `SourceRef` (`codeintel_common.proto`, CR-020; `kind` mở rộng thành `compose|config|adapter|migration|code|proto|wscompat|route|sql`), `ServiceRef{name, proto_service}` (common), `WorktreeSelector` (common), `ContainerRef` (`codeintel_c4.proto`, CR-033), `ComponentRef`, `StoreRef`, `StoreAccess` (`codeintel_dataflow.proto`, CR-034), `TableAccess` (`codeintel_erd.proto`, CR-031; **giữ riêng**, không gộp với `StoreAccess`), cảnh báo giữ bốn kiểu riêng `ParseWarning` (erd), `ContractWarning` (contract), `C4Warning` (c4), `DataFlowGap` (dataflow). Tiền tố cố định: `Erd*`, `C4*`, `DataFlow*`, `Quality*`, `Ai*`, `Requirement*`, `Review*`.
- **Lý do.** README v7 mục 8 điểm 11 và CR-035/038 cần `SourceRef`/`ServiceRef` dùng chung.

### PQ-30 `ContractChange` (038) so với 059; `touchedContracts` (036)
- **Quyết định.** Dây theo CR-038: `kind ∈ proto-service|proto-rpc|proto-message|proto-field|proto-enum|ws-channel|ws-channel-arg|route|route-field|sql-table|sql-column`; `change ∈ added|removed|modified`; `compatibility ∈ breaking|risky|compatible|unknown`; `ruleId` + `details` thay `breakingReasons`; `files[]`+`evidence[]` thay `location`. Frontend ánh xạ (gạch ngang → gạch dưới nếu muốn). `TouchedContract` (036) thêm `compatibility` (và giữ `breaking: bool` = `compatibility=='breaking'`). `ContractChange.kind` thêm `sql-table|sql-column` để phủ `db_schema` của 059.
- **Lý do.** Backend là chủ phân loại; UI không tự phân.

### PQ-31 `ReadingStep` (036) và `ReadingOrderItem` (052)
- **Quyết định.** Dây theo CR-036 (`ReadingStep`: `stepKey, n, file, symbols[], hunks[], reason, reasonParams, dependsOn[], tests[], cycleGroup?, layer`); `components[]` ở `ChangeOverlay`. **Tiến độ đã xem khoá theo `stepKey`** (không `symbol.key`); `ReadingProgress.entries` khoá `stepKey`. Frontend dựng `ReadingOrderItem` từ `ReadingStep`.
- **Lý do.** `stepKey` ổn định theo `(file, sorted(symbolKeys))` (CR-036) và là khoá tiến độ đề xuất của chính CR-036.

### PQ-32 Quy ước chữ hoa/thường của enum
- **Quyết định.** `SymbolRef.kind`, `severity`, `category`, `verdict`, `status`, `source`, `scope`, `overall`... : **chữ thường** (`function`, `error`, `pass`), trừ **`risk.level`/`ImpactGraph.risk`** và **`IndexStatus.overall`**: chữ HOA (`LOW|MEDIUM|HIGH|CRITICAL|UNKNOWN`; `READY|STALE|…`). `ReviewReportModel.risk.level` (090) cũng chữ HOA (đổi từ chữ thường của CR-090).
- **Lý do.** Khớp CR-036/012/073 và 05; một nơi đổi duy nhất (090).

### PQ-33 Hình dạng `QualityTrendPoint` và `CoverageReport`
- **Quyết định.** Dây theo backend (CR-085, 083), frontend 087 phải ánh xạ: `QualityTrendPoint = {turnKey, headCommit, baseCommit, profileRef, verdict, counts:{error,warning,info,byCategory}, metrics:{diffCoverage?,newLayerViolations?,newCycles?,testsFailed?}, runIds[], indexCommit, source, createdAt}` (khoá vắng = không có số, không phải `0`); `CoverageReport` theo CR-083 (`source: measured|estimated`, `totals`, `diff`, `files[]`, `truncated`, `totalCount`, `toolVersions`, `estimatedNote?`). Các kiểu TS của CR-087 (`QualityTrendPoint`, `CoverageReport.lines`) bị thay.
- **Lý do.** Backend nắm dữ liệu; `estimated` phải phân biệt được với `measured`.

### PQ-34 `QualityGate.reasons[]`
- **Quyết định.** `reasons[] = {check, observed, threshold, result ∈ pass|warn|fail|unknown, code, params, runId?, waivedCount?, category?, tool?}`; `category`/`tool` **được thêm** (CR-087 cần để lọc); `basedOn = {runIds[], indexCommit, stale, headCommit, baseCommit, evaluatedAt, profileVersion}`; `profile = "<name>@<scope>/v<version>"`.
- **Lý do.** Gỡ "check là chuỗi tự do".

### PQ-35 Nguồn lượt agent
- **Quyết định.** Nguồn A (renderer gọi `RecordAgentTurn`) là mặc định MVP; nguồn B (hook backend) cần spike (CR-089 Q1) nên **không** nằm trong hợp đồng này. `agent.hook` không dùng. Tín hiệu "agent xong" cho auto-refresh chỉ dùng `orca.infra.agent.statusChanged` kèm `worktree_id`, `dev_server_id` (CR-080) hoặc `codeIntel.hintAgentTurnFinished` (tuỳ chọn, P1).
- **Lý do.** `agent.hook` chưa kiểm chứng phía Go (README v7 điểm 29).

### PQ-36 Mobile
- **Quyết định.** Mobile **không** đi qua `api-gateway` (CR-062): dùng method host `codeIntel.reviewSummary` trên runtime desktop (`MOBILE_RPC_METHOD_ALLOWLIST`); shape ở `CONTRACT-codeintel-ui-api.md` §9. Việc desktop main có tới được gateway hay không **chưa chốt** (mục 9, O-4); trước khi chốt, mobile ở trạng thái `unavailable`.
- **Lý do.** Không phát minh kênh mới cho phiên thiết bị (`DeviceID` bị từ chối ở mọi kênh `codeIntel.*` trừ `settings.get`).

### PQ-37 Xác nhận hai lần cho việc đã quyết ở README v7 mục 8
Giữ nguyên các điều chỉnh 1–29 của README v7 mục 8; hợp đồng này chỉ làm rõ: (a) `agent.hook` không dùng (điểm 29); (b) `codegraph sync` ở worktree liên kết không làm mới (điểm 21): `OVERLAY` dùng diff; (c) `gitnexus analyze` luôn `--index-only` (điểm 7); (d) profile kiểm tra tự lọc env (điểm 22).

> **Tổng: 37 phán quyết** (PQ-01…PQ-37). Điểm không thể chốt ở mục 9.

---
## 2. Bản đồ file proto

> Thư mục: `backend-go/proto/orca/{codeintel,infrafleet,scmintegration}/v1/`. Stub sinh vào `backend-go/proto/gen/go/orca/<pkg>/v1` bằng `buf generate`. **Hiện trạng đã kiểm**: `proto/orca/codeintel/` **chưa có**; `infrafleet/v1/infrafleet.proto` (94 RPC) và `scmintegration/v1/scmintegration.proto` đã có. `buf.yaml`: lint `STANDARD`, breaking `FILE` (`backend-go/proto/buf.yaml`); CI phải gọi `buf` trực tiếp, không dùng `make proto-lint` (có `|| true`, theo CR-CV-010).
> Quy tắc: chỉ thêm; mọi enum có `_UNSPECIFIED = 0`; một message chỉ định nghĩa ở **một** file; RPC chưa có message thì **không** khai báo; request luôn `<Rpc>Request`, response `<Rpc>Response`; tên `Erd*`, `C4*`, `DataFlow*`, `Quality*` theo PQ-29. Số field: khi CR đã ghi số thì **số đó là chuẩn**; khi CR chưa ghi, chủ sở hữu gán theo thứ tự khai báo trong CR và không đổi về sau.

### 2.1 `orca.codeintel.v1` (25 file)

| # | File | Chủ sở hữu | Nội dung (message / RPC) |
|---|---|---|---|
| 1 | `codeintel.proto` | CR-010 tạo; mọi CR thêm dòng `rpc` | `service CodeIntelService` (danh sách RPC ở mục 3.1). Chỉ chứa `service`; không message |
| 2 | `codeintel_common.proto` | CR-020 | `WorktreeSelector{project_id=1, worktree_ref=2}`, `SymbolRef` (key=1, kind=2, native_kind=3, name=4, qualified_name=5, file_path=6, start_line=7, end_line=8, gitnexus_id=9, codegraph_id=10, language=11), `SourceInfo{tool=1,version=2,indexed_at=3,commit=4,line_base=5}`, `SourceRef{path=1,line=2,kind=3}`, `ServiceRef{name=1,proto_service=2}`, `ClusterRef`, `ResultMeta` (13 trường như CR-020 §2.2 + `selector` không có; xem 2.3), `ToolIndexStatus`, `IndexStats`, `PendingChanges`; enum `SymbolKind` (`FUNCTION, METHOD, TYPE, VALUE, FILE, FOLDER, ROUTE, COMPONENT, NAMESPACE, IMPORT, CLUSTER, FLOW, DOC`), `EdgeKind` (14 giá trị CR-020), `ViewKind`, `IndexState`, `Risk`, `ToolName`, `IndexScope` |
| 3 | `codeintel_graph.proto` | CR-020 (kiểu); CR-021 (request/response) | `ClusterNode, ClusterEdge, ArchitectureGraph`, `ModuleNode, ModuleEdge, ModuleGraph`, `SymbolNode, SymbolEdge, SymbolGraph`, `FlowSummary, FlowStep, FlowGraph`, `ImpactSymbol, ImpactLevel, ImpactGraph`, `RouteNode, RouteEdge, RouteMap`; RPC messages của `GetStructure, GetClusterOverview (mới), GetSubgraph, GetImpact, GetSymbol, GetRouteMap` |
| 4 | `codeintel_binding.proto` | CR-012 | `RepoBinding`, `IndexStatus` (tổng hợp: `overall`, `tools[]`, `active_job`, `scope_mismatch`, `index_basis[]`, `last_status_at`), `BindRepo*`, `ListRepoBindings*`, `GetIndexStatus*` |
| 5 | `codeintel_index_basis.proto` | CR-080 | `IndexBasis` (13 trường như CR-080 §2.3) |
| 6 | `codeintel_reindex.proto` | CR-021 (điều phối) + CR-013 (hạn mức) + CR-004 (ngữ nghĩa) | `ReindexJob`, `RequestReindex*`, `GetReindexJob*` |
| 7 | `codeintel_events.proto` | CR-024 | `StreamCodeIntelEventsRequest{repeated WorktreeSelector selectors=1}`, `CodeIntelPush` (mục 2.3) |
| 8 | `codeintel_settings.proto` | CR-073 | `TenantSettings`, `GetSettings*`, `SetSettings*` |
| 9 | `codeintel_erd.proto` | CR-031 | `ErdModel, ErdTable, ErdColumn, ErdIndex, ErdCheck, ErdPolicy, ErdRelation, ErdEndpoint, ErdExternalRef, ErdChange (mới), TableAccess, ParseWarning`; `GetErd*` (nhận `selector, service, dialect, base_ref, head_ref, include_access, include_inferred`; bỏ `ref`/`repo_binding_id`) |
| 10 | `codeintel_contract.proto` | CR-032 | `ServiceContract, RpcContract, RpcEdge, WsChannel, WsTarget, ContractWarning, ContractCatalog` — **chỉ message nội bộ, không RPC** (PQ-29) |
| 11 | `codeintel_c4.proto` | CR-033 | `ContainerRef, C4ComponentView, C4Component, C4Relation, C4External, C4Warning`; `GetArchitecture*` (trả `containers[]` + `view`), `GetC4Overrides*`, `SaveC4Overrides*` |
| 12 | `codeintel_dataflow.proto` | CR-034 | `DataFlow, DataFlowTrigger, DataFlowStep, ComponentRef, StoreAccess, StoreRef, DataFlowGap, RelatedProcess, DataFlowSummary, SequenceModel, SeqParticipant, SeqMessage, DfdModel, DfdNode, DfdEdge`; `ListDataFlows*`, `GetDataFlow*` (số field như CR-034 §2.1) |
| 13 | `codeintel_storage.proto` | CR-035 | `StorageMap, Store, Binding, Topic`; `GetStorageMap*` |
| 14 | `codeintel_change_overlay.proto` | CR-036 | `ChangeOverlay, ChangedFile, ChangedSymbol, ReadingStep, ComponentGroup, RiskAssessment, RiskReason, IndexFreshness, TouchedTable, TouchedContract, ViolationRef, OverlayLimits`; `GetChangeOverlay*`, `GetReadingOrder*` |
| 15 | `codeintel_findings.proto` | CR-037 | `Finding, Evidence, Owner`; `ListFindings*`, `DismissFinding*` |
| 16 | `codeintel_contract_diff.proto` | CR-038 | `ContractDiff, ContractChange, ConsumerRef, MigrationDiff, SqlChange, TableImpact`; `GetContractDiff*` |
| 17 | `codeintel_review_state.proto` | CR-011 (kiểu lưu) + CR-052/060 (nội dung) | `ReviewState`, `GetReviewState*`, `SaveReviewState*` |
| 18 | `codeintel_quality.proto` | CR-082 | `QualityFinding, QualityStepResult, QualityRunSummary, QualityRun` (số field CR-082 §2.1, thêm `error_code=19`, `tree_fingerprint=20`, `provider=21`, `external_ref=22`, `external_url=23`, `fetched_at=24`, `stale_after=25`, `dirty=26`) |
| 19 | `codeintel_quality_gate.proto` | CR-085 | `service QualityGateService` (mục 3.2); `QualityGate, QualityGateReason, QualityProfile, QualityProfileDefinition, QualityWaiver, QualityTrendPoint`; request/response của `StartQualityRun, GetQualityRun, ListQualityRuns, CancelQualityRun (mới), ListQualityFindings, WaiveFinding, GetQualityGate, GetQualityProfile, SaveQualityProfile, GetQualityTrend` |
| 20 | `codeintel_coverage.proto` | CR-083 | `CoverageReport, FileCoverage, DiffCoverage`; `GetCoverage*` |
| 21 | `codeintel_agent_turn.proto` | CR-089 | `AgentTurn, CommandSummary, AgentClaim, ClaimVerification`; `RecordAgentTurn*, ListAgentTurns*, GetAgentTurn*` |
| 22 | `codeintel_requirement_trace.proto` | CR-092 | `RequirementTrace, Requirement, RequirementEvidence`; `GetRequirementTrace*, ConfirmRequirementEvidence*, LinkWorktreeTask*` |
| 23 | `codeintel_review_report.proto` | CR-090 | `ReviewReportModel` (+ phần con); `ExportReviewReport*` |
| 24 | `codeintel_ai_review.proto` | CR-093 | `AiReviewSummary, AiReviewManifest`; `GenerateReviewSummary*` |
| 25 | `codeintel_ci.proto` | CR-086 | `CiComparison, CiRunRef`; `RefreshCiRun*` |

> Tổng 25 file. `codeintel_ci.proto` và `codeintel_ai_review.proto` thuộc đợt 8–9. Không file nào khác được tạo trong package này mà không sửa bảng này.

**Quy tắc phụ thuộc import** (tránh vòng): `codeintel_common` ← mọi file; `codeintel_graph` ← `codeintel_c4/dataflow/change_overlay/findings`; `codeintel_index_basis` ← `codeintel_binding`, `codeintel_quality`; `codeintel_quality` ← `codeintel_quality_gate`, `codeintel_coverage`, `codeintel_ci`; không file nào import `codeintel.proto` (service).

### 2.2 `orca.infrafleet.v1` (sửa `infrafleet.proto`; chủ sở hữu CR-023)

| Thêm | Nội dung |
|---|---|
| `rpc StreamCodeIntelEvents(StreamCodeIntelEventsRequest) returns (stream CodeIntelEvent)` | mẫu `StreamFileChanges` (`infrafleet.proto:114`); request `{string dev_server_id = 1}` (bắt buộc) |
| `message CodeIntelEvent` | xem 2.3; **tenant phải gắn bằng `withTenantFromStreamMetadata` ngay đầu handler** (không bắt chước `StreamFileChanges`, theo CR-023 §2.4; chưa kiểm chứng lỗi của mẫu) |
| `rpc GetAgentCapabilities(GetAgentCapabilitiesRequest) returns (GetAgentCapabilitiesResponse)` | request `{dev_server_id}`; response `{connected, platform, arch, node_version, agent_version, capabilities[], tools[], session_id}` |
| sửa `inboundHandshakeParams`/`HandshakeInfo` (Go) | thêm `Tools []string` (không đổi proto) |
| payload `orca.infra.agent.statusChanged` | thêm `worktree_id`, `dev_server_id` (CR-080), tương thích ngược |

Trùng tên có chủ ý: `StreamCodeIntelEvents`, `StreamCodeIntelEventsRequest` tồn tại ở cả `orca.infrafleet.v1` (theo `dev_server_id`) và `orca.codeintel.v1` (theo `selectors`); khác package nên hợp lệ, nhưng mã Go phải đặt alias import (`fleetv1`, `codeintelv1`).

### 2.3 Message dùng chung đã chốt (nguyên văn, chuẩn)

```proto
// codeintel_common.proto
message WorktreeSelector { string project_id = 1; string worktree_ref = 2; }

message ResultMeta {                   // CR-020 §2.2
  string repo = 1;  string worktree_id = 2;  string dev_server_id = 3;   // 3 không ra WS
  ViewKind view = 4;  repeated SourceInfo sources = 5;
  string head_commit = 6;  bool stale = 7;  bool truncated = 8;
  int64 total_count = 9;               // 0 = không biết; chỉ có nghĩa khi > 0
  string etag = 10;  google.protobuf.Timestamp generated_at = 11;
  bool from_cache = 12;  bool not_modified = 13;
}

message SourceInfo { ToolName tool = 1; string version = 2; string indexed_at = 3; string commit = 4; int32 line_base = 5; }

// codeintel_binding.proto
message IndexStatus {                  // tổng hợp, CR-012 + PQ-08
  string overall = 1;                  // OFFLINE|UNKNOWN|NOT_INSTALLED|BUILDING|MISSING|DEGRADED|OVERLAY|STALE|READY
  repeated ToolIndexStatus tools = 2;  bool scope_mismatch = 3;
  ActiveReindexJob active_job = 4;     // {id, stage, percent?}
  repeated IndexBasis index_basis = 5; google.protobuf.Timestamp last_status_at = 6;
  string error_code = 7;               // khi overall=UNKNOWN: mã thô của agent/infra-fleet
  RepoBinding binding = 8;
}

// codeintel_events.proto  (gateway nhận; số 1..13 theo CR-024 §2.6, 14.. bổ sung)
message StreamCodeIntelEventsRequest { repeated WorktreeSelector selectors = 1; }   // 0..50
message CodeIntelPush {
  string kind = 1;        // changed | reindex_progress | quality_progress | quality_finished | quality_gate_changed
  string event_id = 2;  string worktree_id = 3;  string repo_binding_id = 4;
  string reason = 5;      // index_changed | reindex_finished | head_changed | resync | overflow
  repeated string tools = 6;  string commit = 7;  string indexed_at = 8;
  string job_id = 9;  string stage = 10;  optional int32 percent = 11;  string message = 12;
  google.protobuf.Timestamp occurred_at = 13;
  string project_id = 14;  string head_commit = 15;  bool stale = 16;
  string index_scope = 17;  string freshness = 18;  bool resync = 19;
  string payload_json = 20;   // quality_*: params gốc đã lọc ≤ 8 KiB
}

// infrafleet.proto  (CR-023, sửa theo PQ-17/18)
message StreamCodeIntelEventsRequest { string dev_server_id = 1; }
message CodeIntelEvent {
  string kind = 1;        // index_changed | reindex_progress | quality_progress | quality_finished | resync | overflow
  string workspace_root = 2;  string tool = 3;  // gitnexus | codegraph | git
  string commit = 4;  string indexed_at = 5;  string job_id = 6;  string stage = 7;
  optional int32 percent = 8;  string message = 9;
  google.protobuf.Timestamp received_at = 10;
  string reason = 11;  string head_commit = 12;  bool stale = 13;
  string index_scope = 14;  string merge_base = 15;  string trigger = 16;
  string outcome = 17;  string error_code = 18;  string run_id = 19;
  string payload_json = 20;   // params JSON-RPC gốc ≤ 64 KiB
}
```

> `percent` ở cả hai message là `optional int32` (null = chưa biết, PQ-17). `ToolName` enum chỉ có `GITNEXUS, CODEGRAPH`; thông báo `tool:"git"` mang qua chuỗi `tool` của `CodeIntelEvent`, không qua enum.

### 2.4 `orca.scmintegration.v1` (CR-086)

`rpc ListCommitChecks(ListCommitChecksRequest) returns (ListCommitChecksResponse)` và message `CommitCheck`, `CheckStep`, `CheckAnnotation`, `RateLimitInfo` như CR-086 §2.1 (`provider`, `repo_slug`, `ref_kind: PULL_REQUEST|COMMIT`, `pr_number`, `commit_sha`, `include_annotations`, `max_checks ≤ 200` → `head_sha, provider, items[], overall, fetched_at, capability_unsupported, rate_limit, truncated, total_count`). Provider chưa hỗ trợ trả `capability_unsupported=true`, không lỗi. Không đổi `window.api.gh.prChecks`.

---

## 3. RPC nội bộ

> Mọi RPC của `CodeIntelService` và `QualityGateService`: (1) `internalcaller.Guard`/`StreamGuard` với `CODEINTEL_INTERNAL_CALLER_TOKEN` (rỗng = chặn mọi RPC), (2) tenant+user từ metadata (`x-orca-tenant-id`, `x-orca-user-id`, `x-orca-role`, `x-orca-client-ip`), (3) cờ, (4) quyền OPA, (5) phân giải `selector`, (6) cổng agent, (7) quyền **trước** cache, (8) che bí mật, (9) audit (CR-013 §2.1). `selector` là trường 1 của mọi request gắn worktree.

### 3.1 `orca.codeintel.v1.CodeIntelService`

| RPC | Request (ngoài `selector`) | Response | Action OPA | Kênh WS | Chủ sở hữu |
|---|---|---|---|---|---|
| `BindRepo` | — | `{binding, status}` | `read` | `codeIntel.bindRepo` | CR-012 |
| `ListRepoBindings` | `project_id`, `limit ≤ 200` (không `selector`) | `bindings[]` | `read` | (không kênh) | CR-012 |
| `GetIndexStatus` | `refresh` | `{status: IndexStatus}` | `read` | `codeIntel.status` | CR-012 |
| `RequestReindex` | `mode` (`incremental|full`) | `{job: ReindexJob}` | `reindex` | `codeIntel.reindex` | CR-021 |
| `GetReindexJob` | `job_id` | `{job}` | `read` (được phép khi cờ tắt) | `codeIntel.reindexStatus` | CR-021 |
| `GetStructure` | `path, depth ≤ 3, limit, page_token, if_none_match` | `{meta, data: ModuleGraph, next_page_token}` | `read` | `codeIntel.structure` | CR-020/021 |
| `GetClusterOverview` (mới) | `top_n ≤ 500, if_none_match` | `{meta, data: ArchitectureGraph}` | `read` | (chưa có kênh) | CR-020/021 |
| `GetArchitecture` | `container, include_hidden, if_none_match` | `{meta, containers[], view: C4ComponentView}` | `read` | `codeIntel.architecture` | CR-033 |
| `ListDataFlows` | `trigger_kind, query, service, page_size ≤ 100, page_token` | `{flows[], next_page_token, total, meta}` | `read` | `codeIntel.dataFlows` | CR-034 |
| `GetDataFlow` | `flow_id, dialect, max_service_hops ≤ 8, max_steps ≤ 200, include_sequence, include_dfd, detail` | `{flow, sequence?, dfd?, meta}` | `read` | `codeIntel.dataFlow` | CR-034 |
| `GetErd` | `service, dialect, base_ref, head_ref, include_access, include_inferred` | `{model, meta, warnings[]}` | `read` | `codeIntel.erd` | CR-031 |
| `GetStorageMap` | `env_filter, include_legacy` | `{map, meta}` | `read` | `codeIntel.storage` | CR-035 |
| `GetSubgraph` | `center{symbol|file|cluster}, depth ≤ 3, kinds[], limit ≤ 1500` | `{meta, data: SymbolGraph}` | `read` | `codeIntel.subgraph` | CR-020/021 |
| `GetImpact` | `target{key|name+file+kind}, direction, depth ≤ 3, include_tests` | `{meta, data: ImpactGraph}` | `read` | `codeIntel.impact` | CR-020/021 |
| `GetSymbol` | `key | name+file, include_source` | `{meta, data: SymbolDetail}` | `read_source` | `codeIntel.symbol` | CR-020/021 |
| `GetRouteMap` | `limit, page_token` | `{meta, data: RouteMap, next_page_token}` | `read` | `codeIntel.routes` | CR-020/021 |
| `GetChangeOverlay` | `base_ref, head_ref, mode, detail` | `{meta, overlay}` | `read` | `codeIntel.changeOverlay` | CR-036 |
| `GetReadingOrder` | `base_ref, head_ref` | `{meta, steps[], components[]}` | `read` | `codeIntel.readingOrder` | CR-036 |
| `ListFindings` | `rules[], severities[], path_prefix, include_dismissed, scope (ALL|CHANGED), base_ref, page_size ≤ 200, page_token` | `{findings[], next_page_token, total_count, dismissed_count, truncated, sources[], index_freshness}` | `read` | `codeIntel.findings` | CR-037 |
| `DismissFinding` | `finding_key, action (DISMISS|RESTORE), disposition, reason, note` | `{finding_key, dismissed, disposition}` | `review_write` | `codeIntel.dismissFinding` | CR-037 |
| `GetContractDiff` | `base_ref, kinds[], detail` | `{diff, sources[], index_freshness}` | `read` | `codeIntel.contractDiff` | CR-038 |
| `GetReviewState` | `base_commit, head_commit` | `{state}` (mặc định `version:0`) | `read` | `codeIntel.reviewState.get` | CR-011/052 |
| `SaveReviewState` | `base_commit, head_commit, reading_progress, notes, turn_markers, status, expected_version` | `{state}` | `review_write` | `codeIntel.reviewState.save` | CR-011/052/060 |
| `GetC4Overrides` | `container` | `{document, version, updated_by, updated_at, seed_source}` | `read` | `codeIntel.c4.get` | CR-033 |
| `SaveC4Overrides` | `container, document, expected_version` | `{version, warnings[]}` | `c4_write` | `codeIntel.c4.save` | CR-033 |
| `GetSettings` | — | `{effective, tenant}` | thành viên tenant | `codeIntel.settings.get` | CR-073 |
| `SetSettings` | các cờ (mục 6) | như `GetSettings` | `role=admin` | `codeIntel.settings.set` | CR-073 |
| `StreamCodeIntelEvents` (server-stream) | `selectors[] (0..50)` | `stream CodeIntelPush` | `read` (lọc từng sự kiện) | `codeIntel.subscribe` | CR-024 |
| `HintAgentTurnFinished` (mới, P1, tuỳ chọn) | `agent_pane_key?` | `{}` | `read` | `codeIntel.hintAgentTurnFinished` | CR-080 |

### 3.2 `orca.codeintel.v1.QualityGateService`

| RPC | Request (ngoài `selector`) | Response | Action OPA | Kênh WS | Chủ sở hữu |
|---|---|---|---|---|---|
| `StartQualityRun` | `profile, scope, base?` | `{run: QualityRun}` | `review_write` | `codeIntel.quality.start` | CR-085 (điều phối) / 081 |
| `GetQualityRun` | `run_id` | `{run}` | `quality_read` | `codeIntel.quality.run` | CR-085 |
| `ListQualityRuns` | `limit ≤ 50, source?, page_token` | `{runs[], next_page_token}` | `quality_read` | `codeIntel.quality.runs` | CR-085 |
| `CancelQualityRun` (mới) | `run_id` | `{run}` | `review_write` | `codeIntel.quality.cancel` | CR-085 |
| `ListQualityFindings` | `run_id?, severities[], categories[], file?, in_scope?, page_size ≤ 500, page_token` | `{findings[], total_count, truncated, outside_scope_count, next_page_token}` | `quality_read` | `codeIntel.quality.findings` | CR-082/085 |
| `WaiveFinding` | `subject_kind, subject_key, action (WAIVE|REVOKE), reason, expires_at, scope` | `{waiver}` | `quality_waive` | `codeIntel.quality.waive` | CR-085 |
| `GetQualityGate` | `base_ref?, profile_name?, turn_key?, record?, include_waived_detail?` | `{gate, waivers[≤50], evaluated_at, profile_definition_digest, comparison[]}` | `quality_read` | `codeIntel.quality.gate` | CR-085 (+086 `comparison`) |
| `GetQualityProfile` | `repo_id?, name?` | `{profile, origin, version, runnable_profiles[]}` | `quality_read` | `codeIntel.quality.profile.get` | CR-085 |
| `SaveQualityProfile` | `scope (tenant|repo), name, definition, expected_version` | `{profile, warnings[]}` | `quality_profile_write` | `codeIntel.quality.profile.save` | CR-085 |
| `GetQualityTrend` | `from?, to?, limit ≤ 200, group_by (commit|turn)` | `{points[], truncated, total_count}` | `quality_read` | `codeIntel.quality.trend` | CR-085 |
| `GetCoverage` | `run_id? | head_commit` | `{report: CoverageReport}` | `quality_read` | `codeIntel.quality.coverage` | CR-083 |
| `RecordAgentTurn` | trường §4.2 CR-089 | `{turn}` | `review_write` | `codeIntel.quality.turn.record` | CR-089 |
| `ListAgentTurns` | `limit ≤ 50, before?` | `{turns[]}` | `quality_read` | `codeIntel.quality.turns` | CR-089 |
| `GetAgentTurn` | `turn_id` | `{turn}` | `quality_read` | `codeIntel.quality.turn` | CR-089 |
| `GetRequirementTrace` | `base_ref?, include_inferred, task_id?, turn_key?` | `{trace, evaluated_at}` | `quality_read` | `codeIntel.quality.trace` | CR-092 |
| `ConfirmRequirementEvidence` | `requirement_key, evidence_kind, evidence_ref, link_kind (CONFIRM|REJECT), scope` | `{trace}` | `review_write` | `codeIntel.quality.trace.confirm` | CR-092 |
| `LinkWorktreeTask` | `task_id` (rỗng = gỡ liên kết) | `{trace}` | `review_write` | `codeIntel.quality.trace.link` | CR-092 |
| `GenerateReviewSummary` | `base_ref?, profile_name?, level, dry_run, force_refresh, locale` | `{summary, manifest, cache, labels}` | `quality_read` ∧ `read_source` | `codeIntel.quality.summary` | CR-093 |
| `ExportReviewReport` | `base_ref?, profile_name?, turn_key?, sections[], max_findings, max_reading_steps, include_people, include_waiver_reasons` | `{model, generated_for, warnings[]}` | `quality_read` ∧ `read` | `codeIntel.quality.report` | CR-090 |
| `RefreshCiRun` | `force?` | `{run, comparison[], stale, rate_limited, reset_at}` | `quality_read` | `codeIntel.quality.ci` | CR-086 |

> Push `codeIntel.quality.progress`, `.finished`, `.gateChanged` đi qua `StreamCodeIntelEvents` (không RPC riêng). `ListRepoBindings` không có kênh (CR-040 Q4: chưa thêm `codeIntel.bindings`).

### 3.3 RPC phía nguồn (client của code-intel-service)

| Dịch vụ | RPC dùng | Mục đích | CR |
|---|---|---|---|
| `InfraFleetService` | `RelayByDevServer{dev_server_id, method, params_json}` → `{result_json}`; `IsDevServerConnected`; `GetAgentCapabilities` (mới); `StreamCodeIntelEvents` (mới); `ListDevServers` | gọi agent, năng lực, sự kiện, kiểm duyệt dev server | 012, 021, 023, 024 |
| `ProjectService` | `ListRepos`, `ListWorktrees`, `GetProject`, `ListMembers` | phân giải đích, quyền (không dùng `GetRepo`/`GetWorktree` theo id) | 012, 013 |
| `GitGatewayService` | `DetectWorktrees` (chỉ dạng ngoài project-service) | xác nhận đường dẫn ngoài | 012 |
| `ScmIntegrationService` | `ListCommitChecks` (mới), `GetPullRequestForBranch`, `GetRateLimitStatus` | CI/PR | 086 |
| `AuthService` (auditclient) | `Append` (thiếu `actor_type`/`target_type`, nuốt lỗi) | audit | 013 |
| `TaskService` | `GetTask`, `ResolvePermission`; **đề nghị** `FindTaskBySource` (CR riêng, chưa có) | truy vết yêu cầu | 092 |

---
## 4. Cơ sở dữ liệu của `code-intel-service` (một danh sách duy nhất)

### 4.1 Quy ước

- DB/schema tên **`codeintel`**: Postgres `CREATE SCHEMA IF NOT EXISTS codeintel;` MySQL database `codeintel` (bảng không tiền tố). Id do ứng dụng sinh (`uuid.NewString()`). Migration `golang-migrate` ở `services/code-intel-service/migrations/{postgres,mysql}/NNNN_<tên>.{up,down}.sql`.
- Kiểu khái niệm → Postgres / MySQL: `uuid` = `UUID` / `CHAR(36)`; `ts` = `TIMESTAMPTZ` / `TIMESTAMP(6)`; `json` = `JSONB` / `JSON`; `bool` = `BOOLEAN` / `TINYINT(1)`; `vN` = `VARCHAR(N)`; `text` = `TEXT` (MySQL `MEDIUMTEXT` khi ghi chú); `int`, `smallint`, `bigint`.
- Mọi bảng: `tenant_id uuid NOT NULL`, **không FK** (id tham chiếu `project_id`, `repo_id`, `worktree_id`, `dev_server_id`… kiểm ở ứng dụng), đồng hồ DB cho hết hạn (PG `now()`; MySQL `CURRENT_TIMESTAMP(6)`). Postgres: `ENABLE` + `FORCE ROW LEVEL SECURITY`, chính sách `tenant_isolation` (`USING` và `WITH CHECK`) theo `NULLIF(current_setting('app.tenant_id', true), '')::uuid`, đặt bằng `set_config('app.tenant_id', $1, true)` trong `withTenantTx`; chính sách hẹp `app.maintenance`/`app.relay` cho bảo trì và outbox (CR-011 §2.5). MySQL: không RLS, `WHERE tenant_id = ?` ở mọi truy vấn (test AST bắt phương thức quên). **Dev compose dùng superuser `orca` nên RLS không có hiệu lực ở dev; chỉ `mcp-service` có RLS thật** (README v7 mục 8 điểm 16) — yêu cầu role `NOSUPERUSER NOBYPASSRLS` ghi vào README service.
- Ghi có khoá lạc quan: `version bigint NOT NULL DEFAULT 1`; CAS `UPDATE … WHERE id=? AND tenant_id=? AND version=?`; 0 hàng → `CODEINTEL_NOT_FOUND` hoặc `CODEINTEL_VERSION_CONFLICT`. Vi phạm duy nhất: PG `23505`, MySQL `1062`.

### 4.2 Danh sách bảng (14 bảng nghiệp vụ + 2 hạ tầng)

**Migration dự kiến** (số có thể dịch khi merge; thứ tự tương đối cố định):

| Migration | Bảng | CR tạo |
|---|---|---|
| `0001_init` | `outbox_events`, `processed_events` | CR-010 |
| `0002_code_intel_core` | `tenant_settings`, `repo_bindings`, `graph_snapshots`, `review_states`, `finding_dismissals`, `c4_overrides`, `reindex_jobs` — **đủ cột của hợp đồng này** (không `ALTER` sau) | CR-011 |
| `0003_quality_runs_and_findings` | `quality_runs` (kể cả cột CI), `quality_findings` | CR-082 |
| `0004_quality_gate` | `quality_profiles`, `quality_waivers`, `quality_trend_points` | CR-085 |
| `0005_coverage_reports` | `coverage_reports` | CR-083 |
| `0006_agent_turns` | `agent_turns` | CR-089 |
| `0007_requirement_trace_links` | `requirement_trace_links` | CR-092 |

#### T0. Hạ tầng
| Bảng | Cột | Khoá / chỉ mục |
|---|---|---|
| `outbox_events` | `id uuid PK`, `tenant_id`, `subject text NN`, `occurred_at ts NN`, `version int NN`, `payload json NN`, `created_at ts NN default now`, `published_at ts NULL` | PG: chỉ mục từng phần `(created_at) WHERE published_at IS NULL`; MySQL `(published_at, created_at)`; chính sách relay `app.relay='on'` (`FOR SELECT`, `FOR UPDATE`); giữ 7 ngày sau publish |
| `processed_events` | `tenant_id`, `event_id`, `subject text`, `processed_at ts` | PK `(tenant_id, event_id)`; chỉ mục `processed_at`; dọn 7 ngày |

#### T1. `tenant_settings` (CR-011; cột cờ gom từ 073, 080, 085, 089, 091, 093, 037)
| Cột | Kiểu | Ghi chú |
|---|---|---|
| `tenant_id` | uuid PK | |
| `code_intel_enabled` | bool NN, **không default** | use case luôn chèn giá trị từ `CODEINTEL_TENANT_DEFAULT_ENABLED` (mặc định `false`) |
| `quality_gate_enabled` | bool NN default false | chỉ hiệu lực khi `code_intel_enabled` |
| `quality_security_scan_enabled` | bool NN default false | CR-091 |
| `index_policy` | v16 NN default `auto_in_place` | `auto_in_place|per_worktree|off` (CR-080) |
| `ai_review_level` | v8 NN default `off` | `off|metadata|diff` (CR-093) |
| `ai_review_model` | v64 NN default '' | allowlist tiền tố (CR-093) |
| `agent_turn_store_prompt_excerpt` | bool NN default false | CR-089 |
| `agent_claim_text_enabled` | bool NN default false | CR-089 |
| `hotspot_window_days` | smallint NN default 90 | 30..365 (CR-037) |
| `updated_by` | uuid NULL (NULL = hệ thống) | |
| `updated_at` | ts NN | |

#### T2. `repo_bindings` (CR-011/012)
`id uuid PK`, `tenant_id`, `project_id uuid NN`, `repo_id uuid NN`, `worktree_id uuid NULL` (chỉ khi worktree do Orca ghi), `worktree_ref v255 NN` (chuỗi đã chuẩn hoá, PQ-04), `scope_key v128 NN` (`wt:<worktree_id>` | `path:<repo_id>:<sha256 hex đường dẫn chuẩn hoá>`), `dev_server_id uuid NN`, `workspace_root text NN` (tuyệt đối trên dev server), `path_hash char(64) NN`, `gitnexus_repo v255 NN default ''`, `codegraph_path text NN default ''`, `index_scope v16 NN` (CHECK `exact|repo_root|unresolved`), `last_status json NULL` (≤ 64 KiB), `last_status_at ts NULL`, `created_at`, `updated_at`, `version bigint NN default 1`.
Khoá/chỉ mục: UNIQUE `(tenant_id, project_id, scope_key)`; `(tenant_id, dev_server_id, path_hash)`; `(tenant_id, repo_id)`. Bảo trì: xoá binding rảnh 90 ngày; consumer `orca.project.worktree.deleted` → `DeleteByWorktreeID`.

#### T3. `graph_snapshots` (CR-011/022; PQ-15)
`id uuid PK`, `tenant_id`, `repo_binding_id uuid NN`, `view v32 NN`, `head_commit v64 NN`, `params_hash char(64) NN`, `schema_version int NN`, `etag char(32) NN`, `total_count bigint NN default 0`, `payload json NN` (≤ `CODEINTEL_SNAPSHOT_MAX_BYTES`, mặc định 3 MiB, tối đa 8 MiB), `payload_bytes int NN` (CHECK `≤ 16777216`), `truncated bool NN`, `tool_versions json NN` (mảng `SourceInfo`), `created_at ts NN`, `expires_at ts NN`.
Khoá/chỉ mục: UNIQUE `(tenant_id, repo_binding_id, view, head_commit, params_hash)`; `(expires_at)`; `(tenant_id, repo_binding_id, created_at)`. Upsert: PG `ON CONFLICT … DO UPDATE`; MySQL `ON DUPLICATE KEY UPDATE`. Tập `view` lưu DB: `structure`, `architecture`, `clusters`, `flows`, `flow`, `subgraph`, `impact`, `routes`, `changeOverlay`, `readingOrder`, `erd`, `storage`, `dataflows`, `dataflow`, `findings`, `contractDiff`, `contractCatalog`, `requirementTrace`, `aiSummary`; **không lưu** `status` (bộ nhớ 15 s) và `symbol` (LRU bộ nhớ). TTL 7 ngày; ≤ 3 commit mỗi `(binding, view)`; 64 MiB/binding, 512 MiB/tenant.

#### T4. `review_states` (CR-011/052/060; PQ-22)
`id uuid PK`, `tenant_id`, `repo_binding_id uuid NN`, `base_commit v64 NN`, `head_commit v64 NN` (hai trường rỗng = dòng mức worktree giữ `turn_markers`), `reading_progress json NN` (≤ 64 KiB), `notes json NN` (≤ 256 KiB, ≤ 500 mục), `turn_markers json NN` (≤ 256 KiB, ≤ 5 phần tử), `status v16 NN default 'open'` (`open|reviewed`), `updated_by uuid NN`, `updated_at ts NN`, `version bigint NN default 1`.
UNIQUE `(tenant_id, repo_binding_id, base_commit, head_commit)`. Một dòng cho cả nhóm (tiến độ riêng người là điểm mở).

#### T5. `finding_dismissals` (PQ-05)
`id uuid PK`, `tenant_id`, `repo_id uuid NN`, `repo_binding_id uuid NN`, `finding_key v128 NN`, `disposition v16 NN` (`ignored|resolved`), `reason v500 NN default ''`, `note v500 NN default ''`, `dismissed_by uuid NN`, `at ts NN`. UNIQUE `(tenant_id, repo_id, finding_key)`; chỉ mục `(tenant_id, repo_id)`. Khôi phục = xoá dòng; audit ở CR-013.

#### T6. `c4_overrides` (CR-011/033)
`id uuid PK`, `tenant_id`, `repo_id uuid NN`, `repo_binding_id uuid NN`, `container v255 NN`, `document text NN` (≤ 64 KiB nghiệp vụ, ≤ 128 KiB cột; MySQL `MEDIUMTEXT`), `updated_by uuid NN`, `updated_at ts NN`, `version bigint NN default 1`. UNIQUE `(tenant_id, repo_id, container)`.

#### T7. `reindex_jobs` (CR-011/013/004/080; PQ-16)
`id uuid PK`, `tenant_id`, `repo_binding_id uuid NN`, `mode v16 NN` (`incremental|full`), `status v16 NN` (CHECK `queued|running|succeeded|failed|cancelled`), `stage v32 NN default ''`, `percent smallint NULL` (CHECK 0..100), `outcome v32 NN default ''`, `trigger v16 NN default 'manual'` (`manual|agent_done|head_change|schedule`), `trigger_event_id v64 NULL`, `requested_by uuid NULL` (NULL = hệ thống), `agent_job_id v64 NN default ''` (ULID `ri_…`), `active_key uuid NULL` **UNIQUE** (= `repo_binding_id` khi `queued|running`, NULL khi kết thúc), `message v500 NN default ''` (đã che), `error_code v64 NN default ''`, `created_at`, `started_at NULL`, `finished_at NULL`, `updated_at`, `version bigint NN default 1`.
Chỉ mục: `(tenant_id, repo_binding_id, created_at)`; `(status, updated_at)`. Job mồ côi (`queued|running` quá `CODEINTEL_REINDEX_STALE_AFTER`=45 phút) → `failed` với `error_code='CODEINTEL_REINDEX_ORPHANED'`.

#### T8. `quality_runs` (CR-082 + 086; PQ-25)
`id uuid PK`, `tenant_id`, `repo_id uuid NN`, `repo_binding_id uuid NN`, `worktree_ref v255 NN`, `agent_run_id v64 NN` (ULID agent hoặc `ci:<provider>:<sha>`), `head_commit v64 NN`, `index_commit v64 NN default ''`, `index_basis json NULL` (≤ 4 KiB), `scope v16 NN` (CHECK `worktree|changed|commitRange`), `base_commit v64 NN default ''`, `profile v64 NN`, `status v16 NN` (CHECK `queued|running|succeeded|failed|cancelled`), `source v8 NN` (CHECK `local|ci`), `dirty bool NN default false`, `dirty_fingerprint v80 NN default ''` (= `treeFingerprint` trên dây), `work_tree_changed bool NN default false`, `scope_widened bool NN default false`, `error_count/warning_count/info_count int NN default 0` (trước cắt), `findings_stored int NN default 0`, `findings_truncated bool NN default false`, `steps json NN` (≤ 64 KiB), `error_code v64 NN default ''`, `provider v16 NULL`, `external_ref json NULL`, `external_url v1024 NULL`, `fetched_at ts NULL`, `stale_after ts NULL`, `requested_by uuid NULL`, `active_key uuid NULL` **UNIQUE** (= `repo_binding_id` khi `queued|running`; run CI không đặt), `started_at NULL`, `finished_at NULL`, `created_at`, `updated_at`, `version bigint NN default 1`.
UNIQUE `(tenant_id, repo_binding_id, agent_run_id)`; chỉ mục `(tenant_id, repo_binding_id, created_at)`, `(tenant_id, repo_id, head_commit)`, `(status, updated_at)`. Giữ 180 ngày; mồ côi quá `CODEINTEL_QUALITY_RUN_STALE_AFTER`=60 phút → `failed` với `CODEINTEL_QUALITY_RUN_ORPHANED`.

#### T9. `quality_findings` (CR-082)
`id uuid PK`, `tenant_id`, `run_id uuid NN`, `repo_id uuid NN`, `ordinal int NN`, `step_id v96 NN`, `fingerprint char(32) NN`, `fp_version smallint NN default 1`, `rule_id v128 NN`, `severity v8 NN` (CHECK), `category v16 NN` (CHECK 10 giá trị), `file v1024 NN default ''` (**không chỉ mục**), `line/end_line/col/end_col int NN default 0`, `message v2048 NN`, `tool v32 NN`, `tool_version v32 NN default ''`, `fix_hint v512 NN default ''`, `in_scope bool NN default true`, `created_at ts NN`. UNIQUE `(tenant_id, run_id, ordinal)`; chỉ mục `(tenant_id, repo_id, fingerprint)`. Giữ 30 ngày (xoá lô 500). Nạp: `ON CONFLICT (tenant_id, run_id, ordinal) DO NOTHING` / MySQL `ON DUPLICATE KEY UPDATE id=id`.

#### T10. `quality_profiles` (CR-085)
`id uuid PK`, `tenant_id`, `scope_key v160 NN` (`tenant` | `repo:<repo_id>`), `repo_id uuid NULL`, `name v64 NN default 'default'` (`[a-z0-9-]`), `mode v8 NN` (CHECK `inform|block`; chỉ `inform` ở MVP), `definition json NN` (≤ 64 KiB), `updated_by uuid NN`, `updated_at ts NN`, `version bigint NN default 1`. UNIQUE `(tenant_id, scope_key, name)`. Cấp hiệu lực `repo` > `tenant` > `builtin orca-default` (không trộn từng trường).

#### T11. `quality_waivers` (CR-085)
`id uuid PK`, `tenant_id`, `repo_id uuid NN`, `subject_kind v24 NN` (CHECK `finding|structure_finding|check`), `subject_key v255 NN`, `scope_key v160 NN` (`repo` | `binding:<repo_binding_id>`), `reason text NN` (1–1000 ký tự; `check` ≥ 20), `created_by uuid NN`, `created_at ts NN`, `expires_at ts NN` (≤ 30 ngày; `member` ≤ 7 ngày), `revoked_at ts NULL`, `revoked_by uuid NULL`, `active_key char(64) NULL` **UNIQUE** (sha256 `tenant|repo|kind|key|scope` khi chưa thu hồi), `version bigint NN default 1`. Hiệu lực = `revoked_at IS NULL AND expires_at > now()` (đồng hồ DB). Dọn sau 365 ngày.

#### T12. `quality_trend_points` (CR-085)
`id uuid PK`, `tenant_id`, `repo_binding_id uuid NN`, `head_commit v64 NN`, `base_commit v64 NN default ''`, `turn_key v128 NN default ''`, `profile_ref v160 NN`, `verdict v8 NN` (CHECK `pass|warn|fail|unknown`), `counts json NN` (≤ 4 KiB), `metrics json NN`, `run_ids json NN` (≤ 32), `index_commit v64 NN default ''`, `source v8 NN default 'local'`, `created_at ts NN`. UNIQUE `(tenant_id, repo_binding_id, head_commit, turn_key, profile_ref, source)` (thêm `source`: lệch với CR-085 để `local`/`ci` không ghi đè nhau, giải điểm mở Q7 của CR-085); chỉ mục `(tenant_id, repo_binding_id, created_at)`. Giữ ≤ 200 điểm/binding và 90 ngày.

#### T13. `coverage_reports` (CR-083)
`id uuid PK`, `tenant_id`, `quality_run_id uuid NN`, `repo_binding_id uuid NN`, `worktree_ref v255 NN`, `head_commit v64 NN`, `index_commit v64 NN default ''`, `base_commit v64 NN default ''`, `dirty bool NN`, `tree_hash v80 NN default ''`, `language v8 NN` (`go|ts`), `scope_key v255 NN` (module/gói), `source v16 NN` (`measured|estimated`), `mode v8 NN` (`set|count|atomic|v8`), `total_stmts int NN`, `covered_stmts int NN`, `diff_executable int NN`, `diff_covered int NULL`, `payload json NN` (≤ 1 MiB), `payload_bytes int NN`, `truncated bool NN`, `tool_versions json NN`, `created_at`, `expires_at`. UNIQUE `(tenant_id, repo_binding_id, head_commit, dirty, tree_hash, scope_key, source)`; chỉ mục `(tenant_id, repo_binding_id, created_at)`. Giữ 30 ngày hoặc 20 báo cáo/worktree.

#### T14. `agent_turns` (CR-089; PQ-22)
`id uuid PK`, `tenant_id`, `repo_binding_id uuid NN`, `client_turn_id v128 NN`, `agent_type v64 NN default 'unknown'`, `model v128 NULL`, `model_source v16 NN default 'unknown'`, `agent_session_id uuid NULL`, `source v8 NN` (`renderer|hook|both`), `started_at ts NULL`, `ended_at ts NN`, `interrupted bool NN default false`, `base_head_commit v64 NULL`, `end_head_commit v64 NN`, `tree_dirty_end bool NN`, `files_changed_count int NN default 0`, `files_digest char(64) NN default ''`, `prompt_digest char(64) NN default ''`, `prompt_excerpt v160 NULL`, `commands_summary json NN` (≤ 8 KiB), `claims json NN` (≤ 8 KiB), `verification json NN` (≤ 8 KiB), `created_at`, `updated_at`, `expires_at ts NN`, `version bigint NN default 1`. UNIQUE `(tenant_id, repo_binding_id, client_turn_id)`; chỉ mục `(tenant_id, repo_binding_id, ended_at DESC)`, `(tenant_id, repo_binding_id, end_head_commit)`. Giữ ≤ 200 lượt/binding và 90 ngày (văn bản 30 ngày); xoá binding → xoá lượt.

#### T15. `requirement_trace_links` (CR-092)
`id uuid PK`, `tenant_id`, `repo_id uuid NN`, `scope_key v160 NN` (`worktree:<repo_binding_id>` | `repo`), `requirement_key v160 NN`, `link_kind v24 NN` (`evidence_confirm|evidence_reject|worktree_task`), `evidence_kind v24 NN default ''`, `evidence_ref v255 NN default ''`, `task_id uuid NULL`, `by_user uuid NN`, `at ts NN`, `version bigint NN default 1`. UNIQUE `(tenant_id, repo_id, scope_key, requirement_key, link_kind, evidence_kind, evidence_ref)`. Không hết hạn ở MVP.

> Không có bảng `erd_links` (PQ-28), không bảng `agent_*` ngoài `agent_turns`, không bảng CI riêng (PQ-25). **`quality_waivers` và `finding_dismissals` là hai bảng khác nhau** (PQ-05).

### 4.3 Bảo trì (job nền mỗi `CODEINTEL_MAINTENANCE_INTERVAL`=10 phút, lô 500, `withMaintenanceTx`)
Xoá snapshot hết hạn/sai `schema_version`; vượt hạn mức tenant → xoá cũ nhất (không bao giờ xoá snapshot mới nhất mỗi view); xoá dữ liệu mồ côi (không còn binding) quá 7 ngày; đánh dấu job/run mồ côi; xoá binding rảnh 90 ngày; xoá outbox đã publish và `processed_events` quá 7 ngày; xoá `quality_findings` 30 ngày, `quality_runs` 180 ngày, `quality_trend_points` 90 ngày, `agent_turns` 90 ngày, `coverage_reports` 30 ngày.

---

## 5. Sự kiện (outbox + NATS)

Stream JetStream **`CODEINTEL`** subjects `["orca.codeintel.>"]` tạo ở `main.go` (`pub.EnsureStream(ctx, "CODEINTEL", []string{"orca.codeintel.>"})`, như `INFRAFLEET`; tên chưa kiểm không trùng stream đang chạy). Phát từ outbox cùng transaction (`common/outbox`); envelope `eventbus.Event{ID, TenantID, OccurredAt, Version, Payload}`; at-least-once; consumer idempotent bằng `processed_events (tenant_id, event_id)`; consumer tạm `SubscribeEphemeral` mỗi replica (`common/eventbus/eventbus.go:196` theo CR-024; chưa kiểm). Payload JSON dưới đây là chuẩn (CR chưa định nghĩa phần lớn payload):

| Subject | Payload (`snake_case`, JSON trong outbox) | Phát khi | Consumer |
|---|---|---|---|
| `orca.codeintel.index.changed` | `{event_id, repo_binding_id, project_id, worktree_ref, tools[], commit, head_commit, indexed_at, reason, index_scope, freshness, stale}` | gộp 2 s (tối đa 10 s) sau `codeintel.indexChanged`/`resync`; `event_id` = UUID v5 của `tenant|binding|tools|commit|indexedAt|kind` | gateway push, 085 (ghi điểm khi `stale` đổi), 089 |
| `orca.codeintel.reindex.started` | `{job_id, repo_binding_id, project_id, mode, trigger, requested_by?}` | `RequestReindex` tạo job | (quan sát) |
| `orca.codeintel.reindex.finished` | `{job_id, repo_binding_id, project_id, status, outcome, error_code, finished_at}` | `Finish`/bảo trì | huỷ cache, gateway push |
| `orca.codeintel.review.saved` | `{repo_binding_id, base_commit, head_commit, version}` | `SaveReviewState` | (quan sát) |
| `orca.codeintel.quality.run_finished` | `{run_id, repo_binding_id, repo_id, head_commit, status, source, summary{error,warning,info}}` | `Finish` run (cùng transaction) | 085 (điểm xu hướng, tính lại cổng), 089 (đối chiếu), gateway |
| `orca.codeintel.quality.gate_changed` | `{repo_binding_id, head_commit, base_commit, profile_ref, previous_verdict|null, verdict, run_ids[], turn_key?, evaluated_at}` | `verdict` khác điểm gần nhất | gateway → push `codeIntel.quality.gateChanged` |
| `orca.codeintel.agent_turn.recorded` | `{repo_binding_id, turn_id, end_head_commit}` (chỉ id) | `RecordAgentTurn` | 085, 089 |

**Subject nhận từ service khác** (consumer bền, stream nguồn): `orca.project.worktree.deleted` (stream `PROJECT`, durable `code-intel-service-worktree-deleted`, CR-012); `orca.infra.agent.statusChanged` (publish trực tiếp, **at-most-once**, thiếu `worktree_id`/`dev_server_id` cho tới khi CR-080 sửa, đã kiểm trong CR); `orca.infra.terminal_session.agent_completed|agent_error` (dự phòng). Stream `CODEINTEL` cũng bắt `orca.codeintel.>` nên consumer phải **lọc** subject không thuộc mình (ví dụ `review.saved`).

**Đường không qua NATS**: thông báo agent → infra-fleet `StreamCodeIntelEvents` (gRPC) → code-intel-service (mọi replica mở luồng) → gateway `StreamCodeIntelEvents` (gRPC). `reindex_progress` và `quality_progress` đi trực tiếp (không outbox, không NATS); `index_changed` đi qua outbox + NATS để mọi replica phát.

---

## 6. Cờ tính năng, `tenant_settings` và biến môi trường

### 6.1 Cờ tenant (cột `tenant_settings`)

| Cờ (cột) | Mặc định | Ai đổi | Hiệu lực | Khi tắt |
|---|---|---|---|---|
| `code_intel_enabled` | `false` (mọi tenant mới đến GA) | admin (`SetSettings`) | `CODEINTEL_ENABLED ∧ tenant` | mọi RPC trừ `GetSettings`, `SetSettings`, `GetReindexJob` → `CODEINTEL_DISABLED` |
| `quality_gate_enabled` | `false` | admin | `∧ code_intel_enabled` | `QualityGateService` → `CODEINTEL_QUALITY_GATE_DISABLED` |
| `quality_security_scan_enabled` | `false` | admin (audit) | `∧ quality_gate_enabled` | profile `security-*`/`dependency-diff` ẩn; chạy → `CODEINTEL_PROFILE_UNKNOWN` |
| `index_policy` | `auto_in_place` | admin | tenant | `off`: không tự làm mới |
| `ai_review_level` | `off` | admin (xác nhận hai bước khi `diff`) | `∧ CODEINTEL_AI_REVIEW_ENABLED` | `CODEINTEL_AI_REVIEW_DISABLED` |
| `ai_review_model` | `''` | admin | allowlist tiền tố `claude|gpt|o*|gemini` | `CODEINTEL_INVALID_PARAMS` nếu ngoài allowlist |
| `agent_turn_store_prompt_excerpt`, `agent_claim_text_enabled` | `false` | admin | tenant | không lưu |
| `hotspot_window_days` | 90 | admin | 30..365 | — |

Cờ đọc có cache ≤ 5 s; lỗi đọc = tắt. Cờ tắt khi job reindex đang chạy: job chạy tới hết, RPC mới bị từ chối; `codeintel.indexChanged` vẫn xử lý (huỷ cache); dữ liệu giữ nguyên. **Frontend không mở `codeIntel.subscribe` khi `settings.get` báo tắt.**

### 6.2 Biến môi trường (PQ-23)

| Thành phần | Biến | Mặc định |
|---|---|---|
| code-intel-service (công tắc) | `CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_ENABLED`, `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED` | `false` |
| code-intel-service (hạ tầng) | `GRPC_PORT` 9090, `HTTP_PORT` 8080, `DATABASE_DSN`, `DATABASE_CREDENTIALS_FILE`, `NATS_URL`, `OTLP_ENDPOINT`, `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR`, `OPA_BUNDLE_PATH`, `CODEINTEL_INTERNAL_CALLER_TOKEN` (rỗng = chặn hết) | theo CR-010 |
| code-intel-service (giới hạn) | các biến `CODEINTEL_*` của CR-010/011/013/021/022/024/080/082 (bảng ở các CR) | theo CR |
| api-gateway | `CODE_INTEL_SERVICE_ADDR` (rỗng = cắt, mọi kênh `CODEINTEL_UNAVAILABLE`), `CODE_INTEL_MAX_RESPONSE_BYTES` 2 MiB, `CODE_INTEL_MAX_STREAMS` 500 | CR-040 |
| agent | `ORCA_CODEINTEL_*`, `ORCA_QUALITY_*`, `ORCA_HEAVY_JOBS` (`ORCA_CODEINTEL_DISABLED=1` tắt cứng) | xem agent contract §2.4 |
| compose/CI | service `code-intel-service`, job `migrate-codeintel`, container `orca-go-code-intel`, DB `codeintel`; `backend-go/ci/check-code-intel-service-wiring.sh` | CR-010, 073 |

### 6.3 Quyền OPA (`backend-go/policy/orca-authz/code_intel.rego`)
`read`, `read_source`, `review_write`, `reindex`, `c4_write`, `quality_read`, `quality_waive`, `quality_profile_write` — bảng vai trò ở CR-013 §2.2 và CR-085 §2.9 (owner/member/admin; `c4_write` và `quality_profile_write` chỉ owner/admin; `quality_waive` member tối đa 7 ngày, không miễn `check`/`error`). Chạy kiểm: `GetProject` → `ListMembers` → OPA, cache 10 s; lỗi tra cứu = từ chối (`CODEINTEL_AUTHZ_UNAVAILABLE` khi hạ tầng lỗi).

---

## 7. Thứ tự phụ thuộc giữa các solution

### 7.1 Cổng đồng bộ hợp đồng (làm trước, để ba khu vực song song)

| Cổng | Nội dung | Mở khoá |
|---|---|---|
| **G0** | CR-010 (khung service, `codeintel.proto` rỗng-có-health, `apperrors` Kind mới) + CR-020 (`codeintel_common.proto`, `codeintel_graph.proto`) + stub Go sinh; `WorktreeSelector`, `ResultMeta`, `SymbolRef` | mọi proto khác, collector, gateway |
| **G1** | Tệp vàng agent (CR-070 `testdata/agent-results/*.json` + `mini-repo`) theo `CONTRACT-codeintel-agent-rpc.md` | backend collector không cần dev server thật; frontend fake backend |
| **G2** | CR-023 (timeout, thông báo, `StreamCodeIntelEvents`, `GetAgentCapabilities`, `AgentRPCError`) | CR-021, 024, 080 |
| **G3** | CR-040 phần nền (đăng ký vô điều kiện 46 kênh, giải mã chặt, `codeIntelChannelError`, `SetReadLimit`, parity/loại trừ MCP) với service giả trả `CODEINTEL_UNAVAILABLE` | frontend bridge thật |
| **G4** | CR-050 bridge + fake backend `code-intel-fake-backend.ts` (CR-073 T2) theo `CONTRACT-codeintel-ui-api.md` | mọi lens frontend |

### 7.2 Thứ tự theo khu vực (backend → agent → frontend)

**Backend (`specs/backend-go/crs/v7`)**
```
010 → 011 → 012 → 013
020 ─┐
023 ─┴→ 021 → 022 → 024
030 → 031 → 032 → 033 → 034 → 035 ; 036 (cần 005 (AG), 020, 021, 030) → 037 → 038
040 (nền G3 sau 010/020; kênh đầy đủ sau các RPC) → 041
073 (cờ/settings; sau 013) ; 070-BE (sau 021) ; 071, 072-BE (sau lõi ổn định)
080-BE (sau 004 AG, 012, 024) ; 082-BE (sau 011) → 085 (sau 011, 013, 036, 037) → 089, 090, 092, 093, 095-BE
083-BE (sau 082) ; 086 (sau 082, 081) ; 091-BE (sau 085)
```
**Agent (`specs/agent/crs/v7`)**
```
001 → 002 → 005 ; 002 → 004 ; 001 → 003 ; 070-AG (sau 002/003) ; 072-AG ; 071-AG
037-AG (structuralFacts, sau 002) ; 006 (P2, sau 001–005)
080-AG (sau 004) ; 081 (sau 001) → 082-AG → 083-AG, 084, 091-AG ; 073-AG (kill-switch)
```
**Frontend (`specs/frontend/crs/v7`)** — chỉ bắt đầu dùng kênh thật sau G3; trước đó dùng fake backend (G4):
```
050 → 051 → 052/053 → 054..060 → 061 → 062 ; 088 (sau 050) → 087 (sau 051, 053, 059, 085-BE)
085-FE (sau 051) ; 089-FE (sau 060) ; 090-FE (sau 051) ; 092-FE, 093-FE (sau 085-BE) ; 095-FE (sau 061, 059) ; 073-FE
```

### 7.3 Đợt (README v7 §5, đã chỉnh theo hợp đồng này)

| Đợt | BE | AG | FE | Kết quả chạy được |
|---|---|---|---|---|
| 1 | 010, 011, 020, 023 | 001, 002 | — | service chạy; agent trả `status`/`overview` thật |
| 2 | 012, 013, 021, 030, 031, 040 (nền+đọc) | 005, 070-AG | 050 (bridge, fake) | gọi được qua gateway; ERD từ migration |
| 3 | 036, 073 | — | 051, 052, 053, 057, 061 | **MVP review** |
| 4 | 022, 024, 032, 033, 034 | 003, 004 | 054, 055, 056 | cache, sự kiện, C4, luồng |
| 5 | 037, 038, 040 (ghi+stream), 070-BE, 071, 072 | 037-AG, 071-AG, 072-AG | 059, 060 | phát hiện, hợp đồng, phản hồi agent, kiểm thử |
| 6 | 035, 041 | 006 | 058, 062 | lưu trữ, MCP, mobile, SSH |
| 7 | 080-BE, 082-BE | 080-AG, 081, 082-AG | 088 | index bắt kịp, chạy kiểm tra |
| 8 | 085, 083-BE, 086 | 083-AG, 084 | 085-FE, 087 | cổng chất lượng chế độ chỉ báo |
| 9 | 089, 090, 091, 092, 093 | 091-AG | 089-FE…095 | provenance, báo cáo, bảo mật, truy vết, AI |

---
## 8. ID và quy ước đặt tên cho các nhóm sau

### 8.1 Quy ước

| Loại | Mẫu tên file | Nơi đặt |
|---|---|---|
| Solution backend | `BE-CV-SOL-<CR>-<slug>.md` | `specs/backend-go/crs/v7/<feature>/solutions/` |
| Task backend | `BE-CV-TASK-<CR>-<NN>-<slug>.md` | `specs/backend-go/crs/v7/<feature>/tasks/` |
| Solution frontend | `FE-CV-SOL-<CR>-<slug>.md` | `specs/frontend/crs/v7/<feature>/solutions/` |
| Task frontend | `FE-CV-TASK-<CR>-<NN>-<slug>.md` | `specs/frontend/crs/v7/<feature>/tasks/` |
| Solution agent | `AG-CV-SOL-<CR>-<slug>.md` | `specs/agent/crs/v7/<feature>/solutions/` |
| Task agent | `AG-CV-TASK-<CR>-<NN>-<slug>.md` | `specs/agent/crs/v7/<feature>/tasks/` |

- `<CR>` = 3 chữ số (`001`, `085`). `<NN>` = 2 chữ số, **tăng liên tục trong cùng (khu vực, CR)** kể cả khi CR có nhiều solution (mỗi task ghi solution mẹ ở đầu file). `<slug>` = kebab-case, mô tả **khái niệm cụ thể**; **cấm** `helpers`, `utils`, `common`, `misc`, `shared-stuff` (`AGENTS.md`).
- `<feature>` **trùng tên thư mục** trong `docs/crs/v7/`: `agent-codeintel`, `code-intel-service-foundation`, `code-intel-graph-pipeline`, `code-intel-sources`, `code-intel-gateway`, `review-frontend`, `quality-rollout`, `quality-signals`, `quality-gate`, `quality-visualization`. Một CR có công việc ở khu vực nào thì tạo solution ở khu vực đó, **dưới cùng tên feature**.
- Mỗi CR ở mỗi khu vực có **≥ 1** solution; chia thành nhiều solution khi CR "Large" có nhiều chủ đề độc lập (bảng 8.2). Mỗi solution 3–9 task; mỗi `solutions/` và `tasks/` có `README.md` (bảng CR → solution → task, sơ đồ phụ thuộc, quyết định chung).
- Mọi task mở đầu bằng `Status: [ ] TODO`; đường dẫn file mới ghi "(mới)"; phần chưa chạy ghi "chưa kiểm chứng". Mọi solution/task **phải trích** `CONTRACT-codeintel-*` và `PQ-xx` thay vì chép lại kiểu dữ liệu; nếu cần đổi hợp đồng thì sửa file hợp đồng trước (PR riêng) rồi mới sửa solution.
- Task frontend chạm `desktop/src/preload/index.ts` hoặc `desktop/src/main/runtime/**` (CR-050, 062) vẫn đặt dưới `specs/frontend/crs/v7/` và ghi rõ "ngoài `frontend/`, cần chủ sở hữu desktop duyệt".
- Tuân `AGENTS.md`: không `max-lines` disable; UI theo `guides/STYLEGUIDE.md` (đường dẫn thật là `guides/`, không phải `docs/`); mọi chuỗi UI qua `translate()`; mọi lệnh Git theo `guides/reference/git-compatibility.md` (baseline 2.25, `GitCapabilityCache`).

### 8.2 Ánh xạ CR → khu vực và solution

Ký hiệu: **BE** backend-go, **FE** frontend (kể cả desktop/mobile), **AG** agent; `—` = không có việc ở khu vực đó; nhiều solution cách nhau bằng dấu `;`.

#### Feature `agent-codeintel`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 001 | Nền `codeintel.*` trên agent | — | — | `AG-CV-SOL-001-codeintel-agent-foundation` |
| 002 | Trích xuất GitNexus | — | — | `AG-CV-SOL-002-gitnexus-extraction` |
| 003 | Trích xuất CodeGraph | — | — | `AG-CV-SOL-003-codegraph-extraction` |
| 004 | Làm mới index và thông báo | — | — | `AG-CV-SOL-004-reindex-and-index-notifications` |
| 005 | `detectChanges` | — | — | `AG-CV-SOL-005-detect-changes` |
| 006 | `relay-ssh` Part B (P2) | — | — | `AG-CV-SOL-006-relay-ssh-part-b-handlers` (gồm sửa `dispatcher.ts` ở **cả** `agent/` và `desktop/src/relay/`) |

#### Feature `code-intel-service-foundation`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 010 | Dựng `code-intel-service` | `BE-CV-SOL-010-scaffold-code-intel-service` (gồm wiring go.work/compose/CI/migrate, `apperrors` Kind mới) | — | — |
| 011 | Mô hình dữ liệu, migration, repository | `BE-CV-SOL-011-data-model-and-migrations`; `BE-CV-SOL-011-repositories-and-maintenance` | — | — |
| 012 | Ánh xạ project/worktree → repo | `BE-CV-SOL-012-target-resolution-and-bindings`; `BE-CV-SOL-012-index-status-aggregation` | — | — |
| 013 | Phân quyền, audit, hạn mức | `BE-CV-SOL-013-authorization-flags-and-audit`; `BE-CV-SOL-013-agent-call-gate-and-quotas` | — | — |

#### Feature `code-intel-graph-pipeline`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 020 | Mô hình graph chuẩn | `BE-CV-SOL-020-canonical-graph-model` (proto common/graph + domain `SymbolRef`) | — | — |
| 021 | Collector | `BE-CV-SOL-021-agent-collector` (gồm `codeintel_reindex.proto`) | — | — |
| 022 | Cache snapshot | `BE-CV-SOL-022-snapshot-cache` | — | — |
| 023 | Vận chuyển ở infra-fleet | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | — | — |
| 024 | Phân phối sự kiện | `BE-CV-SOL-024-event-distribution` | — | — |

#### Feature `code-intel-sources`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 030 | Cổng đọc file repo | `BE-CV-SOL-030-repo-file-access-gateway` | — | — |
| 031 | SQL → ERD | `BE-CV-SOL-031-sql-migration-parser`; `BE-CV-SOL-031-erd-model-and-access-scan` | — | — |
| 032 | Proto + danh mục `wscompat` | `BE-CV-SOL-032-proto-and-wscompat-contract-catalog` | — | — |
| 033 | C4 level 3 | `BE-CV-SOL-033-c4-component-view`; `BE-CV-SOL-033-c4-overrides-yaml` | — | — |
| 034 | Luồng dữ liệu | `BE-CV-SOL-034-data-flow-model` | — | — |
| 035 | Bản đồ lưu trữ | `BE-CV-SOL-035-storage-map` | — | — |
| 036 | Change overlay | `BE-CV-SOL-036-change-overlay-pipeline`; `BE-CV-SOL-036-reading-order-and-risk` | — | — |
| 037 | Phân tích cấu trúc | `BE-CV-SOL-037-structure-findings-and-dismissals` | — | `AG-CV-SOL-037-structural-facts` |
| 038 | Contract diff + bảo mật tĩnh | `BE-CV-SOL-038-contract-diff`; `BE-CV-SOL-038-static-tenant-filter-rule` | — | — |

#### Feature `code-intel-gateway`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 040 | Kênh `codeIntel.*` | `BE-CV-SOL-040-codeintel-channel-foundation` (wiring, giải mã chặt, lỗi, giới hạn, read limit, parity); `BE-CV-SOL-040-codeintel-view-channels`; `BE-CV-SOL-040-codeintel-write-and-stream-channels`; `BE-CV-SOL-040-codeintel-quality-channels` (20 kênh quality) | — | — |
| 041 | Tool MCP (P2) | `BE-CV-SOL-041-mcp-codeintel-tools` | — | — |

#### Feature `review-frontend`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 050 | Nền frontend | — | `FE-CV-SOL-050-types-and-runtime-bridge`; `FE-CV-SOL-050-store-and-query-hooks`; `FE-CV-SOL-050-review-tab-wiring` | — |
| 051 | Khung màn Review | — | `FE-CV-SOL-051-review-workspace-shell` | — |
| 052 | Thứ tự đọc, tiến độ | — | `FE-CV-SOL-052-reading-order-and-progress` | — |
| 053 | Lens Ảnh hưởng | — | `FE-CV-SOL-053-impact-lens-and-symbol-detail` | — |
| 054 | Lens Cấu trúc | — | `FE-CV-SOL-054-structure-lens` | — |
| 055 | Lens C4 | — | `FE-CV-SOL-055-architecture-c4-lens` | — |
| 056 | Lens Luồng dữ liệu | — | `FE-CV-SOL-056-dataflow-lens` | — |
| 057 | Lens ERD | — | `FE-CV-SOL-057-erd-lens` | — |
| 058 | Lens Lưu trữ | — | `FE-CV-SOL-058-storage-lens` | — |
| 059 | Lens Hợp đồng + Phát hiện | — | `FE-CV-SOL-059-contract-lens-and-findings` | — |
| 060 | Ghi chú, gửi agent, so sánh lượt | — | `FE-CV-SOL-060-review-notes-and-turn-compare` | — |
| 061 | Điểm vào | — | `FE-CV-SOL-061-review-entry-points` | — |
| 062 | Mobile | — | `FE-CV-SOL-062-mobile-review-summary` (gồm host method desktop) | — |

#### Feature `quality-rollout`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 070 | Fixture vàng + hợp đồng công cụ | `BE-CV-SOL-070-collector-golden-contract` | — | `AG-CV-SOL-070-golden-fixtures-and-parsers` |
| 071 | Ngân sách, metrics, tracing | `BE-CV-SOL-071-metrics-tracing-and-budgets` | — | `AG-CV-SOL-071-perf-block-and-bench` |
| 072 | Kiểm thử bảo mật | `BE-CV-SOL-072-security-tests-service-gateway` | — | `AG-CV-SOL-072-security-tests-agent` |
| 073 | E2E, cờ, rollout | `BE-CV-SOL-073-settings-flag-and-rollout` | `FE-CV-SOL-073-flag-gating-and-web-e2e` | `AG-CV-SOL-073-agent-kill-switch` |

#### Feature `quality-signals`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 080 | Index cho worktree agent + tự làm mới | `BE-CV-SOL-080-auto-refresh-index` | — | `AG-CV-SOL-080-index-basis-and-reindex-triggers` |
| 081 | Bộ chạy kiểm tra | — | — | `AG-CV-SOL-081-quality-runner-core`; `AG-CV-SOL-081-quality-profile-catalog-and-preflight` |
| 082 | `QualityFinding` + parser | `BE-CV-SOL-082-quality-run-storage-and-ingest` | — | `AG-CV-SOL-082-quality-parsers-and-fingerprint` |
| 083 | Coverage | `BE-CV-SOL-083-coverage-storage-and-diff` | — | `AG-CV-SOL-083-coverage-collection` |
| 084 | Rule pack | — | — | `AG-CV-SOL-084-convention-rule-pack` |
| 086 | Gộp CI/PR | `BE-CV-SOL-086-scm-commit-checks`; `BE-CV-SOL-086-ci-run-merge-and-comparison` | — | — |
| 091 | Quét bảo mật (P2) | `BE-CV-SOL-091-security-scan-flag-and-ingest` | — | `AG-CV-SOL-091-security-and-dependency-profiles` |
| 094 | Spike tool GitNexus | **Không có solution**: spike tài liệu của một người (5 ngày), kết quả vào `docs/research/view-code/spikes/094-gitnexus-unused-tools/`; nếu "đi" thì cập nhật CR-037/038/001/002/041 trước khi viết solution | — | — |

#### Feature `quality-gate`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 085 | Cổng chất lượng | `BE-CV-SOL-085-quality-gate-evaluator-and-profiles`; `BE-CV-SOL-085-waivers-and-trend` | `FE-CV-SOL-085-source-control-quality-notice` | — |
| 089 | Dấu vết agent | `BE-CV-SOL-089-agent-turn-provenance` | `FE-CV-SOL-089-agent-turn-recorder` | — |
| 090 | Báo cáo xuất được | `BE-CV-SOL-090-review-report-model` | `FE-CV-SOL-090-review-report-export` | — |
| 092 | Truy vết yêu cầu | `BE-CV-SOL-092-requirement-trace` | `FE-CV-SOL-092-requirement-trace-view` | — |
| 093 | Tóm tắt AI (mặc định tắt) | `BE-CV-SOL-093-ai-review-summary` (gồm ngoại lệ timeout `ai.complete` 120 s ở infra-fleet) | `FE-CV-SOL-093-ai-summary-panel` | — |
| 095 | Telemetry | (số liệu server nằm ở `BE-CV-SOL-071-…`) | `FE-CV-SOL-095-review-telemetry` | — |

#### Feature `quality-visualization`
| CR | Tên | BE | FE | AG |
|---|---|---|---|---|
| 087 | Scorecard, chú thích diff, xu hướng | — | `FE-CV-SOL-087-quality-scorecard-and-state`; `FE-CV-SOL-087-quality-diff-annotations`; `FE-CV-SOL-087-quality-trend-coverage-hotspot` | — |
| 088 | Nền đồ hoạ | — | `FE-CV-SOL-088-graphics-foundation-and-chart-primitives` | — |

**Tổng** (theo bảng): 59 CR; BE ≈ 50 solution, FE ≈ 34, AG ≈ 22; CR-CV-094 không có solution; CR-CV-006, 041, 058, 062, 091, 035, 038 là P2 (làm sau).

### 8.3 Kiểm tra chéo bắt buộc trong mỗi solution
1. Trích số hiệu `PQ-xx` mà solution tuân theo; mọi chỗ lệch CR phải liệt kê ở mục "Lệch giữa CR và hợp đồng".
2. Solution BE thêm RPC/kênh phải cập nhật bảng ở mục 3 và 3.2/`CONTRACT-codeintel-ui-api.md` §3 **trong cùng PR hợp đồng**; gateway: `TestChannelInventory` và `TestToolParity` (`excluded_channels.yaml`, dòng `codeIntel.*`) phải xanh trong cùng PR đăng ký kênh.
3. Mọi nhánh rẽ theo hai dialect có test hai dialect (ma trận CI `dialect: [postgres, mysql]`).
4. Mọi truy vấn DB có `tenant_id` và test cô lập tenant (CR-072).
5. Mọi đường chạm agent chỉ dùng method trong `CONTRACT-codeintel-agent-rpc.md` §4–5; test phản chiếu schema `validate` không có `command|argv|args|env|cwd|timeout`.
6. Chuỗi hiển thị: backend trả khoá/`code`+`params`; frontend `translate()`.

---

## 9. Điểm chưa chốt (cần người quyết định)

| # | Điểm | Hiện tạm theo | CR/PQ |
|---|---|---|---|
| **O-1** | **Frontend có `projectId` của mỗi worktree không** (PQ-04 bắt buộc `{projectId, worktreeId}`). CR-050 chỉ giả định `worktreeId`; `worktreeId` thô hay selector `id:` chưa kiểm | giả định có (CR-CV-050 phải lấy từ store dự án/`repos`) | PQ-04, 050 §6 |
| **O-2** | Nâng `SetReadLimit` của `/ws` lên 320 KiB (chạm toàn bộ kết nối WS) hay tách `reviewState.save`/`c4.save` thành nhiều lời gọi nhỏ | nâng (PQ-14) | CR-040 Q1 |
| **O-3** | Dữ liệu `data` của agent là **dạng đã chuẩn hoá** (PQ-20) — cần xác nhận khi CR-002 chạy thử `cypher` (mẫu chưa chạy: `STEP_EDGES`, `MEMBER_CLUSTER`, `SG_EDGES_AROUND`, `FILE_SYMBOLS_BATCH` dùng `IN`) | chuẩn hoá ở agent | CR-002 |
| **O-4** | Desktop main có tới được `api-gateway` không, bằng credential nào (cho `codeIntel.reviewSummary` của mobile) | mobile `unavailable` tới khi chốt | CR-062 §7.1 |
| **O-5** | Hỗ trợ `relay-ssh` (Part B) trong v7: `AGENTS.md` yêu cầu cân nhắc SSH; Go chạy `--stdio` = Part A (CR-006 kết luận) nhưng Electron/Part B (`desktop/src/relay/relay.ts`) chưa xác nhận Go có dùng | chỉ `direct-websocket` ở MVP; Part A qua `--stdio` được | CR-006 Q1 |
| **O-6** | Chiến lược index cho **worktree liên kết** (`per_worktree` hay chỉ `OVERLAY`): phép thử M1–M7 của CR-080 chưa chạy | `OVERLAY` (không `analyze` ở worktree) | CR-080 |
| **O-7** | Thư viện parse SQL và proto (tự viết hay `pg_query_go`/`protocompile`); ngưỡng điểm rủi ro CR-036; tỷ lệ báo nhầm `sql.missing-tenant-filter` (precision `high` ≥ 60% chưa có dữ liệu) | tự viết, mặc định `info` | CR-031/032/036/038 |
| **O-8** | Nguồn dấu vết lượt "B" (hook backend) — cần spike ghi envelope `agent.hook` thật | chỉ nguồn A (renderer) | CR-089 Q1 |
| **O-9** | Đường dẫn Vault/giấy phép/egress cho công cụ quét; duyệt `govulncheck`, `osv-scanner`, `gitleaks`, `@vitest/coverage-v8`, `elkjs`/`dagre`, `d3-*` (O12) | không thêm mặc định | CR-083/088/091 |
| **O-10** | Khoá `quality_trend_points` thêm `source` (lệch CR-085); xác nhận có chấp nhận | thêm `source` | PQ-33/T12 |
| **O-11** | Mở MCP (O2) cho 9 tool `codeIntel_*` (P2) và quyền `read_source` cho `codeIntel_symbol` | tắt (loại trừ `codeIntel.*`) | CR-041 |
| **O-12** | Schema `c4.yaml` v1 (CR-033) và vị trí seed trong repo; quy trình duyệt | seed tuỳ chọn, DB thắng | CR-033 Q3/Q4 |
| **O-13** | `git.branchDiff` có chấp nhận compare tổng hợp cho khoảng commit; ngữ nghĩa `head` vắng = cây làm việc của `detectChanges` (CR-005 `head` không đặt = working tree) | theo CR-005 (chưa chạy) | CR-005/051 |
| **O-14** | Quy tắc ghép `workspaceRoot` ↔ binding trên Windows/WSL/macOS (phân biệt hoa thường); agent trên Windows trả `unsupported_platform` | chỉ POSIX ở MVP | CR-001/012 |
| **O-15** | Hiệu năng/tài nguyên: mọi con số (ngân sách 071, hạn mức 013, 081 tài nguyên) là giả định một lần đo, chưa phân phối | giữ làm cấu hình | CR-071 |
| **O-16** | Chủ sở hữu bộ che secret dùng chung (`tools/redaction_rules.go` hay `domain.Redactor`) và gói `pathsafety` dùng chung giữa gateway và `mcpserver/tools` | tách gói đặt tên cụ thể (`secretmasking`, `pathsafety`) | CR-040 Q3, CR-072 Q2 |
| **O-17** | `CodeIntelService.CancelReindex`: agent có `reindexCancel` nhưng không RPC/kênh | chưa có (reindex chạy tới hết) | CR-073 |
| **O-18** | Quyền `member` được miễn `check` và quyền `reindex` cho `member` | `member` không miễn `check`; `reindex` cho `member` với hạn mức | CR-085 Q3, CR-013 Q4 |

---

## 10. Việc phải làm ở code hiện có (tổng hợp, để task không bỏ sót)

| Nơi (đã đọc) | Thay đổi | CR |
|---|---|---|
| `agent/src/shared/agent-wire-protocol.ts` (`AgentCapability` union cố định `'pty'|'fs'|'git'|'preflight'`) | mở rộng/nới thành `string` hoặc thêm `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph`, `quality` | 001, 081 |
| `agent/src/relay/agent-session-capabilities.ts` (`buildCapabilities` trả chuỗi tự do) | thêm 4 capability có điều kiện | 001, 081 |
| `agent/src/relay/agent-rpc-dispatch.ts` (`route()` chuỗi `dispatchXxxRpc`) | thêm `dispatchCodeIntelRpc`, `dispatchQualityRpc` trước nhánh `MethodNotFound`; `extractTraceFields` chỉ ghi `workspaceRoot`, method, profile | 001, 081 |
| `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (`execTimeoutForMethod` chỉ `agent.execPrompt`) | bảng timeout theo method (agent contract §2.5) | 023, 093 |
| `…/devserveragent/session.go`, `adapter/agentwsserver/server.go` (`inboundHandshakeParams` không có `tools`) | `Tools []string`; routing thông báo `codeintel.*`, `quality.*`; `Client.SubscribeCodeIntelEvents` | 023 |
| `…/usecase/relay_by_dev_server.go` (mọi lỗi `Exec` → `INFRA_AGENT_EXEC_FAILED`) | `AgentRPCError` + ánh xạ `CODEINTEL_*` + trailer | 023 |
| `backend-go/common/apperrors/apperrors.go` (8 Kind) | thêm `KindResourceExhausted`, `KindUnavailable` | 010 |
| `backend-go/services/api-gateway/internal/adapter/wscompat` (`handler.go` `invokeTimeout` 25 s, không `SetReadLimit`) | `SetReadLimit`, `registerCodeIntelChannels`, `codeIntelChannelError`, `decodeCodeIntelArgs`, subscribe | 040 |
| `backend-go/services/api-gateway/…/mcpserver/tools/excluded_channels.yaml` | dòng `codeIntel.*` cùng PR đăng ký kênh | 040 |
| `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `…/scmintegration/v1/scmintegration.proto` | RPC/message mục 2.2, 2.4 | 023, 086 |
| `backend-go/go.work`, `Makefile`, `deploy/postgres-init-databases.sh` **và** `deploy/dev/docker/postgres/init-databases.sh`, `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/{migrate.sh,build-local.sh}`, `backend-go/ci/check-opa-bundle-in-images.sh`, `.github/workflows/backend-go-code-intel-service.yml` | wiring service mới (CR-010 §2.8); chú ý `deploy/` ở **gốc repo**, không `backend-go/deploy` (README v7 điểm 17) | 010 |
| `frontend/src/shared/*`, `desktop/src/preload/index.ts` (ngoài `frontend/`, không có `mcp`), `frontend/src/renderer/src/web/*` (`window.api` là Proxy `withFallback`), `store/slices`, 18+ chỗ tab `review` | bridge, slice, tab | 050, 051 |
| `desktop/src/main/runtime/runtime-rpc.ts` (`MOBILE_RPC_METHOD_ALLOWLIST`) | `codeIntel.reviewSummary` | 062 |

---

*Kết thúc `CONTRACT-codeintel-proto-and-data-map.md`.*
