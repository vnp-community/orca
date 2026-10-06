# TASK-REQ-029-05: `TaskSpecV2` (kiểm hợp lệ, digest chuẩn tắc), `ScopeMatcher` và `RenderExecutionPacket`

**From Solution:** [BE-REQ-SOL-029](../solutions/BE-REQ-SOL-029-execution-contract-and-readiness-gate.md) mục 2.B, 2.C
**Priority:** P0
**Service/Area:** `request-service` (mới) / domain thuần, không I/O
**File:** `internal/domain/task_spec_v2.go` (mới), `internal/domain/spec_violation.go` (mới), `internal/domain/scope_matcher.go` (mới), `internal/domain/execution_packet.go` (mới), `internal/domain/execution_packet_template.go` (mới), `internal/domain/testdata/{task_spec_v2_valid.json,packet_ep1.golden.txt}` (mới), và các `_test.go`
**Depends on:** TASK-REQ-001-01 (module); TASK-REQ-027-03 (`CanonicalJSON`, `Digest` ở `internal/domain/canonical_json_digest.go`, mẫu vàng), TASK-REQ-027-02 (`SchemaVersion >= 1` nhận v2, bảng `task_specs`); CR-REQ-027 (schema v1, `AC-n`, `exempt_from_coverage`); không phụ thuộc DB hay proto
**Status:** [ ] TODO

---

## Context

- CR-REQ-029 mục 2.1: `TaskSpec` v2 cộng thêm vào v1 của CR-REQ-027 (`objective`, `satisfies`, `acceptance`, `checks[{id,kind,description,command?,expect?}]`, `exempt_from_coverage`), thêm `scope`, `constraints`, `inputs`, `approach`, `checks[].cwd|timeout_seconds|baseline`, `outputs`, `requires`, `irreversible`, `timeout_minutes`, `stop_conditions`. Bảng luật kiểm và mã `REQUEST_SPEC_*` ở CR mục 2.1; `bug` và `security` phải có Check `kind=test` (khớp `test:regression`, CR-REQ-012).
- Digest: TASK-REQ-027-02 (`task-service`, `domain/task_spec.go`: `NewTaskSpec` tính `Digest` = SHA-256 hex của `CanonicalJSON`, bản sao ngắn ở `task-service`) và TASK-REQ-027-03 (`request-service`, `canonical_json_digest.go`: `CanonicalJSON(v any) ([]byte, error)` có chuẩn hoá NFC, `Digest(v any) (string, error)` trả `sha256:<hex>`). **Không tạo hàm canonical thứ ba ở đây**: gọi hàm của 027-03. Cột `spec_digest CHAR(64)` lưu hex **không** có tiền tố `sha256:` (cắt tiền tố khi ghi và khi so với `task_specs.digest`). Mẫu vàng chung hai service do 027-02/027-03 sở hữu.
- Packet (CR mục 2.3): hàm thuần, không đồng hồ, không môi trường, không AI; cùng đầu vào cho cùng byte; `TemplateVersion="ep/1"`; thứ tự mục cố định; `<untrusted>` cho nội dung từ Request/Jira/GitHub; không credential; `requires.env_names` chỉ in tên; qua `secretscan.Redact` của CR-REQ-035 (`common/secretscan` chưa có trong repo: `ls backend-go/common` ngày 2026-10-06 không có thư mục này). Domain **không** import `common/secretscan` trực tiếp: nhận hàm `redact func(string) (string, bool)`.
- Khớp glob: agent không khớp được `**` (`fs.glob` chỉ khớp tên cuối, dùng `find`), nên cổng (task 06) khớp ở Go; `ScopeMatcher` thuần nằm ở domain.
- Quy ước repo: tên tệp theo khái niệm, không `helpers`/`utils`/`common`/`misc` (ví dụ `scope_matcher.go`, không phải `spec_utils.go`).

## Việc cần làm

