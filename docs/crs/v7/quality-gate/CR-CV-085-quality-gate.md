# CR-CV-085 — Cổng chất lượng: profile, kết luận có lý do, miễn trừ, xu hướng theo lượt

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-085 |
| **Tên** | `QualityProfile` (bảng `quality_profiles`), thuật toán kết luận `pass\|warn\|fail\|unknown` kèm `reasons[]`, miễn trừ `quality_waivers`, điểm xu hướng `quality_trend_points`, RPC `GetQualityGate/GetQualityProfile/SaveQualityProfile/WaiveFinding/GetQualityTrend`, cờ `quality_gate_enabled`, cảnh báo (không chặn) trước commit/Create PR, sự kiện `orca.codeintel.quality.gate_changed` |
| **Loại** | Feature (nghiệp vụ lõi của nhóm kiểm soát chất lượng) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-06 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-011 (bảng, `tenant_settings`), CR-CV-013 (quyền, audit), CR-CV-036 (`ChangeOverlay`: tệp/hunk đã đổi), CR-CV-037 (`Finding`, `finding_dismissals`), CR-CV-082 (`QualityRun`, `QualityFinding`), CR-CV-081 (tên profile chạy), CR-CV-073 (RPC `GetSettings/SetSettings`) |
| **Mở khoá** | CR-CV-087 (scorecard), CR-CV-089 (lượt agent), CR-CV-090 (báo cáo), CR-CV-095 (telemetry), CR-CV-093 (ngữ cảnh), CR-CV-092 |
| **Tác động** | `backend-go/services/code-intel-service` (domain, usecase, adapter hai dialect, migration, Rego), `backend-go/proto/orca/codeintel/v1/quality_gate.proto`, `api-gateway` (kênh `codeIntel.quality.*`), `frontend/src/renderer/src/components/right-sidebar/` (cảnh báo trước commit/PR). Nguồn: [research 11](../../../research/view-code/11-additions-for-quality-control.md) C1, C2, C4; O9 |

---

## 1. Bối cảnh và vấn đề

Series v7 trả lời "agent đã đổi gì và ảnh hưởng tới đâu"; chưa có chỗ nào kết luận "thay đổi này đạt chuẩn chưa" ([research 11 §1](../../../research/view-code/11-additions-for-quality-control.md)). Các CR 081–084 và 086 sinh tín hiệu (`QualityRun`, `QualityFinding`, coverage, rule pack, CI); CR này gom chúng thành một **kết luận có lý do** và quyết định ai được chỉnh ngưỡng hay miễn trừ. O9: **chế độ chỉ báo**, không chặn.

Đã đọc ngày 2026-10-06:

1. **Bảng và quyền đã có (đề xuất ở CR khác).** `finding_dismissals` khoá `(tenant_id, repo_id, finding_key)`, "khôi phục" là xoá dòng, không hết hạn (CR-CV-011 mục 2.1; CR-CV-037 2.5). CR-CV-059 dự kiến `disposition: 'ignored' | 'resolved'` nhưng bảng chưa có cột đó (CR-CV-059 mục 6). Hành động OPA hiện chỉ `read`, `read_source`, `review_write`, `reindex`, `c4_write` (CR-CV-013 mục 2.2); chưa có hành động nào cho ngưỡng hay miễn trừ.
2. **Cờ.** `tenant_settings(tenant_id, code_intel_enabled, updated_by, updated_at)`; hiệu lực = cờ tổng ∧ cờ tenant, fail closed (CR-CV-011 2.1, CR-CV-073 2.1). `quality_gate_enabled` (O9) chưa có chỗ lưu.
3. **Lượt agent.** `agentStatusByPaneKey` chỉ trong bộ nhớ renderer; CR-CV-060 mục 2.4 tự tạo `ReviewTurnMarker` với `turnId = "${paneKey}:${doneAt}"` và lưu 5 mốc trong `review_states` (đề xuất). Backend chưa có khái niệm lượt (xem CR-CV-089).
4. **Điểm chèn cảnh báo (đọc code thật).** Trong `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx`: `handleCommit` (dòng 1792, trả `boolean`), `handleCreatePullRequest` (dòng 3008; gọi `createHostedReview(...)` quanh dòng 3066), `runCreatePrIntent` (dòng 3483; chuỗi commit → push → tạo PR), và `<CreateHostedReviewComposer … createError={…} primaryAction={…} />` (dòng 5247). `CreateHostedReviewComposer.tsx` nhận `createError`, `primaryAction.disabled`, `provider: HostedReviewProvider`; `createDisabled` tính trong component (dòng ~108). `HostedReviewProvider` có nhiều giá trị (`github`, `gitlab`, `azure-devops`, `gitea`; `source-control-create-review-blocked-action.ts`), nên chữ phải theo `localizedHostedReviewCopy(...)`, không gắn "PR" cứng. `components/code-review/pr-create-dialog.tsx` là code chết (không import từ nơi sống; README v7 mục 1), **không** dùng làm điểm chèn.
5. **Hợp đồng chất lượng** (README v7 mục 3.10): `QualityGate { verdict, reasons[{check, observed, threshold, result: pass|warn|fail}], mode: inform|block, profile, basedOn{runIds, indexCommit, stale} }`. `reasons[].result` không có `unknown`, trong khi README yêu cầu `unknown` khi thiếu dữ liệu — xem mục 2.3 và "Điều chỉnh hợp đồng" ở README folder.

Vấn đề: (a) chưa có định nghĩa "đạt" lưu được và chỉnh được; (b) không có quy tắc thống nhất để gộp nhiều kiểm tra thành một kết luận mà không che mất dữ liệu thiếu; (c) chưa có cách ghi lại ngoại lệ có người/lý do/hạn; (d) chưa có lịch sử để thấy xu hướng; (e) người dùng không được báo ở đúng chỗ họ sắp commit/tạo PR.

