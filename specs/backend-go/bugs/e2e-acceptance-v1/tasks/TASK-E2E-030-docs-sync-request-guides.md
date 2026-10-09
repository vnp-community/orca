# TASK-E2E-030: Đồng bộ tài liệu với mã

**From Solution:** SOL-E2E-006 · **Resolves:** OBS-D
**Priority:** P2 · **Service/Area:** docs
**Files:**
- docs/guides/request/README.md
- docs/features/F36-multi-server-workflow-orchestration.md
- specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md
**Depends on:** TASK-E2E-001, 008
**Status:** `[~]` MỘT PHẦN (cập nhật 2026-10-09, chưa commit)

---

## Context

`guides/request` vẫn ghi Solution/Plan/Phase "chưa có"; F36 mô tả đích `project:` chưa nói `server:` và `connection:`.

## Việc cần làm

- [ ] Cập nhật bảng trạng thái giai đoạn theo kết quả `orca-flows`. — _chưa làm_
- [ ] F36: ghi ba dạng `target` và điều kiện của `project:`. — _chưa làm_
- [x] Ghi cách đọc/ trả lời Clarification qua API.

## Tiêu chí hoàn thành

- [ ] Không còn câu "chưa có" sai. — _guide request còn câu cũ_

## Kiểm thử

Đối chiếu từng câu với bộ e2e.

## Kết quả thực thi

Một phần. CONTRACT đã đồng bộ (clarification, valueType, retryAfterMs, mã lỗi). **Chưa làm**: `docs/guides/request/README`, F36.
