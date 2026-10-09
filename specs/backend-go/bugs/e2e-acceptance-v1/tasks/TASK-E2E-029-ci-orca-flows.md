# TASK-E2E-029: Đưa `orca-flows` vào CI (smoke hằng đêm, `--strict` định kỳ)

**From Solution:** SOL-E2E-006 · **Resolves:** toàn bộ
**Priority:** P2 · **Service/Area:** ci
**Files:**
- .github/workflows/ (mới hoặc mở rộng)
- backend-go/ci/ (script dựng stack và chạy)
- tests/stages/e2e/orca-flows/run_acceptance.py
**Depends on:** TASK-E2E-026
**Status:** `[ ]` CHƯA LÀM (cập nhật 2026-10-09, chưa commit)

---

## Context

Bộ e2e xuyên tầng chỉ chạy tay nên lỗi RC-A chỉ lộ khi có người chạy.

## Việc cần làm

- [ ] Job dựng stack bằng compose (không Vault ngoài), dev server là agent chạy ngay trong job với repo tạm. — _chưa làm_
- [ ] Hằng đêm: `platform workflow request matrix exits` với `--ai-stub`; hằng tuần: `--strict`. — _chưa làm_
- [ ] Lưu `reports/last-run.{json,md}` làm artifact; thông báo khi đỏ. — _chưa làm_

## Tiêu chí hoàn thành

- [ ] Job xanh 5 đêm liên tiếp trước khi thành cổng chặn. — _chưa làm_

## Kiểm thử

Chạy thử bằng `workflow_dispatch`.

## Kết quả thực thi

Chưa làm. Chưa có workflow CI cho `orca-flows`, `check-grpc-dials.sh`, `check-agent-methods-called.sh`.