1. `spec_violation.go`: `SpecViolation{Code, Path, Message string}`; hằng mã `SpecScopeInvalid = "REQUEST_SPEC_SCOPE_INVALID"` và các mã còn lại ở solution 2.B; `func (v SpecViolation) Error() string`.
2. `task_spec_v2.go`: các kiểu ở solution 2.B, JSON tag `snake_case` đúng CR; `ParseTaskSpecV2(raw []byte) (TaskSpecV2, error)` từ chối `schema_version != 2` (`REQUEST_SPEC_CHECK_INVALID` không dùng; trả lỗi riêng `ErrSpecSchemaVersion`), từ chối khoá lạ ở cấp gốc, và từ chối quá 32 KB trước khi giải (`REQUEST_SPEC_TOO_LARGE`).
3. `Validate(ctx SpecContext) []SpecViolation` (gom hết vi phạm, không dừng ở lỗi đầu):
   - `scope`: `include` không rỗng, mỗi mẫu hợp lệ (`ValidateGlob`), không chứa `..`, không bắt đầu bằng `/` hay chữ cái ổ đĩa `X:`, không có `\`; `max_files` trong `[1,50]` (0 coi như vắng, mặc định 8, chỉ đặt khi ghi chuẩn tắc); `create` không chứa ký tự glob.
   - `checks`: ít nhất một (trừ `ctx.ExemptFromCoverage`):
     - `id` duy nhất `^[a-z][a-z0-9_-]{0,31}$`
     - `kind` ∈ `command|test|lint|typecheck|diff_rule|manual`
     - kind chạy được (`command|test|lint|typecheck`) bắt buộc `command` không rỗng, `timeout_seconds` trong `[1,300]` (mặc định 120)
     - `diff_rule` chỉ nhận `Rule` ∈ {`changed_files_subset_of_scope`, `no_new_files_outside_create`, `no_test_files_modified`}
     - `manual` không chạy được và **không** đủ để thoả "có Check"
     - `command` không chứa ký tự điều khiển, không dài quá 500
     - `expect.match` biên dịch được bằng `regexp` (RE2).
   - `inputs[].from_task` ∈ `ctx.DependsOn` và task đó khai `outputs[].name` trùng (`ctx.DependsOn` là `map[taskID][]outputName`); `inputs[].path` (loại `file`) thoả luật đường dẫn như `scope`.
   - `requires.env_names`: `^[A-Z][A-Z0-9_]{0,63}$`, tối đa 64; `requires.tools` `^[A-Za-z0-9._+-]{1,64}$`.
   - `timeout_minutes` trong `[1,15]` (mặc định 10).
   - `irreversible=true` thì `ctx.Labels` chứa `gate:pre_deploy`.
   - `acceptance` không chứa từ trong `ctx.VagueWords` (so khớp không phân biệt hoa thường, theo từ, có dấu tiếng Việt).
   - `bug` và `security` (`ctx.TaskType`/`ctx.RequestType`) phải có Check `kind=test`.
   - Kích thước: mỗi danh sách ≤ 20 phần tử.
4. Digest spec: `(s TaskSpecV2) CanonicalDigest() (string, error)` gọi `Digest(s)` của TASK-REQ-027-03 rồi `strings.TrimPrefix(d, "sha256:")` (64 ký tự hex). Spec lưu ở `task_specs.spec` là JSON thô do AI/Plan sinh:
   - để digest khớp với `task_specs.digest` của `task-service`, `request-service` băm **JSON thô của `GetTaskSpecs.spec_json` đã chuẩn tắc**, không băm lại từ `TaskSpecV2` đã giải (giải rồi mã hoá lại có thể làm rơi khoá lạ). Vì vậy hàm thật dùng là `DigestOfRawSpec(raw []byte) (string, error)` (giải về `any`, gọi `Digest`, cắt tiền tố)
   - `CanonicalDigest()` chỉ dùng trong test.
5. `scope_matcher.go`: `type ScopeMatcher struct{ include, exclude []compiledGlob }`:
   - `NewScopeMatcher(include, exclude []string) (*ScopeMatcher, error)`
   - `Match(path string) bool` (đường dẫn dùng `/`, chuẩn hoá `\` thành `/` trước; hỗ trợ `**` (nhiều cấp), `*` (không qua `/`), `?`, lớp ký tự `[abc]`; không phân biệt hoa thường **chỉ** khi cờ `caseInsensitive` bật cho host Windows)
   - `Violations(files []string, create []string, maxFiles int) []ScopeFinding` trả `OUT_OF_SCOPE`, `EXCLUDED`, `NEW_FILE_OUTSIDE_CREATE`, `TOO_MANY_FILES`.
6. `execution_packet.go`: `PacketInput`, `ExecutionPacket`, `RenderExecutionPacket(in PacketInput, redact func(string) (string, bool)) ExecutionPacket` như solution 2.C. Dựng bằng `strings.Builder` (chuỗi đưa vào qua `NormalizeNFC` của 027-03 trước khi băm để tiếng Việt dựng sẵn và tổ hợp cho cùng digest) với hằng chuỗi tiêu đề ở `execution_packet_template.go` (một nơi, để đổi chữ là tăng `TemplateVersion`). Cắt theo rune: tóm tắt Request 1200, đầu vào 4096 byte (cắt tại ranh giới rune, thêm `…[cắt]`), `PriorVerdictSummary` 1024. `untrustedBoundary(content string) string` = `"untrusted-" + hex(sha256(content))[:12]`. Mọi `Text` qua `redact` rồi mới tính `Digest = SHA256Hex(Text)`; `InputDigest` = `sha256` hex của `CanonicalJSON(PacketInput bỏ Nonce, Attempt, PriorVerdictSummary)` (hàm của TASK-REQ-027-03).
7. Sinh golden: `testdata/packet_ep1.golden.txt` từ một `PacketInput` mẫu (tiếng Việt có dấu, hai đầu vào, ba Check, một điều kiện dừng); test so byte; cờ `-update` (cờ test tự khai báo) để cập nhật có chủ ý, không tự ghi trong CI.
8. Không tạo tệp vector mới: dùng mẫu vàng canonical của TASK-REQ-027-03 và 027-02 (cùng nội dung ở hai service, CI so sánh). Thêm một ca riêng cho spec v2 (khoá lồng `scope`, `checks[]`, tiếng Việt có dấu) vào **mẫu vàng đó** bằng PR phối hợp, nếu 027 chưa có.

## Kiểm thử

- `TestTaskSpecV2_Validate_Table`: mỗi dòng của bảng CR mục 2.1 một ca âm và một ca dương, kiểm đúng `Code` và `Path`; ca nhiều vi phạm cùng lúc; `exempt_from_coverage` bỏ qua `CHECK_MISSING`; `bug` thiếu Check `test`; `irreversible` thiếu nhãn.
- `TestParseTaskSpecV2_RejectsV1`, `_RejectsUnknownRootKey`, `_RejectsOver32KB`.
- `TestDigestOfRawSpec_MatchesTaskServiceGolden` (mẫu vàng chung của 027), `_KeyOrderIndependent`, `_StableAcross1000Runs`, `_PrefixStripped64Hex`.
- `TestScopeMatcher_Table` (`**`, `*`, `?`, thư mục sâu, đường dẫn Windows có `\`, tiếng Việt có dấu, `exclude` thắng `include`), `TestScopeViolations_*` (file mới ngoài `create`, vượt `max_files`).
- `TestRenderExecutionPacket_Golden`, `_DeterministicAcross1000Calls`, `_ChangeTemplateTextChangesVersionGuard` (test kiểm hằng `TemplateVersionEP1` khớp băm của chuỗi tiêu đề: đổi chữ mà không tăng phiên bản thì test đỏ), `_UntrustedBoundaryDependsOnContent`, `_ContentCannotForgeClosingTag`, `_NoEnvValues` (spec có `env_names`, test quét `Text` tìm giá trị đặt qua `PacketInput` giả), `_RedactsAfterRender`, `_TruncatesInputsAt4KB`, `_PriorVerdictOnlyOnRetry`, `_NonceInContractSection`.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run "TaskSpec|Canonical|Scope|Packet"`.