## 2. Giải pháp đề xuất

### 2.1 `QualityProfile` và bảng `quality_profiles` (mới)

Một profile là tài liệu JSON có phiên bản schema, lưu theo phạm vi `tenant` (mặc định cho mọi repo) hoặc `repo`. Khi không có dòng nào, dùng **profile dựng sẵn `orca-default`** trong mã Go (mục 2.5); `SaveQualityProfile` mới tạo dòng.

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK, do ứng dụng sinh |
| `tenant_id` | uuid | NOT NULL |
| `scope_key` | varchar(160) | NOT NULL: `tenant` hoặc `repo:<repo_id>` (cùng lý do `repo_bindings.scope_key`, CR-CV-011: MySQL không có chỉ mục duy nhất từng phần) |
| `repo_id` | uuid | NULL khi `scope_key='tenant'` |
| `name` | varchar(64) | NOT NULL, mặc định `default`; chỉ ký tự `[a-z0-9-]` |
| `mode` | varchar(8) | NOT NULL, CHECK IN (`inform`,`block`); **MVP chỉ chấp nhận `inform`** (mục 2.2) |
| `definition` | jsonb/json | NOT NULL, ≤ 64 KiB, `json.Valid` + UTF-8 (như CR-CV-011 2.3) |
| `updated_by` | uuid | NOT NULL |
| `updated_at` | timestamptz | NOT NULL, đồng hồ DB |
| `version` | bigint | NOT NULL DEFAULT 1 (CAS) |

Duy nhất `(tenant_id, scope_key, name)`. Không FK. Postgres: `ENABLE`+`FORCE ROW LEVEL SECURITY` theo mẫu CR-CV-011; MySQL lọc `tenant_id` ở mọi `WHERE`. Quy tắc chọn profile hiệu lực: `repo:<id>` ∣ `tenant` ∣ dựng sẵn — **không trộn từng trường** giữa các cấp (dễ hiểu, dễ giải thích lý do); UI hiển thị "Profile: repo/tenant/mặc định".

`definition` (schema v1; proto `QualityProfileDefinition`, kiểm ở domain `quality_profile.go`):

```jsonc
{ "schemaVersion": 1,
  "checks": [                       // ≤ 32
    { "id": "lint",       "profile": "lint",       "category": "lint",      "required": true,
      "maxErrors": 0, "maxWarnings": 10 },
    { "id": "typecheck",  "profile": "typecheck",  "category": "typecheck", "required": true, "maxErrors": 0 },
    { "id": "unit",       "profile": "unit",       "category": "test",      "required": true, "maxFailed": 0 }
  ],
  "findings":  { "countScope": "changedFiles", "blockingSeverities": ["error"], "warnBudget": 10 },
  "coverage":  { "required": false, "diffCoverageWarnBelow": 0.6, "diffCoverageFailBelow": null },
  "structure": { "newLayerViolationErrorFails": true, "newLayerViolationWarningWarns": true, "newCyclesWarn": true },
  "freshness": { "indexMustMatchHead": true } }
```

`checks[].profile` là **tên profile chạy** của CR-CV-081 (README v7 mục 3.10, `quality.listProfiles`); `SaveQualityProfile` kiểm tên ∈ danh sách mà dev server của repo đang quảng bá, nếu agent không sẵn sàng thì vẫn lưu nhưng trả `warnings[]` (không chặn lưu vì quảng bá có thể lệch). `category` ∈ enum của `QualityFinding.category`. Giới hạn: `maxErrors/maxWarnings/maxFailed` ∈ [0, 10 000]; `diffCoverage*` ∈ [0,1]; từ chối khoá lạ (decode `DisallowUnknownFields`). Ngưỡng không bao giờ chứa lệnh hay đường dẫn (D5/O11: profile chạy chỉ là tên).

### 2.2 Chế độ `inform` và cờ `quality_gate_enabled`

- `mode=block` lưu được ở schema (để không đổi migration sau) nhưng `SaveQualityProfile` trả `CODEINTEL_INVALID_ARGUMENT` (`reason=block_mode_not_enabled`) cho tới khi có quyết định sản phẩm riêng (O9; câu hỏi mở Q1). Mọi điểm tích hợp (mục 2.8) **chỉ cảnh báo**; `QualityGate.mode` luôn `inform`.
- Cờ phụ `quality_gate_enabled`: thêm cột `quality_gate_enabled boolean NOT NULL` vào `tenant_settings` (migration của CR này; ghi vào "Điều chỉnh hợp đồng"), mặc định tenant mới `false` qua biến `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED`; cờ tổng `CODE_INTEL_QUALITY_GATE_ENABLED` (mặc định `false`). Hiệu lực = `code_intel_enabled ∧ quality_gate_enabled` (cùng cache ≤ 5 giây của CR-CV-073 2.1), fail closed. RPC `GetSettings/SetSettings` (CR-CV-073) mang thêm trường `quality_gate_enabled`; chỉ `role=admin` đặt được; ghi audit `codeintel.settings.set`. Khi tắt: các RPC của mục 2.7 trả `CODEINTEL_QUALITY_GATE_DISABLED` (`FailedPrecondition`, mã mới), dữ liệu `quality_*` giữ nguyên; consumer nội bộ vẫn chạy để không tắc outbox (cùng bảng "hành vi khi tắt giữa chừng" của CR-CV-073).

### 2.3 Thuật toán kết luận

Hàm thuần `EvaluateGate(profile, input, now) QualityGate` trong `internal/domain/quality_gate_evaluator.go`, không I/O (kiểm thử bảng). `input` do use case `EvaluateQualityGate` ghép từ DB:

