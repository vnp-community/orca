# BE-CV-TASK-090-04: Use case `ExportReviewReport` (ghép nguồn, lỗi từng phần → `warnings`)

**From Solution:** BE-CV-SOL-090-review-report-model
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/usecase/export_review_report.go`, `review_report_ports.go` (mới)
**Depends on:** BE-CV-TASK-090-02, 090-03, BE-CV-TASK-085-06
**Status:** [ ] TODO

## Việc cần làm
1. Port: `Overlay`, `Gate` (`EvaluateQualityGate`), `QualityFindings`, `StructureFindings`, `ContractDiff`, `Architecture`, `Erd`, `RepoProviderResolver` (từ `Repo.url`, L6).
2. Thu thập song song có giới hạn; mỗi nguồn lỗi ⇒ mã `*_unavailable`, không fail RPC; **không** gọi agent trực tiếp.
3. `sections[]` lọc phần; `include_people/include_waiver_reasons` nhận nhưng chưa tác động (L7).
4. `generated_for`; bảng mã `warnings` (SOL-090 §2.5).

## Kiểm thử
- Port giả: từng nguồn lỗi/timeout; thiếu run ⇒ `gate.unknown`; `provider` GitHub/GitLab/self-hosted; kết quả ≤ 2 MiB.

## Tiêu chí hoàn thành
- [ ] RPC chỉ lỗi cờ/quyền/tham số; [ ] không điền giá trị giả; [ ] `sections` hoạt động.

## Rủi ro
- Nguồn chưa tồn tại (036/037/038/031) ⇒ chỉ test bằng fake tới khi merge.
