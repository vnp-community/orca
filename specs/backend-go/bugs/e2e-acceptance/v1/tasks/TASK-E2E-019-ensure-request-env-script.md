# TASK-E2E-019: Script `ensure-request-env.sh` sinh token nội bộ

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-002
**Priority:** P1 · **Service/Area:** deploy/dev
**Files:**
- deploy/dev/scripts/ensure-request-env.sh (mới, theo mẫu ensure-mcp-env.sh)
- deploy/dev/scripts/sync-to-server.sh
**Depends on:** TASK-E2E-018
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Token phải khớp nhau giữa gateway, `request-service` và `issue-status-sync`; tạo tay dễ sai.

## Việc cần làm

- [x] Chỉ điền khoá thiếu/rỗng: `REQUEST_INTERNAL_CALLER_TOKEN`, `REQUEST_SERVICE_INTERNAL_TOKEN`; sinh ngẫu nhiên bằng `openssl rand`; không in giá trị.
- [~] Chạy được local và `--remote` như script mẫu. — _chạy local; `--remote` chưa kiểm_
- [x] `sync-to-server.sh` gọi trước `docker compose up`.
- [x] Không bao giờ ghi đè giá trị đã đặt.

## Tiêu chí hoàn thành

- [x] Chạy hai lần không đổi gì ở lần hai.
- [x] Token trong gateway và `request-service` bằng nhau.

## Kiểm thử

Chạy trên `.env` mẫu trống.

## Kết quả thực thi

Xong. `deploy/dev/scripts/ensure-request-env.sh` (idempotent, đã kiểm), được `sync-to-server.sh` gọi.
