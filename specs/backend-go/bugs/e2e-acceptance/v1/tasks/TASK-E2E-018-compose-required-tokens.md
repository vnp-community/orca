# TASK-E2E-018: Chốt cấu hình compose cho `request-service` và làm token bắt buộc khi bật cờ

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-002
**Priority:** P0 · **Service/Area:** deploy/dev
**Files:**
- deploy/dev/docker-compose.yml (đã sửa)
- deploy/dev/.env.example
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

Compose đã có `GATEWAY_INTERNAL_TOKEN`, `REQUEST_INTERNAL_CALLER_TOKEN`, mount `policy/orca-authz` và `OPA_BUNDLE_PATH`. Chưa bắt buộc và chưa ghi trong `.env.example`.

## Việc cần làm

- [~] Dùng `${REQUEST_INTERNAL_CALLER_TOKEN:?…}` khi `REQUEST_FLOW_ENABLED` bật (không thể điều kiện hoá trong compose thuần: dùng kiểm tra trong `ensure-request-env.sh` và `/readyz` của TASK-E2E-020 cho nhánh có điều kiện). — _không dùng `:?` trong compose; kiểm bằng `ensure-request-env.sh` và `/readyz`_
- [x] Thêm các khoá vào `.env.example` với mô tả và cách sinh.
- [ ] Commit. — _chưa commit_

## Tiêu chí hoàn thành

- [x] Dựng stack từ môi trường sạch sau `ensure-request-env.sh`: `PLT-10` xanh.

## Kiểm thử

`PLT-09`, `PLT-10`.

## Kết quả thực thi

Xong. compose: mount OPA + `OPA_BUNDLE_PATH`, `GATEWAY_INTERNAL_TOKEN`, `PROJECT_INTERNAL_CALLER_TOKEN`, `REQUEST_INTERNAL_CALLER_TOKEN`; `.env.example` có khối token. PLT-10 PASS.