| Đầu vào | Nguồn |
|---|---|
| `headCommit`, `baseCommit` (merge-base, O7), `changedFiles` + hunks | `ChangeOverlay` (CR-CV-036; `graph_snapshots`/tính lại) |
| `runs` (mỗi `profile` một run phù hợp) | `quality_runs` (CR-CV-082) |
| `findings` của các run đó | `quality_findings` |
| `structureFindings` (`introduced: yes\|touched\|unknown`) | CR-CV-037 `ListFindings` |
| `coverage` | `coverage_reports` (CR-CV-083), nếu `coverage.required` |
| `waivers` hiệu lực, `dismissals` | `quality_waivers` (2.4), `finding_dismissals` |
| `indexFreshness` (`indexCommit`, `stale`) | `repo_bindings.last_status` / CR-CV-080 |

**Chọn run phù hợp cho từng `check`:** cùng `repo_binding`, cùng tên profile chạy, `headCommit` = HEAD hiện tại, trạng thái `succeeded` hoặc `failed` (`queued/running` → kết quả `unknown` kèm lý do `running`; `cancelled` → `unknown`), lấy run có `startedAt` mới nhất; `scope` phải bao phủ yêu cầu (`worktree` ⊇ `changed`; `commitRange` phải cùng `base`). Run có `headCommit` khác HEAD không bao giờ được dùng (không "tái dùng cho gần đúng").

**Kết quả từng kiểm tra** (`result` ∈ `pass|warn|fail|unknown`; thêm giá trị `unknown` vào `reasons[].result`):

| Tình huống | `result` | Ghi chú |
|---|---|---|
| Kiểm tra `required` chưa có run phù hợp | `unknown` | lý do `no_run`; kèm hành động "Chạy kiểm tra" |
| Run `failed` do môi trường/công cụ (không có kết quả parse; ví dụ `CODEINTEL_ENV_NOT_READY`) | `unknown` | không coi là `fail` vì không phải lỗi mã; **cần CR-CV-082 cung cấp `errorCode` hoặc phân biệt `failed`** (mục 6, 7) |
| Số `error` (sau miễn trừ, trong `countScope`) > `maxErrors` | `fail` | `observed="3"`, `threshold="0"` |
| Số `warning` > `warnBudget`/`maxWarnings` | `warn` | |
| Test: `failed` > `maxFailed` | `fail` | test thất bại là mức dự án, không lọc theo tệp đã đổi |
| Typecheck: `error` > ngưỡng | `fail` | mức dự án (lỗi kiểu ở tệp không đổi vẫn do thay đổi gây ra) |
| Coverage bật, diff coverage < `diffCoverageFailBelow` | `fail`; `< WarnBelow` → `warn` | dữ liệu coverage thiếu → `unknown` (lý do `no_coverage`); không bật → **không đánh giá, không xuất lý do** |
| Phát hiện cấu trúc `introduced=yes` mức `error` | `fail`; mức `warning` → `warn` | `introduced=unknown` → `unknown` cho check `structure` (không đoán) |
| `indexMustMatchHead` và `indexCommit ≠ HEAD` | check `structure` (và test-gap) → `unknown`, `basedOn.stale=true` | lint/test/typecheck không phụ thuộc index nên giữ nguyên |

**Gộp thành `verdict`** — thứ tự ưu tiên `fail > unknown > warn > pass`:
1. Có ≥ 1 `reasons[].result = fail` → `fail`.
2. Else có ≥ 1 `unknown` ở kiểm tra bắt buộc → `unknown`.
3. Else có ≥ 1 `warn` → `warn`.
4. Else → `pass`. `pass` chỉ khi **mọi** kiểm tra `required` có dữ liệu hợp lệ.

`unknown` đứng trên `warn` có chủ ý: "chưa đủ dữ liệu" không được che bởi "chỉ có cảnh báo nhỏ"; và `fail` vẫn hiện kèm các lý do `unknown` để người đọc biết bức tranh chưa đầy đủ. Không bao giờ sinh một điểm số số học đơn (research 11 §7: "điểm số gây hiểu lầm").

**Lý do hiển thị được:** mỗi phần tử `reasons[]` giữ đúng tên README (`check`, `observed`, `threshold`, `result`) **cộng** (trường tuỳ chọn, bổ sung): `code` (enum ổn định: `errors_over_limit`, `no_run`, `run_running`, `env_not_ready`, `stale_index`, `no_coverage`, `waived`, …), `params` (map chuỗi), `runId?`, `waivedCount`. Văn bản cuối do frontend dựng từ `code`+`params` qua `translate()` (README v7 mục 6; không câu hiển thị cứng ở backend). `observed/threshold` là chuỗi đã định dạng cho log và báo cáo (CR-CV-090).

`basedOn`: `runIds[]` (đã dùng), `indexCommit`, `stale`; đề xuất thêm `headCommit`, `baseCommit`, `evaluatedAt`, `profileVersion`. `profile` = `"<name>@<scope>/v<version>"` để biết cấu hình nào đã dùng.

**Độ cũ theo lần chạy:** backend chỉ biết `commit`; nếu người dùng sửa tiếp trên cùng commit sau khi chạy, backend **không** phát hiện được (agent chưa ghi dấu vân tay cây làm việc). Giảm thiểu: UI luôn hiện `finishedAt` của run và nút chạy lại; yêu cầu CR-CV-081/082 cân nhắc `QualityRun.treeFingerprint` (câu hỏi mở Q4). Ghi rõ trong tài liệu hiển thị, không nói "đạt" khi chưa chắc.

### 2.4 Miễn trừ `quality_waivers` và quan hệ với `finding_dismissals`

**Quyết định: tách, không hợp nhất.** `finding_dismissals` (CR-CV-037/059) là *phân loại của người review* với phát hiện cấu trúc: không hết hạn, theo `finding_key`, ẩn khỏi danh sách. `quality_waivers` là *ngoại lệ của cổng*: có người duyệt, lý do bắt buộc, **hết hạn bắt buộc**, theo `fingerprint` của `QualityFinding` hoặc cả một kiểm tra. Gộp sẽ làm "bỏ qua" vô thời hạn lặng lẽ biến một lỗi chặn thành đạt. Quan hệ ràng buộc:

