# TASK-REQ-027-03: Registry JSON Schema, `ValidateArtifact`, JSON chuẩn tắc, digest và `Provenance`

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `request-service`
**File:** `schemas/v1/{request,solution,diagnosis,findings,answer,plan,phase,task}.schema.json`, `schemas/embed.go`, `internal/domain/artifact_kind.go`, `internal/domain/artifact_schema.go`, `internal/domain/canonical_json_digest.go`, `internal/domain/provenance.go`, `testdata/artifacts/{valid,invalid}/*.json` và test (mới)
**Depends on:** TASK-REQ-001-01 (module `go.mod`), TASK-REQ-002-02 (domain khung)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test ./internal/domain -run 'Schema|Canonical|Provenance|Upgrade|Validate'` và fuzz `FuzzCanonicalJSON` 15s)

---

## Context

CR-REQ-027 mục 2.1, 2.6. Đã đọc `services/api-gateway/go.mod`: `github.com/google/jsonschema-go v0.4.3` là phụ thuộc **trực tiếp** (dùng ở `internal/adapter/mcpserver/tools/spec.go`), nghĩa là CR nói "chưa có thư viện" là lệch; đề xuất dùng lại. Chưa kiểm chứng thư viện hỗ trợ đủ draft 2020-12 (`$defs`, `$ref` nội bộ, `if/then/else`, `unevaluatedProperties`, `format`), nên bước 1 là spike.

BE-REQ-SOL-007 đã định nghĩa `DigestOptions(options, chosen)` dựa trên JSON chuẩn tắc; task này tạo **một** hàm `CanonicalJSON` mà SOL-007 gọi lại (sửa TASK-REQ-007-02 khi hợp nhất, hoặc để 007-02 import hàm này). Domain thuần: schema nhúng bằng `//go:embed` nằm ở package riêng `schemas` (domain không import `embed` trực tiếp từ ngoài cây của nó; đặt `schemas/embed.go` và truyền `fs.FS` vào `NewSchemaRegistry`).

Schema của tám `kind` theo CR 2.5: Request (`acceptance_criteria[]`, `type_fields`), Solution (`options` với `requirement_coverage`, `constraints`, `non_functional`, `test_strategy`, `evidence_refs`, `open_questions` và `assumptions` có cấu trúc), Diagnosis, Findings, Answer (nội dung do SOL-008), Plan v1 (`goal, scope, risks, rollback, verification_strategy, implements, assumptions, satisfies_all`), Phase v1 (`goal, entry_criteria, exit_criteria, depends_on_phases`), Task v1 (`objective, satisfies, acceptance, checks[], exempt_from_coverage`).

## Việc cần làm

1. **Spike (0,5 ngày):** thử `jsonschema-go` với tám schema nháp, chạy bộ `testdata` bên dưới;
   - ghi kết luận (đạt hoặc thư viện thay thế) vào `docs` của PR và vào Q1 của SOL-027.
   - Không thêm họ `xeipuuv` (hiện chỉ là phụ thuộc gián tiếp).
2. `artifact_kind.go`: `type ArtifactKind string` với `request, solution, diagnosis, findings, answer, plan, phase, task`;
   - `LatestSchemaVersion(kind) int` (mọi `kind` hiện là 1)
   - `MaxArtifactBytes(kind) int` (Request 128 KB, Solution 64 KB, Plan 256 KB, Task 32 KB, còn lại 64 KB; số đề xuất chưa đo).
3. `artifact_schema.go`: `type Violation struct{ Path, Code, Message string; Line int }`;
   - `SchemaRegistry` (nạp từ `fs.FS`, biên dịch một lần, an toàn đồng thời)
   - `(r *SchemaRegistry) Validate(kind ArtifactKind, version int, raw []byte) []Violation`: kiểm kích thước (`REQUEST_ARTIFACT_LIMIT_EXCEEDED`), version lớn hơn bản đã biết (`REQUEST_ARTIFACT_SCHEMA_VERSION_UNSUPPORTED`), JSON hợp lệ, rồi schema
   - mọi lỗi schema có `Path` là JSON Pointer và `Code="REQUEST_ARTIFACT_SCHEMA_INVALID"`, tối đa 50 lỗi, không dừng ở lỗi đầu.
