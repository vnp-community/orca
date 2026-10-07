# BE-CV-TASK-037-06: Giao diện `Detector`, use case `ListFindings`, cache snapshot, `origin`

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/finding_detector.go`, `list_findings.go`, `list_findings_test.go`; `internal/usecase/structure_detectors.go` (mới: bốn bộ lớp/vòng/hotspot/mã chết bọc domain + collector)
**Depends on:** BE-CV-TASK-037-02, 037-03, 037-04, 037-05; BE-CV-SOL-021 (collector `StructuralFacts`), BE-CV-SOL-030 (`Log`, `ReadFile`), BE-CV-SOL-012/013, BE-CV-SOL-022
**Status:** [x] DONE

---

## Context

Solution §2.C, §2.E. Bộ `sql.*` của CR-038 cắm vào cùng giao diện (BE-CV-SOL-038-static-tenant-filter-rule). Quyền kiểm trước cache.

## Việc cần làm

1. `finding_detector.go`: `type Detector interface{ Name() string; Rules() []string; Detect(ctx, DetectInput) ([]Finding, error) }`; `DetectInput{Repo RepoRef, Binding, WindowDays int, ChangedFiles []string (có thể nil), Reader RepoSourceReader, Facts StructuralFactsSource}`. Đăng ký bộ qua slice ở composition root; `rules[]` của request lọc bộ nào chạy; bộ đánh dấu `Experimental()` chỉ chạy khi `rules[]` nêu **tường minh** rule của nó (dùng cho `sql.*`, solution 038).
2. `structure_detectors.go`: bộ `layer` (`layerImports` ×4 `pair`, hoặc không `pair` = cả 4), `cycles`, `hotspot` (`importInDegree`, `fileSizes`, `Log`, `hotspot_window_days` từ `tenant_settings` qua port), `deadexport`, mỗi bộ timeout riêng (≤ 55 s agent / 90 s Go; đặt 60 s), lỗi ⇒ `DetectorStatus`. `CODEINTEL_AGENT_UNSUPPORTED` ⇒ `skipped`.
3. `list_findings.go`: cờ ⇒ quyền `read` ⇒ `selector`; `scope` rỗng ⇒ `CHANGED` (UI mặc định). Chạy bộ song song (hạn mức đồng thời CR-013), singleflight theo khoá; ghi `graph_snapshots(view="findings", head_commit=indexedCommit, params_hash)` (PQ-15; không chứa dismiss). Hợp nhất, gán `owner` (cho `CHANGED`/trang hiện tại, tối đa 200 tệp/lần để hạn chế `shortlog`), `origin` (2.C bước 7; cần `ChangedFiles` từ overlay qua port mềm `ChangedFilesSource`), lọc `rules/severities/path_prefix`, ghép dismiss (`ListByKeys` theo trang), `include_dismissed`, phân trang (`page_token` đục, ≤ 512), `total_count`, `dismissed_count`, `truncated` (≤ 2 000 finding/lần tính).
4. Index cũ (`ComputeFreshness` của 036) ⇒ hạ `confidence` một bậc; trả `index_freshness`.
5. Mã lỗi theo solution §2.E; `CODEINTEL_TIMEOUT` ⇒ hoàn tất nền.

## Kiểm thử

- `go test ./services/code-intel-service/internal/usecase/ -run ListFindings -race` với fake agent (tệp vàng), fake `Log`, fake dismissal repo: một bộ `cycles` timeout ⇒ các bộ khác vẫn trả + `detectors` đúng; `scope=CHANGED` chỉ finding chạm tệp đổi; `origin` đủ bốn giá trị (có/không snapshot cũ); dismissed ẩn/hiện, `dismissed_count`; bộ `Experimental` không chạy khi `rules` rỗng; phân trang ổn định; quyền sai không chạm agent/cache; hai tenant không đọc chéo cache.
- Hiệu năng (ghi, không chặn): tổng thời gian trên fixture; ngân sách < 60 s lần đầu là giả định (CR-071).

## Tiêu chí hoàn thành

- [x] Tiêu chí §9 mục "một bộ lỗi", "`scope=CHANGED`", "hạn mức" đạt.
- [x] Dismiss không nằm trong snapshot.

## Rủi ro và lưu ý

- `shortlog` mỗi tệp tốn một lệnh qua SSH; giới hạn 200 tệp/lần và cache theo `(tệp, cửa sổ)` ngắn hạn.