| Hành động | Ẩn khỏi danh sách | Ảnh hưởng cổng |
|---|---|---|
| `DismissFinding` (`disposition=ignored`) với phát hiện cấu trúc mức `warning`/`info` | có | không tính vào `warn` (đã được người xem xét) |
| `DismissFinding` với mức `error` hoặc mức thuộc `blockingSeverities` | có | **không** miễn cổng; phát hiện vẫn tính vào kết luận, lý do ghi `dismissed_not_waived` |
| `WaiveFinding` (WAIVE) | không (hiện nhãn "đã miễn đến <ngày>") | miễn khỏi cổng tới `expires_at`, lý do `waived` + `waivedCount` |
| `DismissFinding(RESTORE)`/`WaiveFinding(REVOKE)` | | tính lại ngay |

Nhờ vậy `DismissFinding` giữ nguyên nghĩa ở CR-CV-037/059 (không đổi hợp đồng của chúng) và chỉ `WaiveFinding` mới có thể đổi kết luận cổng từ `fail`. UI (CR-CV-087) đặt hai nút khác nhau ("Bỏ qua" / "Miễn khỏi cổng") với giải thích một dòng.

Bảng `quality_waivers` (mới):

| Cột | Kiểu | Ràng buộc |
|-----|------|-----------|
| `id` | uuid | PK |
| `tenant_id` | uuid | NOT NULL |
| `repo_id` | uuid | NOT NULL (như `finding_dismissals`: sống sót khi binding bị xoá) |
| `subject_kind` | varchar(24) | NOT NULL, CHECK IN (`finding`,`structure_finding`,`check`) |
| `subject_key` | varchar(255) | NOT NULL: `QualityFinding.fingerprint`, hoặc `finding_key` của CR-CV-037, hoặc `check.id` |
| `scope_key` | varchar(160) | NOT NULL: `repo` hoặc `binding:<repo_binding_id>` (miễn riêng một worktree) |
| `reason` | text | NOT NULL, 1–1000 ký tự (UTF-8 hợp lệ) |
| `created_by`, `created_at` | uuid, timestamptz | NOT NULL |
| `expires_at` | timestamptz | **NOT NULL**; tối đa 30 ngày kể từ `now()` (cấu hình `CODEINTEL_WAIVER_MAX_DAYS`, đề xuất; chưa hiệu chỉnh) |
| `revoked_at`, `revoked_by` | timestamptz, uuid | NULL |
| `active_key` | char(64) | NULL, UNIQUE: sha256(`tenant|repo|kind|key|scope`) khi chưa thu hồi, NULL khi đã thu hồi (cùng mẫu `reindex_jobs.active_key` của CR-CV-011) |
| `version` | bigint | NOT NULL DEFAULT 1 |

Hiệu lực = `revoked_at IS NULL AND expires_at > <đồng hồ DB>` (F6 của CR-CV-010/011: dùng `now()` DB, không dùng giờ máy ứng dụng). Gia hạn = upsert theo `active_key` (cập nhật `expires_at`, `reason`, `created_by`); hết hạn không xoá dòng (lịch sử), dọn sau 365 ngày trong công việc bảo trì. `WaiveFinding` idempotent (at-least-once). Miễn `check` (cả kiểm tra) cần lý do ≥ 20 ký tự và hiển thị nổi bật trên scorecard.

### 2.5 Ngưỡng mặc định cho Orca (`orca-default`)

**Đây là giá trị khởi điểm chưa hiệu chỉnh**: chưa đo tỉ lệ báo nhầm, thời gian chạy hay mức nhiễu trên dữ liệu thật của Orca; CR-CV-095 là cơ chế để điều chỉnh. Hợp lệ chỉ với kiểm tra mà repo **đã có** công cụ (research 11 §3.1); coverage/phức tạp/bảo mật mặc định tắt (O12).

| Kiểm tra (`profile` chạy, tên do CR-CV-081 chốt) | `required` | Quy tắc khởi điểm |
|---|---|---|
| `lint` (oxlint) | có | `error` > 0 → fail; `warning` > 10 trong tệp đã đổi → warn |
| `typecheck` (`tsc --noEmit`) | có | `error` > 0 → fail |
| `unit` (vitest) | có | test thất bại > 0 → fail |
| `go-vet`, `go-lint` | có **nếu** thay đổi chạm `backend-go/**` | `error` > 0 → fail |
| `proto` (`buf lint`/`breaking`) | có nếu chạm `.proto` | `error` > 0 → fail |
| `repo-rules` (CR-CV-084) | có | `error` > 0 → fail; `warning` → warn |
| Cấu trúc (CR-CV-037) | có | `layer.domain-imports-outer` mới → fail; `layer.usecase-imports-adapter` mới → warn; `cycle.import` mới → warn; `hotspot`, `dead` không tính |
| Coverage (CR-CV-083) | **không** | nếu bật: diff coverage < 60% → warn; không có ngưỡng fail |
| Bảo mật/phụ thuộc (CR-CV-091) | không | nếu bật: bí mật mới (`error`) → fail |
| Độ tươi index | — | index ≠ HEAD → kiểm tra cấu trúc `unknown` |

"Có `required` có điều kiện" nghĩa là điều kiện áp bằng `changedFiles` của overlay; thiếu overlay thì kiểm tra đó `unknown`, không bỏ qua.

### 2.6 Lịch sử và xu hướng: `quality_trend_points`

Mỗi lần kết luận được tính ở một thời điểm "đáng ghi" thì lưu một điểm (không lưu mỗi lần UI đọc):