4. Nâng phiên bản: `type Upgrader func(raw []byte) ([]byte, error)`;
   - `r.Upgrade(kind, fromVersion, raw)` chạy chuỗi `Upgrade<Kind>V<N>ToV<N+1>` đã đăng ký
   - hiện chỉ có mẫu giả `v0->v1` trong test để chứng minh cơ chế.
   - Người ghi luôn ghi bản mới nhất
   - người đọc nâng khi đọc và không ghi ngược.
5. `canonical_json_digest.go`: `NormalizeNFC(s string) string` (`golang.org/x/text/unicode/norm`), `CanonicalJSON(v any) ([]byte, error)` (khoá theo thứ tự chữ cái, không khoảng trắng, mọi chuỗi qua NFC, số nguyên không có `.0`, không escape HTML), `Digest(v any) (string, error)` trả `sha256:<hex>`;
   - `DigestRaw(raw []byte)` parse rồi chuẩn tắc.
6. `provenance.go`: struct `Provenance{Generator{Kind, Tool, Model, ModelSource}, Prompt{Template, Version, Digest}, RunID string, Attempt int, InputDigest string, InputRefs []InputRef, Actor{ID, Kind}, GeneratedAt time.Time}`;
   - `GeneratorKind` `native|openspec|human|mcp`, `ModelSource` `agent_response|param|unknown`
   - `ComputeInputDigest(in InputDigestParts) (string, error)` với `InputDigestParts{RequestSnapshot any; PriorArtifactDigests []string; PromptVersion string; ProjectContextDigest string}`
   - `(p Provenance) Validate() error`
   - **cấm** trường `credential_ref`, khoá, `env` (test phản chiếu: struct không có trường như vậy, JSON mẫu có chúng bị `REQUEST_ARTIFACT_SCHEMA_INVALID`).
7. Tám tệp schema (draft 2020-12, `additionalProperties:false` ở gốc trừ trường tuỳ chọn đã liệt kê, `schema_version` là `integer` `const` hoặc `minimum 1`);
   - `$id` dạng `https://orca.local/schemas/v1/<kind>.schema.json` (không truy cập mạng, thư viện phải giải `$ref` nội bộ không cần HTTP).
8. `testdata/artifacts/valid/<kind>_*.json` (ít nhất 1 mẫu mỗi kind; Solution có hai mẫu: có và không `requirement_coverage`) và `testdata/artifacts/invalid/<luật>.json` (mỗi luật schema một mẫu: thiếu trường bắt buộc, kiểu sai, enum ngoài tập, `opt-N` trùng, quá dài).
   - Tệp mẫu dùng chung với `task-service` cho `task.schema.json` (copy có kiểm CI, xem 027-02).

## Kiểm thử

