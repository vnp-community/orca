# TASK-E2E-021: Sau build không mất mount; script `redeploy-local.sh` cho vòng lặp phát triển

**From Solution:** SOL-E2E-004 · **Resolves:** BUG-E2E-005
**Priority:** P1 · **Service/Area:** deploy/dev
**Files:**
- deploy/dev/scripts/build-local.sh (đã sửa dùng rsync)
- deploy/dev/scripts/redeploy-local.sh (mới)
**Depends on:** không
**Status:** `[x]` XONG (cập nhật 2026-10-09, chưa commit)

---

## Context

`dist/` và `policy/` giờ giữ inode. Mount theo file nhị phân vẫn cần restart để nạp bản mới.

## Việc cần làm

- [ ] Commit thay đổi `build-local.sh`. — _chưa commit_
- [x] `redeploy-local.sh <service…>`: build các service đó (`GOARCH` tự nhận theo `docker info`), `docker compose up -d` với cả hai file compose khi có overlay, `docker restart` các container tương ứng, rồi chạy `run_acceptance.py platform`.
- [x] Không recreate Vault dev (dữ liệu trong bộ nhớ).

## Tiêu chí hoàn thành

- [x] Chạy `redeploy-local.sh request-service` rồi `PLT-*` xanh.

## Kiểm thử

Chạy thật hai lần liên tiếp.

## Kết quả thực thi

Xong. `build-local.sh` dùng rsync; `redeploy-local.sh` (nhận kiến trúc, build, chạy migration của service vừa dựng, `up -d`, restart toàn bộ service Go khi có container bị tạo lại, chạy smoke PLT). Vòng v2 sửa lỗi SIGPIPE khi lọc danh sách migrate.
