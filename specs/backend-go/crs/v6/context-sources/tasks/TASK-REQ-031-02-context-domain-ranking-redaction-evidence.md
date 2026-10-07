# TASK-REQ-031-02: Domain thuần (nguồn, hợp đồng adapter, xếp hạng, cắt, Evidence) và bộ che PII

**From Solution:** BE-REQ-SOL-031 (mục C)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/{context_source.go,source_catalog.go,source_adapter.go,context_pack.go,evidence.go}` (mới) và `_test.go`; `.../internal/adapter/redaction/pii_patterns.go` (mới); `.../internal/usecase/ports.go` (thêm `Redactor`, `SourceAdapterRegistry`)
**Depends on:** BE-REQ-SOL-002 (domain `Request`, `Stage` có thể đã có ở CR-003; nếu chưa, task này định nghĩa `Stage`); TASK-REQ-035-01 (`common/secretscan`) cho bản thật của `Redactor`
**Status:** `[x] DONE`

---

## Context

- Domain phải chỉ dùng stdlib (TDD `arch/03`): không import `pgx`, `grpc`, `slog`. Xếp hạng, cắt, lắp, bọc `<untrusted>` là hàm thuần để test golden.
- `Stage` của Context Pack là `classify|solution|plan|task|execute|risk`; **không** trùng `Request.status` hay bước AI của BE-REQ-SOL-034 (`classify|solution|diagnosis|findings|answer|plan|taskspec|execute`). Có bảng ánh xạ ở `build_context_pack.go` (task 05): `diagnosis, findings, answer` dùng pack `solution`; `taskspec` dùng `task`.
- Ma trận chọn nguồn mặc định (CR mục 2.2) phải mã hoá thành dữ liệu trong `source_catalog.go`, khoá theo `SourceKind`, không rải if/else.
- `mcp-service/internal/domain/secret_redactor.go` có `Redactor` kiểu `Redact(string) (string, bool)`; giữ cùng chữ ký để đổi qua lại.
- GitNexus báo `embeddings: 0`, nên `relevance` chỉ trùng từ khoá chuẩn hoá (CR quyết định 4).

## Việc cần làm

1. `context_source.go`: kiểu `SourceKind string` với đủ 22 hằng của CR (`KindRequestOrigin = "request_origin"`, `KindConventions`, `KindDecisions`, `KindSpecs`, `KindServiceCatalog`, `KindCodeGraph`, `KindGitHistory`, `KindContracts`, `KindSchema`, `KindDependencies`, `KindCIConfig`, `KindCIResults`, `KindCoverage`, `KindObservability`, `KindIncidents`, `KindFeatureFlags`, `KindSecurityScan`, `KindPolicy`, `KindOwnership`, `KindHistory`, `KindDevServerProfile`, `KindExternalKnowledge`); `Transport` (`internal|mcp`), `Trust` (`high|medium|low`), `SourceStatus` (`draft|active|disabled`); struct `ContextSource{ID, TenantID, Key, Kind, Transport, Adapter, ServerRef string; Scopes []string; Trust; TTLSeconds, MaxBytes, RateLimitPerMinute int; Redaction RedactionConfig; EnabledFor EnabledFor; Status; OwnerID, CreatedBy string; Version int64; CreatedAt, UpdatedAt time.Time}`.
2. `EnabledFor{Projects, RequestTypes, Stages []string}` và `func (e EnabledFor) Match(projectID string, t RequestType, st Stage) bool` (`"*"` khớp mọi giá trị; danh sách rỗng khớp không gì).
3. `func (s ContextSource) Validate() error`: `Key` khớp `^[a-z][a-z0-9_]{1,63}$`; `Kind` hợp lệ; `Transport=internal` cần `Adapter` thuộc `DefaultCatalog()`, `ServerRef` rỗng; `mcp` cần `ServerRef` là UUID, `Adapter` rỗng, `Scopes` không rỗng, `Kind` thuộc nhóm có thể đến từ MCP (`ci_results`, `coverage`, `observability`, `incidents`, `feature_flags`, `security_scan`, `external_knowledge`, `decisions`, `specs`); `MaxBytes` trong `[1024, 1048576]`; `OwnerID` bắt buộc; `Trust=high` cho nguồn `mcp` bị từ chối (tối đa `medium`, nghiên cứu mục 3). Lỗi trả `ErrSourceInvalid{Field, Reason}` ánh xạ `REQUEST_SOURCE_INVALID` ở tầng gRPC.
4. `source_catalog.go`: `func DefaultCatalog() []ContextSource` trả 14 nguồn đợt 1 (bảng 2.3 của CR: `repo_conventions`, `repo_decisions`, `repo_specs`, `repo_contracts`, `repo_schema`, `repo_dependencies`, `ci_config`, `policy_opa`, `code_graph`, `git_history`, `request_origin`, `history`, `ownership`, `dev_server_profile`) với `Trust`, `TTLSeconds` (đề xuất: 3600 cho repo, 300 cho `request_origin`, 600 cho `code_graph`), `MaxBytes=65536`, `Status=active`, `EnabledFor` theo ma trận; `var DefaultStageMatrix map[SourceGroup]map[Stage]Requirement` (`Required|Recommended|None`) cho nhóm A đến J. Hàm `Merge(defaults, overrides []ContextSource) []ContextSource` (ghi đè theo `Key`, nguồn `mcp` chỉ đến từ overrides).
5. `source_adapter.go`: `SourceAdapter`, `SourceQuery{Text string; Kinds []SourceKind; Limit int}`, `SourceFilter`, `SourceRef{SourceID, Ref, Title string}`, `SourceItem` đúng CR 2.2; `func (i SourceItem) Validate() error` (thiếu `SourceID`, `Ref`, `RetrievedAt` zero, `Freshness` ngoài `fresh|stale|unknown`, `Trust` hợp lệ, `Digest` không là 64 hex, `Size<0` đều lỗi); `ErrSubscribeUnsupported`.
6. `context_pack.go`: `Stage` (nếu chưa có), `BuildInput`, `ContextPack{ID, TenantID, RequestID, Stage, CPVersion, InputDigest, Digest string; BudgetTokens, UsedTokens int; Items []PackItem; Missing []MissingEntry; Body string; CreatedAt time.Time}`, `PackItem{EvidenceID string; Rank, Tokens, Redactions int; Truncated bool}`, `MissingEntry`, hằng `CPVersion = "cp/1"`; hàm thuần `NormalizeTerms(s string) []string`, `Relevance(terms []string, it SourceItem) float64`, `Score`, `Rank(items []SourceItem, terms []string, now time.Time, ttl func(sourceID string) int) []RankedItem`, `EstimateTokens`, `Truncate`, `WrapUntrusted`, `Assemble(ranked []RankedItem, budget int) (body string, kept []PackItem, dropped []MissingEntry)`; `ComputeInputDigest(parts ...string) string` (sha256 hex, ngăn cách bằng `\x1f`); `ComputePackDigest(body string, items []PackItem) string`.
7. `Truncate`: giữ đầu 60% ngân sách, thêm dòng `[... cắt <n> byte ...]`, rồi 20% cuối nếu mảnh là mã (có thể bỏ), kết quả trả `truncated=true`; không bao giờ cắt giữa ký tự UTF-8 (dùng `utf8.RuneStart` lùi về biên rune).
8. `WrapUntrusted` thoát chuỗi `</untrusted>` xuất hiện trong nội dung bằng `<\/untrusted>` để nội dung không đóng khối sớm; thuộc tính `source`, `ref` được thoát ký tự `"` và `<`.
9. `evidence.go`: `Evidence{...}` đúng cột CR 2.5; `EvidenceRef(seq int) string` trả `EVD-<seq>`; `ParseEvidenceRef(s) (int, bool)`; `EvidenceUse{Kind, ID string}` với `Kind` ∈ `solution|plan|task|assessment`; `CheckEvidenceRefs(refs []string, known map[string]bool) []string` trả các ref lạ (đầu ra AI chứa ref ngoài pack thì tầng gọi trả `REQUEST_CONTEXT_UNKNOWN_EVIDENCE`). `Excerpt` cắt cứng 4096 byte ở constructor `NewEvidence`.
10. `redaction/pii_patterns.go`: `type PIIRedactor struct{}` với `Redact(s string) (string, int)` che email (`[REDACTED:email]`), số điện thoại Việt Nam (`(\+84|0)(3|5|7|8|9)\d{8}`) và quốc tế `\+\d{1,3}[ -]?\d{6,12}`; trả số lần che. Ghép trong `adapter/redaction/pipeline.go`: `Pipeline{Secrets Redactor; PII *PIIRedactor}.Apply(profiles []string, s string) (string, int)`; `profiles` có `secrets` thì gọi `secretscan.Redact`, có `pii` thì gọi PII. Port `Redactor` đặt ở `usecase/ports.go` (`Apply`), bản giả `FakeRedactor` ở `_test.go` khi `secretscan` chưa có.

