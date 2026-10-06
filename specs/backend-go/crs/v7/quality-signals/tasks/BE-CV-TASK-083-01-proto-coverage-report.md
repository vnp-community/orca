# BE-CV-TASK-083-01: Proto `CoverageReport`, `FileCoverage`, `DiffCoverage`, `GetCoverage*`

**From Solution:** BE-CV-SOL-083
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_coverage.proto` (mới); dòng `rpc GetCoverage` trong `codeintel_quality_gate.proto` (của SOL-085)
**Depends on:** BE-CV-TASK-082-01; dòng `rpc` chờ BE-CV-SOL-085 tạo file
**Status:** [ ] TODO

## Context
C-DM §2.1 #20, §3.2. Hình dạng PQ-33. Số field do chủ sở hữu CR gán theo thứ tự khai báo, không đổi. `pct`, `diff_coverage` là `optional double`. `GetCoverageResponse` thêm `reason` (C-UI cần; lệch C-DM).

## Việc cần làm
1. Viết message theo SOL-083 mục 2.B; `LineRange{from,to}`.
2. `GetCoverageRequest{selector, oneof {run_id, head_commit}}`.
3. Thêm `rpc GetCoverage` khi SOL-085 có service; PR hợp đồng cập nhật bảng §3.2 cùng lúc (§8.3 mục 2).

## Kiểm thử
- `buf lint`/`buf breaking`; test phản chiếu: `estimated` không bắt buộc trường phần trăm (optional).

## Tiêu chí hoàn thành
- [ ] Stub biên dịch; không trùng tên.

## Rủi ro
Thứ tự với SOL-085 (SOL-083 Q3).
