# CR-DS-006 — Dev Server Agent Approval & Grouping

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-006 |
| **Tên** | Admin Approval Workflow + Dev Server Groups |
| **Loại** | Feature — Multi-tenant Access Control (Phase 1) |
| **Priority** | P1 — High |
| **Phiên bản** | v6.1 |
| **Ngày tạo** | 2026-08-28 |
| **Trạng thái** | ✅ Hoàn tất (backend + frontend) — RPC + admin-gating + test (BE-SOL-001/002), Admin UI đã triển khai (FE-SOL-001), deployed b15.openledger.vn |
| **Phụ thuộc** | CR-DS-001, CR-DS-002, CR-AG-004 |
| **Tác động HLD** | C3-components.md (Fleet), security.md |

---

## 1. Bối cảnh & Vấn đề

`backend-go`'s `infra-fleet-service` hiện coi mọi agent kết nối vào là tự động tin cậy và tự động hiển thị cho **mọi user trong tenant** — không có bước phê duyệt, và không có cách nhóm các dev server lại để giới hạn ai được thấy/dùng server nào. Với một tenant nhiều phòng ban (nhiều team dùng chung một Orca deployment), đây là một lỗ hổng: bất kỳ agent nào kết nối vào cũng lập tức khả dụng cho tất cả mọi người.

**Luồng đúng cần có** (theo yêu cầu):
```
Agent kết nối đến backend-go
    │
    ▼
Admin vào phê duyệt/chấp nhận (CR-DS-006, phần 2 — chưa làm)
    │
    ▼
Admin phân dev server vào 1 nhóm (CR-DS-006, phần 1 — ĐÃ LÀM)
    │
    ▼
Nhóm dev server được phân quyền theo nhóm user/phòng ban (CR-DS-007 — chưa làm)
    │
    ▼
User login, xác định phòng ban (CR-DS-008 — chưa làm) → chỉ thấy dev server
được phân quyền cho phòng ban của mình
```

CR này (DS-006) chỉ phủ **phần data model + grouping** (nửa dưới của sơ đồ trên: "phân dev server vào 1 nhóm"). Phần "admin phê duyệt agent mới kết nối" (RPC `ApproveDevServer`/`RejectDevServer` + màn hình admin) được thiết kế trong CR này nhưng **chưa triển khai** — xem `specs/backend-go/crs/v0/dev-server-access-control/solutions/BE-SOL-002-*.md`.

## 2. Rà soát những gì đã có sẵn (tránh làm lại)

- `agentwsserver.Registry` + `POST/GET /api/agent-token` (đã có, đã deploy) — cơ chế "mint token cho 1 devServerID cụ thể, chờ agent connect" của **direct-websocket** mode, kế thừa từ CR-AG-004 (thiết kế cho desktop/Electron, single-user). Endpoint này khoá bằng `ORCA_AGENT_API_SECRET` (server-only secret) — **không an toàn để expose thẳng cho user thường qua trình duyệt**; CR-DS-006 phần 2 cần một RPC mới, xác thực theo tenant/admin session thật, gọi lại registry này ở phía trong.
- `ProjectGroup` (project-service) — cây phân cấp (`parent_group_id`) cho Project, KHÔNG liên kết Team/Department. Dùng làm **mẫu thiết kế** cho `DevServerGroup` (không tái sử dụng bảng, vì domain khác — repo vs dev server).
- Tenant-service's Company→Department→Team→User (4 lớp) — dùng cho **settings inheritance**, không phải authorization. CR-DS-007 sẽ thêm bảng mapping mới, không sửa cấu trúc này.

## 3. Giải pháp — Phase 1 (data model, ĐÃ TRIỂN KHAI)

> **Cập nhật triển khai (2026-08-28):** migration thực tế là **`0008`**, không
> phải `0007` như phác thảo ban đầu ở mục này — khi deploy lên b15 phát hiện
> một migration `0007` KHÁC đang chạy dở (dirty), do một phiên làm việc song
> song khác tạo ra (chưa commit), thêm cột `status` cho một khái niệm hoàn
> toàn khác (health/bootstrap: `pending|healthy|degraded|unhealthy`). Để
> tránh đụng cả về số thứ tự migration lẫn tên cột, cột approval-status ở
> đây được đặt tên **`approval_status`** (không phải `status`), và migration
> đổi thành `0008_dev_server_approval_status_and_groups`. Nội dung/ý nghĩa
> giữ nguyên như mô tả dưới đây, chỉ đổi tên cột + số thứ tự.

### 3.1 `infra.dev_servers` — thêm 2 cột
```sql
ALTER TABLE infra.dev_servers
  ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'approved'
    CHECK (approval_status IN ('pending_approval', 'approved', 'rejected')),
  ADD COLUMN group_id UUID REFERENCES infra.dev_server_groups(id);
```
Cột-level default là `'approved'` (bảo vệ mọi row insert ngoài Go domain layer); tầng ứng dụng (Go) mới thực sự đặt `pending_approval` cho dev server **mới tạo từ nay về sau** — `domain.NewDevServer` luôn set giá trị này. `ADD COLUMN ... DEFAULT` tự backfill mọi row cũ thành `'approved'` ngay trong cùng câu ALTER (Postgres áp default cho row có sẵn khi `ADD COLUMN NOT NULL`), nên không cần `UPDATE` riêng — dev server đã tồn tại trước migration không bị khoá quyền truy cập hiện có.

### 3.2 Bảng mới `infra.dev_server_groups`
```sql
CREATE TABLE infra.dev_server_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  name TEXT NOT NULL,
  parent_group_id UUID REFERENCES infra.dev_server_groups(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
Cây phân cấp giống `ProjectGroup`, scope theo tenant, RLS `tenant_isolation` giống mọi bảng khác trong schema này, không có mapping Department ở migration này (CR-DS-007 thêm bảng join riêng để 2 CR có thể ship/rollback độc lập).

### 3.3 Domain + usecase (backend-go, `infra-fleet-service`)
- `domain.DevServer` — thêm `Status DevServerStatus`, `GroupID string`.
- `domain.DevServerGroup{ID, TenantID, Name, ParentGroupID}` — entity mới.
- Usecase mới: `CreateDevServerGroup`, `ListDevServerGroups` (tenant-wide), `AssignDevServerGroup` (gán 1 dev server vào 1 group).
- `RegisterDevServer`'s usecase set `Status = pending_approval` mặc định khi tạo mới (không đổi hành vi hiện có về host/mode).

**Chưa làm ở Phase 1** (cố tình để lại cho Phase 2/CR sau, xem specs để biết trạng thái từng phần):
- RPC + UI để admin approve/reject.
- Field `status` chưa được ENFORCE ở bất kỳ đâu (một `pending_approval` dev server vẫn dùng được bình thường hôm nay — thêm cột trước, gate sau, để tránh khoá nhầm user đang dùng hệ thống giữa chừng một pha chưa hoàn chỉnh).

## 4. Acceptance Criteria

- [x] Migration `infra.dev_servers.status`/`group_id` + bảng `infra.dev_server_groups` chạy sạch trên Postgres thật (Testcontainers + b15).
- [x] `DevServer` domain struct + usecase mới có unit + integration test.
- [ ] RPC `ApproveDevServer`/`RejectDevServer` (Phase 2 — xem BE-SOL-002).
- [ ] Admin UI để duyệt + gán nhóm (Phase 2 — xem FE-SOL-001).
- [ ] `status != approved` thực sự chặn truy cập (Phase 2, cùng với CR-DS-007's OPA policy — không gate một mình vì chưa có department mapping để fallback).