## Tiêu chí hoàn thành

- [ ] Mỗi dòng bảng 2.1 của CR có test âm và dương; mọi mã `REQUEST_SPEC_*` xuất hiện trong test.
- [ ] `RenderExecutionPacket` cùng đầu vào cho cùng `Digest` qua 1000 lần; đổi một ký tự template làm test guard đỏ.
- [ ] Nội dung độc hại (`</untrusted-...>`, `ORCA_RESULT_BEGIN`) trong Request không phá khối và không tạo khối kết quả hợp lệ.
- [ ] Không có giá trị biến môi trường hay `credential_ref` trong packet (test quét).
- [ ] Domain không import `common`, adapter hay `os`/`time.Now`.
- [ ] `DigestOfRawSpec` trùng `task_specs.digest` của `task-service` trên mẫu vàng chung (không còn bản canonical thứ ba).

## Rủi ro và lưu ý

- `ScopeMatcher` tự cài: sai khác nhỏ với `git pathspec` làm cổng báo nhầm; ghi rõ cú pháp hỗ trợ và coi mẫu lạ (`{a,b}`) là không hợp lệ ở `Validate` (`SCOPE_INVALID`) thay vì khớp sai.
- Lệnh trong Check là chuỗi người dùng/AI viết, sẽ chạy trên dev server (cổng, `VerifyExecution`): `Validate` chỉ kiểm hình thức; việc chạy luôn qua `agent.exec` với `binary`/`args` tách, **không** qua shell nối chuỗi (task 06, 07).
- `Digest` của 027-03 trả `sha256:<hex>` còn `task_specs.digest` ở `task-service` là hex trần (027-02): cắt tiền tố ở một chỗ duy nhất (`DigestOfRawSpec`) và test; nhầm một chỗ là mọi `spec_digest` lệch.
- `secretscan.Redact` có thể đổi độ dài văn bản; vì `Digest` tính sau redact nên digest phụ thuộc phiên bản bộ mẫu (`patterns_version`): ghi `patterns_version` vào `InputDigest` khi có (nếu CR-REQ-035 cung cấp).
- Mô hình "từ mơ hồ" dùng danh sách cấu hình, chưa đo tỉ lệ dương tính giả.