| Cột | Kiểu | Ghi chú |
|-----|------|---------|
| `id`, `tenant_id` | uuid | |
| `repo_binding_id` | uuid | khoá theo binding như `review_states` (id worktree có 3 dạng chuỗi, CR-CV-011) |
| `head_commit`, `base_commit` | varchar(64) | |
| `turn_key` | varchar(128) | NOT NULL DEFAULT `''`; `''` = không thuộc lượt nào. Là chuỗi **mờ** do bên gọi cấp; CR-CV-089 sẽ dùng cùng giá trị làm `agent_turns.client_turn_id` (khớp `turnId = "${paneKey}:${doneAt}"` của CR-CV-060 2.4) |
| `profile_ref` | varchar(160) | `<name>@<scope>/v<version>` |
| `verdict` | varchar(8) | CHECK IN (`pass`,`warn`,`fail`,`unknown`) |
| `counts` | jsonb/json | `{error,warning,info, byCategory:{…}}` sau miễn trừ, ≤ 4 KiB |
| `metrics` | jsonb/json | `{diffCoverage?, newLayerViolations, newCycles, testsFailed, …}`, giá trị thiếu = khoá vắng (không `0`) |
| `run_ids` | jsonb/json | ≤ 32 id |
| `index_commit`, `source` | varchar | `source` ∈ `local\|ci` (CR-CV-086) |
| `created_at` | timestamptz | đồng hồ DB |

Duy nhất `(tenant_id, repo_binding_id, head_commit, turn_key, profile_ref)`; ghi lại cùng khoá là **thay thế** (kết luận mới nhất cho cặp đó). Chỉ mục `(tenant_id, repo_binding_id, created_at)`. Giữ tối đa 200 điểm/binding và 90 ngày (đề xuất), dọn theo lô trong bảo trì như `graph_snapshots`.

Ghi điểm khi: (a) consumer của sự kiện `orca.codeintel.quality.run_finished` (CR-CV-081/082) tính lại cổng cho `(binding, head)`; (b) consumer `orca.codeintel.index.changed` khi `stale` đổi; (c) `GetQualityGate` có tham số `record=true` (UI gọi sau khi agent xong, mang `turn_key`) — chỉ ghi nếu chưa có điểm cho khoá đó. Consumer theo `processed_events (tenant_id, event_id)` (idempotent).

`GetQualityTrend` trả chuỗi điểm theo thời gian để vẽ (CR-CV-087), kèm `delta` giữa hai điểm liền kề (số `error/warning`, diff coverage, vi phạm lớp) để so "lượt này vs lượt trước" cùng nhãn "mốc lượt" của CR-CV-060 (`turn_key` khớp `ReviewTurnMarker.turnId`). Điểm thiếu dữ liệu (`verdict=unknown`) được vẽ như khoảng trống, **không nội suy**.

### 2.7 RPC (`orca.codeintel.v1.QualityGateService`, file `quality_gate.proto` mới) và kênh

Message do CR này sở hữu (README v7 mục 3.6: không khai báo RPC chưa có message). `QualityRun`, `QualityFinding` thuộc CR-CV-082; **`QualityGate` do CR-CV-082 khai báo theo README, CR này định nghĩa hành vi và các trường bổ sung ở 2.3**; nếu 082 chưa khai báo thì CR này khai báo (ghi vào "Điều chỉnh hợp đồng").

| RPC | Request (chính) | Response (chính) | Quyền (2.9) | Kênh WS |
|---|---|---|---|---|
| `GetQualityGate` | `repo_binding_id`, `base_ref?`, `profile_name?`, `turn_key?`, `record?`, `include_waived_detail?` | `gate`, `waivers[≤50]`, `evaluatedAt`, `profileDefinitionDigest` | `quality_read` | `codeIntel.quality.gate` |
| `GetQualityProfile` | `repo_id`, `name?` | `profile` (hiệu lực), `origin: repo\|tenant\|builtin`, `version`, `runnableProfiles[]?` | `quality_read` | `codeIntel.quality.profile.get` |
| `SaveQualityProfile` | `repo_id` hoặc tenant, `profile`, `expected_version` (0 = tạo) | `profile`, `warnings[]` | `quality_profile_write` | `codeIntel.quality.profile.save` |
| `WaiveFinding` | `repo_binding_id`, `subject_kind`, `subject_key`, `action: WAIVE\|REVOKE`, `reason`, `expires_at`, `scope: repo\|binding` | `waiver` | `quality_waive` | `codeIntel.quality.waive` |
| `GetQualityTrend` | `repo_binding_id`, `from?`, `to?`, `limit ≤ 200`, `group_by: commit\|turn` | `points[]`, `truncated`, `totalCount` | `quality_read` | `codeIntel.quality.trend` |

Lỗi mới (tiền tố `CODEINTEL_`): `CODEINTEL_QUALITY_GATE_DISABLED` (`FailedPrecondition`), `CODEINTEL_PROFILE_INVALID` (`InvalidArgument`, kèm `data.field`), `CODEINTEL_WAIVER_EXPIRY_INVALID` (`InvalidArgument`: quá hạn tối đa hoặc ở quá khứ); dùng lại `CODEINTEL_VERSION_CONFLICT`, `CODEINTEL_NOT_FOUND`, `CODEINTEL_NOT_AUTHORIZED`.

Hạn mức: `reasons[] ≤ 64`, lý do vượt thì gộp theo `check`; chính sách chung của CR-CV-013 2.6 (tốc độ, đồng thời) áp dụng nguyên văn; `GetQualityGate` chỉ đọc DB (không gọi agent), nên không qua `AgentCallGate`.

### 2.8 Sự kiện, đẩy lên UI

