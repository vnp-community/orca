# TASK-E2E-022: Cập nhật tài liệu triển khai

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-002, BUG-E2E-005
**Priority:** P2 · **Service/Area:** docs
**Files:**
- deploy/dev/README.md
- docs/guides/request/admin-enable-request-flow.md
- tests/stages/e2e/orca-flows/README.md
**Depends on:** TASK-E2E-019, 021
**Status:** `[~]` MỘT PHẦN (cập nhật 2026-10-09, chưa commit)

---

## Context

README "Local-only" còn hướng dẫn `docker compose up -d postgres vault nats` trong khi compose gốc không có Vault.

## Việc cần làm

- [~] Mô tả overlay `docker-compose.local.yml`; token request; `ensure-request-env.sh`; `redeploy-local.sh`. — _mô tả nằm ở comment script và `.env.example`; README deploy chưa cập nhật_
- [ ] Ghi `DOCKER_HOST` cho testcontainers dưới Colima/Docker Desktop. — _chưa ghi_
- [ ] Sửa mô tả cờ luồng (hai tầng) cho khớp. — _chưa sửa_

## Tiêu chí hoàn thành

- [ ] Một người mới dựng được stack local theo README mà không phải hỏi. — _chưa kiểm_

## Kiểm thử

Kiểm bằng cách làm theo từng bước trên máy sạch.

## Kết quả thực thi

Một phần. Đã cập nhật CONTRACT, `.env.example`, comment script. **Chưa cập nhật**: `deploy/dev/README.md`, `docs/guides/request/admin-enable-request-flow.md`, ghi chú `DOCKER_HOST` của Colima.