- `TestSchemaRegistry_ValidSamples_AllKinds`
- `TestSchemaRegistry_InvalidSamples_ReturnPointerAndCode` (từng mẫu sai, kiểm `Path` chính xác).
- `TestValidate_ReportsAllErrors_NotFirstOnly`
- `TestValidate_CapsAt50Violations`
- `TestValidate_VersionUnsupported`
- `TestValidate_TooLarge`.
- `TestUpgrade_ChainV0ToV1_Pure` (gọi hai lần cho cùng kết quả, không đổi đầu vào).
- `TestCanonicalJSON_KeyOrderAndWhitespace`
- `TestCanonicalJSON_NFC_ComposedAndDecomposedSameDigest` ("Nguyễn" dạng dựng sẵn và tổ hợp)
- `TestCanonicalJSON_NumbersStable`
- `TestCanonicalJSON_GoldenAgainstSOL007DigestOptions`.
- `TestProvenance_RejectsSecretFields`
- `TestComputeInputDigest_ChangesWithPromptVersion`.
- Fuzz `FuzzCanonicalJSON` (không panic; `Canonical(Canonical(x)) == Canonical(x)`).
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'Schema|Canonical|Provenance|Upgrade' && go test ./services/request-service/internal/domain/ -fuzz FuzzCanonicalJSON -fuzztime 20s`.

## Tiêu chí hoàn thành

- [x] Spike có kết luận bằng văn bản; thư viện được chọn biên dịch được cả tám schema.
- [x] Mỗi `kind` có mẫu hợp lệ; mỗi luật schema có mẫu sai với `Path` đúng.
- [x] `CanonicalJSON` idempotent, ổn định theo thứ tự khoá và NFC; SOL-007 `DigestOptions` có thể gọi nó.
- [x] `Provenance` không thể mang `credential_ref`, khoá, `env`.
- [x] Không có truy cập mạng khi biên dịch schema (test chạy với `GOFLAGS=-mod=readonly`, không HTTP).

## Rủi ro và lưu ý

- Nếu thư viện không đủ, chi phí đổi thư viện nằm cả ở task này; giữ `SchemaRegistry` là cổng mỏng để đổi dễ.
- Đo chi phí `Validate` ở kích thước Plan 256 KB chưa làm (chưa kiểm chứng); thêm benchmark `BenchmarkValidatePlan` để người sau có số.
- Giới hạn kích thước là số đề xuất, không phải số đã đo.
- `golang.org/x/text` phải thành phụ thuộc trực tiếp của module `request-service`; không dùng `replace`.

## Ghi chú triển khai (rf/art)

**Kết quả spike (bước 1).** Thử `github.com/google/jsonschema-go v0.4.3` với 8 schema thật: hỗ trợ đúng `$defs`, `$ref` nội bộ, `if/then`, `const`, `enum`, `pattern`, `uniqueItems`, `unevaluatedProperties`; **nhưng** (1) chỉ trả **lỗi đầu tiên** (`Validate` dừng ở lỗi đầu), (2) lỗi mô tả đường dẫn **schema** (`/properties/acs/items/...`), không phải JSON Pointer của dữ liệu, (3) không kiểm `format` (`"nope"` qua `date-time`). Trái yêu cầu "không dừng ở lỗi đầu, `Path` là JSON Pointer". Thử `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` (MIT, Go thuần, có trong module cache, không phải họ `xeipuuv`): trả đủ lỗi với `InstanceLocation`, kiểm `format`, `unevaluatedProperties` đúng. **Chọn santhosh-tekuri v6**; `SchemaRegistry` là cổng mỏng nên đổi thư viện chỉ sửa một file.

- `ValidateArtifact` của task là `SchemaRegistry.Validate(kind, version, raw)` và `ValidateDocument(kind, raw)` (tự đọc `schema_version`); `Violation` dùng chung struct sẵn có ở `plan_proposal.go`, thêm trường `Path` (cộng thêm, không đổi nơi gọi).
- Kiểm trùng id trong mảng đối tượng (`opt-N`, `AC-n`, `Q-n`, `A-n`, `c-n`) làm sau schema vì JSON Schema không diễn đạt được.
- `CanonicalJSON` sao đúng thuật toán của task-service (nhánh `rf/task-b`); mẫu vàng `testdata/artifacts/task/canonical_cases.json` **giống từng byte** với bản của task-service (đã so bằng `git show rf/task-b:...`; `scripts/check-artifact-samples.sh` so hai thư mục sau khi hợp nhất). `DigestOptions` (SOL-007) giờ gọi `CanonicalJSON`, nên từ chối khoá trùng và chuẩn hoá NFC (hành vi chặt hơn bản cũ).
- `ComputeInputDigest` của task trùng tên hàm có sẵn (`context_pack.go`) nên đặt là `ComputeProvenanceInputDigest`. `Provenance.Actor.Kind` dùng bộ từ vựng `ai|user|system` (`RevisionActorKind`) như schema.
- Không truy cập mạng: `TestSchemaRegistry_NoNetworkNeededToCompile` thay `http.DefaultTransport` bằng bộ chặn và biên dịch 8 schema (không dùng cờ `GOFLAGS=-mod=readonly`).
- Benchmark `BenchmarkValidatePlan` chạy được (mẫu plan nhỏ: ~16 µs/lần); chi phí ở Plan 256 KB **chưa đo**.
- Test tên khác task: `TestCanonicalJSON_GoldenAgainstSOL007DigestOptions` thành `TestCanonicalJSON_Golden` + `TestDigestOptions_UsesCanonicalCore`.