Subject `orca.codeintel.quality.gate_changed` (README v7 3.10), outbox cùng transaction với ghi `quality_trend_points`; payload tối thiểu, **không** chứa mã nguồn hay thông điệp lỗi: `{tenant_id, repo_binding_id, head_commit, base_commit, profile_ref, previous_verdict|null, verdict, run_ids[], turn_key?, evaluated_at}`. Phát khi `verdict` khác điểm gần nhất của cùng binding (hoặc điểm đầu tiên), khử trùng theo `(binding, head, profile_ref, verdict, hash(run_ids))`. Consumer: gateway đẩy tới UI. README 3.10 chỉ liệt kê push `codeIntel.quality.progress/finished`; đề xuất thêm push `codeIntel.quality.gateChanged` (CR-CV-040 chốt tên, ghi vào "Điều chỉnh hợp đồng"). Không có consumer nào khác ở series này; CR-CV-095 chỉ dùng sự kiện phía client.

### 2.9 Quyền (theo CR-CV-013)

Thêm hành động vào `backend-go/policy/orca-authz/code_intel.rego` và `code_intel_test.rego`:

| `action` | RPC | `owner` | `member` | admin toàn cục |
|---|---|:-:|:-:|:-:|
| `quality_read` | `GetQualityGate`, `GetQualityProfile`, `GetQualityTrend`, đọc danh sách miễn trừ | ✓ | ✓ | ✓ |
| `quality_waive` | `WaiveFinding` | ✓ | ✓ (hết hạn tối đa 7 ngày; không miễn `check` hay phát hiện `error`) | ✓ |
| `quality_profile_write` | `SaveQualityProfile` | ✓ | ✗ | ✓ |

Lý do: ngưỡng là tri thức chung của repo (cùng logic `c4_write`); người review là `member` cần xử lý nhanh ngoại lệ nhỏ nhưng có trần thời hạn và phạm vi. Mọi lời gọi qua chuỗi kiểm tra 0–9 của CR-CV-013 2.1 (cờ trước quyền; quyền trước đọc DB). Audit (bảng 2.4 của CR-CV-013 thêm): `codeintel.quality.profile.save`, `codeintel.quality.waive`, `codeintel.quality.waive.revoke` (`target`: `profile:<repo>:<name>`, `waiver:<repo>:<kind>:<key cắt 120>`), `outcome` allowed/denied; `reason` ghi vào log `slog` có `audit=true`, không vào `audit_log` (giống Q3 của CR-CV-013). Từ chối OPA ghi `codeintel.access`.

### 2.9b Cảnh báo trước commit / Create PR (C2, chỉ cảnh báo)

File mới trong `frontend/src/renderer/src/components/right-sidebar/`:

| File (mới) | Nội dung |
|---|---|
| `use-source-control-quality-gate.ts` | Hook `useSourceControlQualityGate({ worktreeId, headOid, baseRef })` → `{ visible, verdict, reasons, loading, stale, runChecks() }`. `visible` chỉ khi `useCodeIntelSupport().state === 'enabled'` (CR-CV-050) **và** cờ `quality_gate_enabled` (từ `codeIntel.settings.get`); không `visible` thì **không gọi RPC nào**. Làm mới khi `headOid` đổi, sau khi nhận push `codeIntel.quality.gateChanged`/`finished`; debounce, bỏ qua khi tab ẩn |
| `source-control-quality-gate-notice.tsx` | Khối nhỏ (token từ `main.css`, `lucide-react`, mọi chuỗi qua `translate()`): `fail` → "Cổng chất lượng: không đạt (N lý do). Bạn vẫn có thể {commit/tạo {PR\|MR}}."; `unknown` → "Chưa đủ dữ liệu để kết luận" + nút "Chạy kiểm tra"; `warn` → một dòng; `pass` → **không** hiện khối (tránh "tốt giả"; thông tin đủ ở scorecard). Liên kết "Xem lý do" mở tab Review qua `openReviewFromEntryPoint(worktreeId, 'source-control', { lens: 'quality' })` (CR-CV-061/087). Tên Create review lấy từ `localizedHostedReviewCopy(resolveSupportedHostedReviewCopyProvider(provider))` nên đúng cho GitHub, GitLab và provider khác |

Điểm chèn (đã đọc): (1) `CreateHostedReviewComposer` thêm prop tuỳ chọn `qualityNotice?: React.ReactNode`, render ngay trên hàng nút chính, **không** đưa vào biểu thức `createDisabled` và không đổi `primaryAction.disabled`; (2) `CommitArea` (`source-control-commit-area.tsx`) thêm prop tương tự; (3) `SourceControl.tsx` chỉ gọi hook và truyền node xuống ở khoảng dòng 5247 (file được ghi nhận là rất lớn, ~6 700 dòng, CR-CV-061 mục 1.2 — thêm vài dòng, logic nằm file mới); (4) `handleCommit` (1792), `handleCreatePullRequest` (3008) và `runCreatePrIntent` (3483) **không đổi hành vi**. Với chuỗi tự động "commit → push → tạo PR" (`runCreatePrIntent`) không có hộp thoại xác nhận trong MVP; thêm xác nhận là câu hỏi mở Q2. Hệ quả cho SSH/remote: cổng chỉ đọc kết quả đã lưu ở backend nên độ trễ dev server không chặn UI; hook có timeout 3 s rồi hiện `unknown` ("không lấy được cổng"), không bao giờ chặn nút.

### 2.10 Cấu trúc file (tất cả mới; tên theo khái niệm)

Backend `backend-go/services/code-intel-service/internal/`: `domain/quality_profile.go`, `quality_gate_evaluator.go`, `quality_waiver.go`, `quality_trend_point.go`, `quality_gate_errors.go`; `usecase/evaluate_quality_gate.go`, `save_quality_profile.go`, `waive_quality_finding.go`, `record_quality_trend_point.go`, `quality_trend_maintenance.go`; `adapter/postgres/` và `adapter/mysql/` `quality_profile_repository.go`, `quality_waiver_repository.go`, `quality_trend_repository.go`; `adapter/grpc/quality_gate_server.go`; migration `…_quality_gate.{up,down}.sql` cho cả hai dialect (số thứ tự theo thứ tự merge; `quality_runs`, `quality_findings`, `coverage_reports` do migration của CR-CV-082/083). Gateway: `channels_codeintel_quality.go` (theo mẫu `channels_*.go`, CR-CV-040).

