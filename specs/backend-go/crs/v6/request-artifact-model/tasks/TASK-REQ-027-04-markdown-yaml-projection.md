# TASK-REQ-027-04: Bản chiếu Markdown/YAML: `RenderArtifact`, `ParseProjection` và `ExportArtifactProjection`

**From Solution:** BE-REQ-SOL-027
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/artifact_projection.go`, `internal/domain/projection_frontmatter.go`, `internal/domain/projection_regions.go`, `internal/usecase/export_artifact_projection.go`, `testdata/projection/*.md` và test (mới)
**Depends on:** TASK-REQ-027-03 (`Violation`, `CanonicalJSON`, `ValidateArtifact`)
**Status:** [x] DONE

---

## Context

CR-REQ-027 mục 2.7: bản chiếu là tệp văn bản sinh từ mô hình chuẩn trong DB; **mô hình chuẩn vẫn là nguồn sự thật**. Quy tắc phân tích chặt: phần mở đầu YAML ở dòng đầu, khối ```` ```orca-json ```` trong vùng Orca, các dòng khớp văn phạm của vùng đó; mọi văn bản khác là tự do, không suy ra dữ liệu. BE-REQ-SOL-026 (OpenSpec) dùng cùng hàm `RenderArtifact` để viết lại vùng `options` và (qua `tasks_md_render.go`) vùng `plan`; vì vậy API ở đây phải ổn định và không phụ thuộc OpenSpec.

Đã kiểm: `gopkg.in/yaml.v3 v3.0.1` có ở `api-gateway/go.mod` (trực tiếp); `request-service` phải thêm vào module của nó. `yaml.v3` hỗ trợ `Decoder.KnownFields(true)`; không có tuỳ chọn tắt anchor/alias, nên bước 2 chặn bằng quét văn bản trước khi giải.

Đầu vào bản chiếu có thể đến từ tệp trong repo của dự án (không tin cậy, CR mục 2.7 điểm 7): kích thước tối đa, loại ký tự điều khiển, không bao giờ thành lệnh hay đường dẫn.

## Việc cần làm

1. `projection_frontmatter.go`: `Frontmatter{OrcaSchema int; Kind ArtifactKind; ID, Request, Status, Supersedes, Digest string; GeneratedBy GeneratedBy}` với yaml tag là `orca_schema`, `kind`, `id`, `request`, `status`, `supersedes`, `digest`, `generated_by`.
   - `ParseFrontmatter(md string) (Frontmatter, rest string, []Violation)`: dòng đầu phải là `---`, kết thúc bằng `---` trong 64 KB đầu (`PROJECTION_FRONTMATTER_MISSING`, `..._TOO_LARGE`)
   - từ chối `&anchor`, `*alias`, `!!tag`, `<<:` bằng quét dòng trước khi giải (`PROJECTION_YAML_UNSAFE`)
   - `KnownFields(true)`
   - thiếu `orca_schema`, `kind`, `id` thì `PROJECTION_FRONTMATTER_FIELD_REQUIRED` kèm số dòng.
2. `projection_regions.go`: `ExtractRegion(md, section string) (body string, startLine, endLine int, ok bool)` tìm cặp `<!-- orca:begin <section> ... -->` và `<!-- orca:end <section> -->`, không lồng;
   - `ExtractOrcaJSON(body string) ([]byte, []Violation)`: đúng một khối ```` ```orca-json ```` trong vùng (nhiều hơn hoặc không có thì vi phạm), nội dung phải là JSON hợp lệ; `ReplaceRegion(md, section, newBody string) (string, error)` thay chính xác giữa cặp chú thích, **giữ nguyên mọi byte ngoài vùng**; vùng không tồn tại thì chèn ở cuối đề mục tương ứng hoặc lỗi `PROJECTION_REGION_MISSING` (chọn lỗi, không đoán).
3. `artifact_projection.go`: `RenderArtifact(kind ArtifactKind, doc []byte, meta ProjectionMeta) ([]byte, error)`: frontmatter + tiêu đề + đề mục cố định theo `kind` (Solution: `Summary`, `Options`, `Requirement coverage`, `Assumptions`, `Open questions`; Plan: `Goal`, `Phases`, `Tasks`, `Risks`, `Rollback`) + vùng Orca có `digest=`;
   - xác định (thứ tự khoá cố định, `\n`, NFC).
   - `ParseProjection(md []byte, kind ArtifactKind) (doc []byte, []Violation)`: lấy dữ liệu **chỉ** từ frontmatter, khối `orca-json` trong vùng, và dòng khớp văn phạm
   - chạy `ValidateArtifact` trên kết quả
   - `Violation.Line` là số dòng tuyệt đối.
4. Ràng buộc đầu vào: tối đa 1 MB mỗi tệp (`PROJECTION_TOO_LARGE`), loại ký tự điều khiển trừ `\n`, `\t` (từ chối, không tự xoá), chuẩn hoá `\r\n`.
5. `ExportArtifactProjection.Execute(ctx, requestID, artifactID string, format string) (ExportResult, error)`: `RequireTenantID`;
   - `format=="markdown"` (khác thì `REQUEST_ARTIFACT_FORMAT_UNSUPPORTED`)
   - Solution lấy từ `SolutionRepository`, Plan lấy qua port `PlanReader` (nhận cây từ task-service `GetSubtree` + `GetTaskSpecs`; nếu port chưa có thì trả `Unimplemented` cho Plan và ghi chú, làm ở 027-07)
   - trả `{filename, content, digest}`.
   - Chỉ đọc, không ghi DB.
6. Không có hàm "nhập bản chiếu vào DB" ở task này: CR quyết định mô hình chuẩn là nguồn;
   - `ParseProjection` phục vụ kiểm tra đầu ra công cụ ngoài (026) và test vòng đi về.
7. Mẫu `testdata/projection/`: `solution_ok.md`, `plan_ok.md`, `frontmatter_alias.md` (có `&a` và `*a`), `two_orca_json_blocks.md`, `prose_fake_json.md` (JSON giả ngoài vùng), `crlf.md`, `vietnamese_decomposed.md`.

## Kiểm thử

- `TestRender_Parse_RoundTrip_AllSamples` (`Parse(Render(x)) == x` về nội dung chuẩn tắc, so bằng `CanonicalJSON`); `TestRender_Deterministic`.
- `TestParse_IgnoresProseOutsideRegion` (JSON giả và checkbox giả ngoài vùng không đổi kết quả)
- `TestParse_FrontmatterRejectsAliasAnchorTag`
- `TestParse_TwoOrcaJSONBlocks_Violation`
- `TestParse_ViolationLineNumbersAbsolute`.
- `TestReplaceRegion_PreservesBytesOutside` (hash phần ngoài vùng trước và sau bằng nhau)
- `TestReplaceRegion_MissingRegion_Error`.
- `TestParse_RejectsControlCharsAndOversize`
- `TestRender_NFC_Normalises`.
- `TestExportArtifactProjection_Markdown_Solution`
- `TestExportArtifactProjection_UnknownFormat`
- `TestExportArtifactProjection_CrossTenantNotFound`.
- `FuzzParseProjection`: không panic, mọi `Line` trong khoảng hợp lệ, thời gian chạy bị chặn bởi giới hạn kích thước.
- Lệnh: `cd /opt/repos/orca/backend-go && go test ./services/request-service/internal/domain/... -run 'Projection|Frontmatter|Region' && go test ./services/request-service/internal/usecase/... -run ExportArtifactProjection && go test ./services/request-service/internal/domain/ -fuzz FuzzParseProjection -fuzztime 30s`.

## Tiêu chí hoàn thành

- [x] `Parse(Render(x)) == x` cho mọi mẫu; `Render` xác định.
- [x] Văn bản ngoài vùng Orca không bao giờ đổi dữ liệu parse được.
- [x] YAML có anchor, alias, tag bị từ chối với số dòng.
- [x] `ReplaceRegion` giữ nguyên byte ngoài vùng (test băm).
- [x] Đầu vào từ repo bị chặn kích thước và ký tự điều khiển.
- [x] Hàm xuất bản chiếu có test chéo tenant.

## Rủi ro và lưu ý

- Bản chiếu tăng bề mặt tấn công vì đọc lại nội dung do agent viết (CR mục 6); biện pháp là các giới hạn trên, chưa có kiểm thử với đầu ra agent thật.
- Đề mục cố định theo `kind` do CR định nghĩa nhưng chưa có mẫu thật; sẽ chỉnh khi có phản hồi UI (CR-REQ-021).
- `yaml.v3` đã có CVE lịch sử về DoS qua alias; quét chặn trước khi giải là biện pháp chính, không dựa vào thư viện.
- Quy tắc `Plan` ở đây chỉ gồm khung; văn phạm `tasks.md` thuộc BE-REQ-SOL-026 (`tasks_md_parser.go`), không nhân đôi.
