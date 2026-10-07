# BE-CV-TASK-036-08: Bảng quy tắc điểm rủi ro và `ScoreRisk` (`modelVersion "1"`)

**From Solution:** BE-CV-SOL-036-reading-order-and-risk
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/overlayrisk/risk_rules.go`, `risk_scoring.go` và `_test.go` (mới)
**Depends on:** BE-CV-TASK-036-03
**Status:** [x] DONE

---

## Context

Solution reading-order §2.C. Mọi điểm hiển thị được; không điểm ẩn. Hằng số khởi điểm chưa hiệu chỉnh.

## Việc cần làm

1. `risk_rules.go`: hằng có tên (`pointsSizeMedium`, …), `ModelVersion = "1"`, ngưỡng mức `(2,5,9)`, danh sách `SensitivePaths` (5 tiền tố của solution). Comment hai dòng: "giá trị khởi điểm, chưa hiệu chỉnh; đổi ⇒ tăng ModelVersion".
2. `risk_scoring.go`: `ScoreRisk(in RiskInput) RiskAssessment`. `RiskInput`: `changedFiles` (đủ, trước khi cắt), `changedSymbols`, kết quả `impact` (risk cao nhất, danh sách `tested`), `affectedFlows`, `affectedClusters`, `touchedContracts`, `migrationsDestructive bool`, `violations`, `freshness`, `missingSources []string`, `toolRisk`. Sinh `RiskReason{code, points, messageKey, params, evidence}` cho từng quy tắc (chọn một trong cặp `SIZE_*`, `IMPACT_*`, `FLOWS`, `CLUSTERS`); `messageKey` dạng `codeintel.risk.<code>`; `evidence` là `SymbolRef` hoặc đường dẫn/`findingKey` chuỗi; `VIOLATION` tối đa 2 finding `introduced`.
3. `incomplete=true` nếu `missingSources` khác rỗng, impact có `UNKNOWN`/lỗi/hết hạn, freshness `behind|missing`; thêm `DATA_MISSING` (`points=0`, `params.source`) mỗi nguồn; **không** thay đổi mức vì thiếu. `confidence` theo solution §2.C. `toolRisk` chỉ sao chép.
4. `UNTESTED`: đếm `tested=="no"`; `unknown` không tính là chưa test. `MIGRATION`/`CONTRACT` vẫn tính theo đường dẫn khi nguồn 038 vắng; khi đó thêm `DATA_MISSING{source:"contract_diff"}`.
5. Sắp `reasons` theo `points` giảm dần rồi `code`; `score` là tổng; `level` từ ngưỡng; kiểu `level` chuỗi HOA.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/overlayrisk/...`: table-driven từng dòng bảng; biên điểm 2/3, 5/6, 9/10; `score == Σ points` (property test); mọi reason (trừ `DATA_MISSING`) có `messageKey` và `evidence` không rỗng; impact lỗi toàn bộ ⇒ `incomplete`, mức ≥ mức từ nguồn còn lại; tệp test/doc/generated không vào `SIZE_*`; dữ liệu 3 000 tệp ⇒ điểm bằng tính trước khi cắt; `SENSITIVE_PATH` cho từng tiền tố; tiền tố tương tự (`backend-go/services/auth-service-x`) không khớp nhầm (so khớp theo ranh giới thư mục).

## Tiêu chí hoàn thành

- [x] Bảng test phủ mọi mã; hằng có tên; `ModelVersion` được test.
- [x] Không phụ thuộc ngoài stdlib.

## Rủi ro và lưu ý

- Hiệu chỉnh bằng fixture PR lịch sử thuộc CR-070 (`BE-CV-SOL-070-collector-golden-contract`); trước đó ghi rõ "chưa tin cậy HIGH/CRITICAL" ở README service.