## 3. Quyết định thiết kế

1. **`unknown` tách khỏi `warn` và ưu tiên cao hơn** (`fail > unknown > warn > pass`): thiếu dữ liệu không được trộn với "gần đạt"; đúng ràng buộc README ("không bao giờ suy diễn thành `pass`").
2. **Hàm đánh giá thuần**, tính khi đọc, không cache kết luận: đầu vào nhỏ, rẻ, và tránh kết luận lưu cũ; chỉ *điểm xu hướng* được lưu.
3. **Tách waiver khỏi dismissal** (2.4): bảo toàn nghĩa của CR-CV-037/059 và buộc hết hạn cho ngoại lệ của cổng.
4. **Không trộn profile giữa các cấp**: lý do hiển thị dễ giải thích; tránh ngưỡng "ma" lai từ hai nơi.
5. **Chỉ đếm phát hiện trong tệp đã đổi, trừ kiểm tra mức dự án** (test, typecheck): giảm báo nhầm do nợ cũ (research 11 §7) mà không bỏ sót lỗi do thay đổi gây ra ở nơi khác.
6. **Cổng không chạy kiểm tra**: chỉ đọc kết quả có sẵn; chạy là việc của CR-CV-081 (bắt đầu bằng hành động người dùng, O11).
7. **Chữ hiển thị sinh ở frontend từ `code`+`params`**: i18n và không overclaim (STYLEGUIDE).
8. **`pass` không hiện cảnh báo ở Source Control**: tránh cảm giác "được phê duyệt"; chỉ báo khi có điều cần chú ý.

## 4. Tiêu chí chấp nhận

- [ ] Hàm `EvaluateGate` có bảng kiểm thử phủ: thiếu run → `unknown`; run sai HEAD không bao giờ dùng; run `running` → `unknown`; có `fail` và có `unknown` → `fail` kèm cả hai lý do; chỉ `warn` → `warn`; đủ dữ liệu, không vượt ngưỡng → `pass`; không có test nào cho ra `pass` khi một kiểm tra `required` thiếu dữ liệu.
- [ ] `index.commit ≠ HEAD` làm kiểm tra cấu trúc thành `unknown` và `basedOn.stale=true`; lint/typecheck/test không đổi.
- [ ] Coverage không bật → không có lý do coverage trong `reasons[]`; bật mà thiếu báo cáo → `unknown`/`no_coverage`.
- [ ] `SaveQualityProfile` từ chối `mode=block`, khoá lạ, ngưỡng ngoài khoảng, `expected_version` lệch (`CODEINTEL_VERSION_CONFLICT`); lưu thành công tăng `version`; hai dialect cho cùng kết quả.
- [ ] `member` gọi `SaveQualityProfile` nhận `CODEINTEL_NOT_AUTHORIZED`; `member` miễn `error` hoặc `check`, hoặc hết hạn > 7 ngày bị từ chối; `owner` miễn tối đa 30 ngày; `expires_at` ở quá khứ bị từ chối.
- [ ] Waiver hết hạn tự mất hiệu lực không cần tác vụ nền (so với đồng hồ DB); `REVOKE` và gia hạn tính lại cổng tức thì; `WaiveFinding` gọi hai lần cùng đầu vào cho đúng một dòng hiệu lực.
- [ ] `DismissFinding` một phát hiện `error` **không** làm `fail` thành `pass`; `WaiveFinding` thì có, và lý do `waived`/`waivedCount` hiện trong `reasons[]`.
- [ ] Mỗi lần `quality.run_finished` đến hai lần với cùng `event_id` tạo đúng một `quality_trend_points`; `gate_changed` phát đúng khi `verdict` đổi và không phát khi không đổi.
- [ ] `GetQualityTrend` trả `limit ≤ 200`, `truncated/totalCount` đúng; điểm `unknown` không được nội suy; `turn_key` khớp `turnId` của CR-CV-060.
- [ ] Cờ `quality_gate_enabled` tắt: mọi RPC mục 2.7 trả `CODEINTEL_QUALITY_GATE_DISABLED`, frontend không gọi RPC và không hiện khối cảnh báo; bật lại thấy dữ liệu cũ.
- [ ] Ở Source Control, verdict `fail` hiện thông báo nhưng nút commit/Create PR vẫn dùng được và hành vi `handleCommit`/`handleCreatePullRequest`/`runCreatePrIntent` không đổi (test hiện có xanh); provider GitLab hiện chữ "merge request".
- [ ] Mọi bảng mới có `tenant_id`, mọi truy vấn lọc tenant (test AST của CR-CV-011 phủ các repository mới); RPC không nhận đường dẫn/ lệnh từ client.
- [ ] `go test` hai dialect (`-tags=integration`), `opa test policy/orca-authz/`, `buf lint` xanh; không file nào > ngưỡng max-lines và không có `max-lines` disable.

## 5. Kiểm thử

