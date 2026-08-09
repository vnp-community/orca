# BUG-BE-HLD-020 — Không thể rebind/đổi Dev Server cho project đã tồn tại (binding bất biến sau khi tạo)

**Mức độ:** 🟡 MEDIUM (Feature gap)
**Status:** 🔴 Open
**Module:** `backend/src/main/project/ProjectService.ts`, `project-rpc-handler.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.13/F34)

---

## Mô tả

`docs/features/F34-project-dev-server-binding.md` liệt "Dev Server binding: chọn/đổi dev server cho project" như tính năng trong Project Settings dành cho Lead/Admin.

Thực tế `ProjectService.update()` (`ProjectService.ts:186-200`) chỉ patch `name`/`description`/`defaultBranch`/`visibility`. Field `devServerId` **không nằm trong schema `UpdateProjectParam.patch`** (`project-rpc-handler.ts:40-48`), và **không có method `bindDevServer`/`rebindDevServer`/`updateDevServer` nào** trong `ProjectService`. Binding chỉ được set 1 lần lúc `create()`, sau đó bất biến vĩnh viễn.

## Hậu quả

- Nếu 1 Dev Server bị decommission hoặc đổi sang server khác (nâng cấp phần cứng, chuyển vùng...), **không có cách nào** chuyển project đang gắn với nó sang server mới thông qua API — phải xoá và tạo lại project (mất lịch sử, task, workflow liên kết).
- Mâu thuẫn trực tiếp với user-flow F34 mô tả (Lead/Admin vào Project Settings đổi dev server).

## Bằng chứng

- `backend/src/main/project/ProjectService.ts:186-200` — `update()` không có `devServerId` trong danh sách field patch được.
- `backend/src/main/project/project-rpc-handler.ts:40-48` — `UpdateProjectParam.patch` schema xác nhận thiếu field.
- `backend/src/main/project/ProjectService.ts:82-136` — `create()` là nơi duy nhất set `devServerId`.

## Đề xuất fix

1. Thêm `devServerId` vào `UpdateProjectParam.patch` schema và logic `ProjectService.update()`.
2. Validate dev server mới tồn tại (dùng lại `DevServerManager.get()` như đã làm ở `create()`) trước khi cho phép rebind.
3. Cân nhắc: khi rebind, có cần cảnh báo/chặn nếu đang có workflow execution/task đang chạy dở trên server cũ hay không (tránh orphan execution) — nên làm rõ business rule này trước khi implement.
4. Giới hạn quyền rebind theo đúng RBAC (Lead/Admin), phụ thuộc vào việc fix [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md) trước.

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.13 (F34)
- Doc gốc: `docs/features/F34-project-dev-server-binding.md`
- Liên quan: [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md)