## Kiểm thử

- `source_catalog_test.go`: `DefaultCatalog()` hợp lệ qua `Validate` cả 14 dòng; test ma trận: nguồn nhóm B đánh `Required` cho `solution`, `plan`, `task`, `execute`, `risk` (đối chiếu bảng CR).
- `context_pack_test.go`: golden `testdata/rank_basic.golden`, `testdata/assemble_budget.golden`; `TestRank_TieBreakBySourceIDThenRef`; `TestTruncate_NoSplitUTF8` với tiếng Việt có dấu; `TestAssemble_UsedTokensNeverExceedBudget` (property, 200 mẫu ngẫu nhiên có seed cố định); `TestWrapUntrusted_EscapesClosingTag`; `TestSameInputSameDigest`.
- `source_adapter_test.go`: bảng `SourceItem.Validate` (mỗi trường thiếu một lần).
- `evidence_test.go`: `TestCheckEvidenceRefs_UnknownRef`, `TestNewEvidence_ExcerptCappedAt4KB`.
- `pii_patterns_test.go`: dương (email, `0912345678`, `+84 912345678`, `+1-415-555-0100`), âm (chuỗi số hoá đơn 10 chữ số bắt đầu bằng 1, UUID, hash 40 hex).
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/adapter/redaction/...`.

## Tiêu chí hoàn thành

- [x] Domain không import gói ngoài stdlib (kiểm bằng `go list -deps` trong test hoặc lint kiến trúc hiện có).
- [x] Cùng đầu vào cho cùng `Digest`, thứ tự và điểm cắt xác định.
- [x] `UsedTokens <= BudgetTokens` trong mọi tổ hợp thử.
- [x] Nội dung chứa `</untrusted>` không thoát khỏi khối.
- [x] PII: không dương tính giả trên UUID, hash.

## Ví dụ tham khảo

Ví dụ `EnabledFor` và kết quả `Match` (đưa vào bảng test):

```go
ef := domain.EnabledFor{Projects: []string{"*"}, RequestTypes: []string{"change_request", "bug"}, Stages: []string{"solution", "plan"}}
ef.Match("p1", domain.TypeBug, domain.StageSolution)     // true
ef.Match("p1", domain.TypeDocs, domain.StageSolution)    // false: loại không có trong danh sách
ef.Match("p1", domain.TypeBug, domain.StageClassify)     // false: bước không có trong danh sách
domain.EnabledFor{}.Match("p1", domain.TypeBug, domain.StageSolution) // false: danh sách rỗng khớp không gì
```

Ví dụ điểm xếp hạng (để golden): mảnh `trust=high`, tuổi 0, `relevance=1.0` cho `0.5*1 + 0.3*1 + 0.2*1 = 1.0`; mảnh `trust=low`, tuổi bằng TTL, `relevance=0.5` cho `0.5*0.5 + 0.3*0 + 0.2*0.3 = 0.31`.

Thứ tự làm gợi ý: (1) kiểu và `Validate` (có test ngay); (2) `NormalizeTerms`, `Relevance`, `Score`; (3) `Truncate`, `EstimateTokens`; (4) `Assemble` và `WrapUntrusted`; (5) `Evidence`; (6) bộ che PII. Mỗi bước commit riêng để review dễ.

## Rủi ro và lưu ý

- Từ dừng tiếng Việt: dùng danh sách nhỏ cố định trong mã (không thêm thư viện); chất lượng chưa đo.
- Ước lượng byte/4 phóng đại cho tiếng Việt có dấu (nhiều byte mỗi ký tự); chấp nhận, ghi vào `rủi ro`, hiệu chỉnh sau khi có `usage` thật (BE-REQ-SOL-034).
- Đổi bất kỳ hằng nào trong xếp hạng phải đổi `CPVersion` (`cp/2`) và cập nhật golden.
- `Stage` có thể đã được CR-003 định nghĩa; kiểm `grep -rn "type Stage" internal/domain` trước khi tạo để không trùng.
