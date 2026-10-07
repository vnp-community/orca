# TASK-REQ-034-06: `GroundingChecker` kiểm đường dẫn và lệnh do AI đề xuất

**From Solution:** BE-REQ-SOL-034 (mục E, CR 2.6)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/domain/grounding_report.go` (mới), `.../internal/usecase/grounding_checker.go` (mới), `.../internal/usecase/grounding_extractors.go` (mới: trích tham chiếu từ từng loại sản phẩm), `.../internal/usecase/ports.go` (sửa), và `_test.go` tương ứng
**Depends on:** TASK-REQ-034-04 (điểm gọi trong `AIGateway.Run`); BE-REQ-SOL-031 task 03 (`RepoReader`, `SafeRelPath`) và task 04 (`DevServerProfileReader`)
**Status:** `[x] DONE`

---

## Context

- Agent (đã đọc ở `agent/src/relay/`): `fs.glob {pattern, cwd?, ignore?}` tối đa 200 kết quả (`fs-agent-extensions.ts:219`); `fs.readFile {path}` nhận **đường dẫn tuyệt đối** và không chống `..` (`:103`); `fs.stat` có ở `agent-rpc-dispatch-fs.ts:53`. Mọi tham chiếu đến từ AI là không tin cậy: phải qua `SafeRelPath` (task 031-03) **trước** khi gọi Relay, nếu không một Solution độc có thể khiến backend hỏi `/etc/passwd`.
- CR 2.6: đường dẫn `new:true` bỏ qua; lệnh không xác định được là `unverified`, không phải `missing`; `ratio` dưới ngưỡng 0,8 thì thử lại đúng một lần với danh sách thiếu; vẫn thấp thì `needs_review=true`, **không tự bỏ phần sai**; tối đa 100 kiểm tra mỗi sản phẩm; không dev server thì `status="skipped"`.
- Cách đọc số dòng bằng `fs.readFile` (tham số khoảng) **chưa kiểm chứng**: v1 dùng `ReadFile` cắt 512 KB rồi đếm `\n` (chỉ cho tối đa 20 tham chiếu `file:line`); phương án thay: `fs.grep` đếm.
- Cấu trúc đầu ra cần trích: `affected_areas[].name` (kind `module|api|schema`), `scope.include`, bằng chứng `file:line` của `diagnosis` (CR-REQ-008), tệp trong task (CR-REQ-012), `checks[].command` và lệnh test trong Plan. Định nghĩa schema thật do SOL-007, 008, 012; task này chỉ định nghĩa **giao diện** `GroundingExtractor` và các hàm trích theo kiểu JSON đã chốt trong những solution đó (đọc lại khi làm, không sao chép từ trí nhớ).

## Việc cần làm

1. `grounding_report.go`: 

```go
type RefKind string // path | path_line | command
type GroundingRef struct{ Kind RefKind; Value string; Line int; New bool }
type GroundingFinding struct{ Ref GroundingRef; Status string; Detail string } // Status: ok | missing | unverified | skipped
type GroundingReport struct {
    Status      string // checked | skipped | partial
    Checked     int
    Missing     []GroundingFinding
    Unverified  []GroundingFinding
    Ratio       float64 // ok / (ok + missing); unverified không tính
    NeedsReview bool
    Retried     bool
}
func ComputeRatio(findings []GroundingFinding) float64
```

   `Ratio` khi không có tham chiếu kiểm được (toàn `unverified`/`new`) là `1.0` (không phạt).
2. `ports.go`: `type GroundingExtractor func(raw []byte) ([]domain.GroundingRef, error)`; cổng `GroundingChecker{Check(ctx, projectID string, refs []domain.GroundingRef) (domain.GroundingReport, error)}`.
3. `grounding_checker.go`: 
   - Giới hạn: lấy tối đa 100 tham chiếu đầu (đã khử trùng, ổn định theo thứ tự xuất hiện); phần còn lại ghi `Detail="over_limit"`, tính `unverified`.
   - **Đường dẫn:** `SafeRelPath`; vi phạm (tuyệt đối, `..`) ⇒ `missing` với `Detail="unsafe_path"` (không gọi Relay). Gộp theo thư mục cha: một `Glob("<dir>/*")` cho mọi tệp cùng thư mục (giảm số lời gọi), tồn tại nếu có trong kết quả; thư mục lớn quá 200 mục (trần `fs.glob`) thì chuyển sang `Glob(<chính xác đường dẫn>)` từng tệp.
   - **`path_line`:** tối đa 20; `ReadFile` cắt 512 KB; `line` ≤ số dòng thì `ok`, lớn hơn `missing` (`Detail="line_out_of_range"`); không đọc được (nhị phân, quá lớn) ⇒ `unverified`.
   - **Lệnh:** tách token đầu; `DevServerProfile.Tools` chứa token ⇒ `ok`; không chứa và `Degraded=false` ⇒ `missing`; hồ sơ không có ⇒ `unverified`. Script `pnpm run <x>` hoặc `npm run <x>`: đọc `package.json` (cắt 256 KB) và so `scripts`; `make <x>`: đọc `Makefile` và so mục tiêu `^<x>:`; `go test ./<pkg>`: kiểm thư mục `pkg` tồn tại. Không xác định được ⇒ `unverified`.
   - Không dev server (`ErrNotConnected`) ⇒ `Status="skipped"`, không lỗi, không chặn.
   - Tổng thời gian: `context.WithTimeout(60s)` cho toàn bộ kiểm; quá hạn ⇒ `Status="partial"`, phần chưa kiểm là `unverified`.
4. Vòng thử lại ở `AIGateway.Run` (task 04, thêm tại bước 6): `report.Ratio < ThresholdRatio` (cấu hình `REQUEST_AI_GROUNDING_THRESHOLD`, mặc định 0,8) và `!report.Retried` ⇒ gọi lại `Run` đúng **một lần** với biến prompt thêm `GroundingFeedback` (danh sách đường dẫn không tồn tại, mỗi mục ≤ 200 ký tự, tối đa 50 mục); kết quả lần hai được kiểm lại; vẫn dưới ngưỡng thì lưu với `NeedsReview=true`, `Retried=true`. Lần thử lại tính vào ngân sách và ledger như mọi lời gọi.
5. `grounding_extractors.go`: `ExtractSolutionRefs`, `ExtractDiagnosisRefs`, `ExtractPlanRefs` theo schema đã chốt ở SOL-007/008/012; `New` đọc từ trường `new` (hoặc tương đương mà SOL-007 chọn). Hàm thuần, dễ test; trả lỗi không làm hỏng sản phẩm (bước grounding không bao giờ làm lỗi sản phẩm, chỉ ghi `status=skipped`).
6. Báo cáo lưu cùng sản phẩm: `grounding_report` JSON do bước gọi ghi (trong `solutions.options` hoặc cột riêng do SOL-007 chọn). `REQUEST_AI_GROUNDING_FAILED` chỉ phát khi cấu hình `grounding_action=fail` (mặc định `flag`).
7. Metric `request_ai_grounding_ratio` (histogram) và log mức `info` chỉ gồm số lượng, không đường dẫn.

## Kiểm thử

- `grounding_checker_test.go` (`FakeRepoReader`, `FakeProfileReader`): đường dẫn không tồn tại vào `missing`; `New:true` không kiểm; `../etc/passwd` ⇒ `missing(unsafe_path)` và **Relay không được gọi** (fake đếm); gộp thư mục: 10 tệp cùng thư mục chỉ một `Glob`; 100 trần; `path_line` vượt số dòng ⇒ `missing`; lệnh `pnpm run lint` thiếu script ⇒ `missing`, `jest` không có trong hồ sơ ⇒ `missing`, hồ sơ rỗng ⇒ `unverified`; không dev server ⇒ `skipped`; quá 60 giây ⇒ `partial`.
- `grounding_report_test.go`: `ComputeRatio` bảng (0 kiểm ⇒ 1.0; 3 ok 1 missing ⇒ 0.75; `unverified` không tính).
- `grounding_extractors_test.go`: golden từ fixture JSON của SOL-007/008/012 (bổ sung khi những solution đó có fixture).
- `ai_gateway_test.go` (mở rộng task 04): ratio thấp ⇒ đúng một lần thử lại có `GroundingFeedback`; lần hai vẫn thấp ⇒ `NeedsReview=true` và **không** xoá phần sai; lần hai đủ ⇒ `NeedsReview=false`.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/...`.

## Tiêu chí hoàn thành

- [x] Đường dẫn không tồn tại vào `missing`; `new:true` không; ratio thấp ⇒ đúng một lần thử lại rồi `needs_review=true`.
- [x] Không đường dẫn độc nào (tuyệt đối, `..`) tới được Relay.
- [x] Không dev server: `skipped`, không chặn.
- [x] Tối đa 100 kiểm tra và 60 giây mỗi sản phẩm.
- [x] Log và metric không chứa đường dẫn hay lệnh nguyên văn.

## Rủi ro và lưu ý

- Chi phí và độ trễ Relay trên repo lớn chưa đo; dev server dùng chung nhiều Request: cân nhắc giới hạn đồng thời `GroundingChecker` mỗi dev server (đề xuất 2, chưa đo).
- `fs.glob` trần 200 kết quả: thư mục lớn có thể báo thiếu giả; đã có đường lùi kiểm từng tệp.
- Kiểm lệnh chỉ cấu trúc, không chạy lệnh (không thực thi gì trên dev server).
- Ngưỡng 0,8 chưa hiệu chỉnh (Q3).
