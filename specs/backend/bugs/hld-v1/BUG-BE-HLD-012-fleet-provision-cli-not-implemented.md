# BUG-BE-HLD-012 — CLI `orca fleet provision --project ... --concurrency N --dry-run` hoàn toàn không tồn tại

**Mức độ:** 🟡 MEDIUM (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/cli/` (thiếu subcommand `fleet`)
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.10/F31, CR-003)

---

## Mô tả

`docs/features/F31-fleet-provisioning.md` (CR-003) mô tả lệnh CLI `orca fleet provision --project <name> --concurrency <N> --dry-run` để bulk-provision nhiều Dev Server cùng lúc.

Thực tế:
- Không có thư mục `backend/src/cli/`.
- `backend/src/main/cli/` chỉ có `appimage-cli-wrapper.ts`, `cli-installer.ts`, `linux-bare-orca-dispatcher.ts`, `linux-terminal-orca-cli-shim.ts` — **không có subcommand `fleet` nào**.
- Grep toàn repo cho `"fleet provision"` / `.command('fleet'`: 0 kết quả.

Toàn bộ CR-003 (bulk provision với concurrency control, dry-run mode) chưa được implement.

## Hậu quả

- Không có cách nào provision nhiều Dev Server cùng lúc từ CLI — phải làm thủ công từng server một qua Admin Panel (nếu UI có hỗ trợ) hoặc gọi RPC `dev-server.*` lặp lại thủ công.
- Tính năng `--dry-run` (kiểm tra trước khi thực thi thật) không có cách nào dùng.

## Bằng chứng

- `find backend/src/cli -type d` → không tồn tại.
- `ls backend/src/main/cli/` → 4 file, không có `fleet-provision*`.
- Fleet config parser (`backend/src/shared/fleet-config-parser.ts`) và `groupSshTargetsByProject()` (`backend/src/shared/ssh-types.ts:203-238`) đã sẵn sàng làm nền tảng — chỉ thiếu lớp CLI orchestration ở trên.

## Đề xuất fix

1. Thêm subcommand `fleet` vào Orca CLI (`backend/src/main/cli/`), parse `--project`/`--concurrency`/`--dry-run`.
2. Tái sử dụng `parseFleetConfig()`/`groupSshTargetsByProject()` đã có để đọc danh sách server cần provision.
3. Với mỗi server, gọi `FleetBootstrapService`-equivalent (`bootstrapServer()`, xem [BUG-BE-HLD-013](./BUG-BE-HLD-013-fleet-bootstrap-missing-diskcheck-sha256-verify.md)) với concurrency limit (`p-limit` hoặc tương đương), hỗ trợ `--dry-run` chỉ log kế hoạch không thực thi.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.10 (F31), §6 mục 9 (Top 10)
- Doc gốc: `docs/features/F31-fleet-provisioning.md` (CR-003)
- Liên quan: [BUG-BE-HLD-013](./BUG-BE-HLD-013-fleet-bootstrap-missing-diskcheck-sha256-verify.md)