- **Unit (domain)**: bảng ca cho `EvaluateGate` (mục 2.3), kiểm schema profile, tính hiệu lực waiver với đồng hồ giả, chọn run phù hợp, cắt `reasons[]`.
- **Repository (hai dialect, integration)**: CAS `version`, upsert waiver theo `active_key`, duy nhất `quality_trend_points`, dọn theo lô, UTF-8/`\u0000` (CR-CV-011 2.3).
- **Use case**: consumer idempotent; `gate_changed` khử trùng; cờ tắt; quyền (Rego + use case).
- **Hợp đồng**: fixture JSON của `QualityGate` (có và không có trường bổ sung) để frontend kiểm tương thích; chạy cùng bộ fixture của CR-CV-070.
- **Frontend (vitest + RTL)**: hook không gọi RPC khi cờ tắt; khối cảnh báo cho mỗi verdict; không đổi `createDisabled`; provider GitLab/GitHub; timeout 3 s.
- **Đo thật (trước khi chốt ngưỡng)**: chạy profile `orca-default` trên 10–20 worktree thật của Orca, ghi tỉ lệ `fail/warn/unknown` và nguyên nhân; chỉ khi đó mới coi giá trị mục 2.5 là "đã hiệu chỉnh".

## 6. Rủi ro và điểm chưa kiểm chứng

- Ngưỡng mục 2.5 là phỏng đoán; nguy cơ nhiễu làm mất niềm tin (research 11 §7). Giảm thiểu: chế độ `inform`, ghi `dismissed_not_waived`, đo ở CR-CV-095.
- **`QualityRun.status = failed` mơ hồ**: README không phân biệt "tìm thấy lỗi" với "công cụ hỏng". Thuật toán giả định có `errorCode`/cờ phân biệt từ CR-CV-082; nếu không, mọi run `failed` phải xử lý theo số `error` đã parse và coi không có kết quả parse là `unknown`.
- Chưa phát hiện được sửa tiếp trên cùng commit sau khi chạy (2.3); kết luận có thể cũ cho tới khi chạy lại.
- Phụ thuộc `ChangeOverlay` có `hunks`/danh sách tệp ổn định (CR-CV-036 còn là đề xuất); thiếu overlay thì kiểm tra theo tệp thành `unknown`.
- `finding_key` đổi khi đổi tên tệp/symbol (CR-CV-037 mục 6) → miễn trừ/bỏ qua có thể "mất" sau refactor; waiver theo `fingerprint` của `QualityFinding` ổn định hơn nhưng phụ thuộc parser của CR-CV-082.
- `repo_id` (CR-CV-011) và `repo_binding_id` (CR-CV-037 2.5, CR-CV-059) đang khác nhau về khoá của `finding_dismissals`; CR này dùng `repo_id` theo CR-CV-011 (nguồn quyết định), cần CR-CV-037 sửa.
- Chưa chạy được thật bất kỳ phần nào; mọi số liệu về nhiễu/thời gian là dự đoán.
- Cạnh tranh tên mã lỗi cờ: CR-CV-013 dùng `CODEINTEL_FEATURE_DISABLED`, CR-CV-073 dùng `CODEINTEL_DISABLED`; CR này thêm mã riêng cho cờ phụ và không quyết thay.

## 7. Câu hỏi mở

- **Q1.** Khi nào (và theo tiêu chí nào) nâng `inform` lên `block`? Đề xuất tiêu chí số ở CR-CV-095 mục 2.5; cần quyết định sản phẩm và chốt phạm vi chặn (commit, push, Create PR).
- **Q2.** Có thêm hộp thoại xác nhận một lần khi chuỗi "commit → push → tạo PR" tự động chạy với cổng `fail`? MVP: không.
- **Q3.** `member` có được miễn phát hiện `error` không (mặc định: không)? Ai duyệt miễn trừ `check`?
- **Q4.** CR-CV-081/082 có thể thêm `QualityRun.errorCode` và `treeFingerprint` không? Gate cần cả hai để tách "lỗi môi trường" khỏi "lỗi mã" và để phát hiện sửa sau khi chạy.
- **Q5.** `finding_dismissals` cần `disposition`; CR-CV-011 chưa có cột (CR-CV-059 mục 6). Chốt trước khi triển khai bảng 2.4.
- **Q6.** Có cần profile theo nhánh/đường dẫn con (monorepo `backend-go/` vs `frontend/`) ngay không? Hiện chỉ điều kiện `required` theo tệp đã đổi.
- **Q7.** Điểm xu hướng cho kết quả CI (`source=ci`, CR-CV-086) có chung dòng với `local` hay tách? Hiện khoá không có `source`; hai nguồn cùng commit sẽ ghi đè nhau.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 2 O9, O13, O14; mục 3.5, 3.10; mục 8)
- `/opt/repos/orca/docs/research/view-code/11-additions-for-quality-control.md` (C1, C2, C4; §7)
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-011-code-intel-data-model-and-repositories.md` (2.1 `tenant_settings`, `finding_dismissals`, `reindex_jobs.active_key`; 2.3; 2.5)
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-013-authorization-audit-and-quotas.md` (2.1, 2.2, 2.4)
- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-036-change-overlay.md` (2.3), `CR-CV-037-structure-analysis.md` (2.1, 2.5)
- `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-059-contract-lens-and-findings.md`, `CR-CV-060-review-notes-send-to-agent-and-turn-compare.md` (2.4), `CR-CV-061-review-entry-points.md` (2.4)
- `/opt/repos/orca/docs/crs/v7/quality-rollout/CR-CV-073-e2e-feature-flag-rollout-runbook.md` (2.1)
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` (`handleCommit` :1792, `handleCreatePullRequest` :3008, `runCreatePrIntent` :3483, `CreateHostedReviewComposer` :5247)
- `/opt/repos/orca/frontend/src/renderer/src/components/right-sidebar/CreateHostedReviewComposer.tsx`, `source-control-create-review-blocked-action.ts`, `source-control-commit-area.tsx`
- `/opt/repos/orca/frontend/src/renderer/src/components/code-review/pr-create-dialog.tsx` (code chết, không dùng)
- CR cùng nhóm (chỉ tham chiếu ID): CR-CV-080, 081, 082, 083, 084, 086, 087, 088, 091, 094
- `/opt/repos/orca/AGENTS.md`, `/opt/repos/orca/guides/STYLEGUIDE.md`
