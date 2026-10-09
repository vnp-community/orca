# TASK-E2E-005: Test P3: method backend gọi agent phải tồn tại ở agent

**From Solution:** SOL-E2E-001 · **Resolves:** BUG-E2E-004
**Priority:** P1 · **Service/Area:** ci
**Files:**
- backend-go/ci/check-agent-methods-called.sh (mới)
- backend-go/ci/check-agent-capability-golden.sh (mở rộng)
- agent/src/relay/agent-methods.ts hoặc nguồn danh sách method hiện có
**Depends on:** TASK-E2E-004
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Không có hợp đồng nào bắt buộc "mọi method backend gọi qua Relay đều có ở agent". `fs.checkPath` lọt vì thế.

## Việc cần làm

- [x] Quét Go: literal chuỗi tên method truyền cho `Relay`, `RelayByDevServer`, `RelayStream`, các hằng như `shellExecMethod`.
- [x] Trích danh sách method agent (từ nguồn dispatcher hoặc tệp vàng capability).
- [x] Báo method backend gọi mà agent không có; bỏ qua danh sách cho phép có lý do (method tuỳ chọn có kiểm tra capability).
- [x] Thêm vào CI chung với các script `check-*.sh` hiện có; chạy được trên macOS và Linux (G9).
- [ ] Tuỳ chọn: cảnh báo `MinAgentVersion` khi backend dùng method mới. — _không làm (tuỳ chọn)_

## Tiêu chí hoàn thành

- [x] Xoá `fs.checkPath` khỏi agent làm script đỏ.
- [x] Script xanh ở HEAD sau TASK-E2E-004.

## Kiểm thử

Chạy script ở hai trạng thái (có/không method).

## Kết quả thực thi

Xong phần kiểm tra: `backend-go/ci/check-agent-methods-called.sh` + `agent-methods-called-known-gaps.txt` (24 mục nợ đã biết, ratchet). Đã chứng minh bắt được lỗi khi đổi tên `fs.checkPath`. **Chưa gắn vào workflow CI** (xem TASK-029).
