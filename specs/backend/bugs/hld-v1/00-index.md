# hld-v1 — Bug index (Backend vs HLD/Feature Docs audit, 2026-08-08/09)

**Nguồn:** `audit/backend/backend-vs-design-review.md` — audit 2 vòng đối chiếu `backend/src/**` với `docs/hld/{backend,dev-server,web-server}-server-architecture.md` (Vòng 1) và `docs/features/F22–F40` (Vòng 2), cùng rà soát `docs/adrs/v2/ADR-016..020`.

Mỗi bug dưới đây có bằng chứng `file:line` xác nhận trực tiếp từ code (không suy đoán). Trước khi bắt đầu fix, đọc kỹ audit report gốc để có đầy đủ ngữ cảnh — mỗi bug ticket chỉ trích phần liên quan.

## Danh sách bug (20)

| ID | Mức độ | Domain | Mô tả ngắn |
|----|--------|--------|------------|
| [BUG-BE-HLD-001](./BUG-BE-HLD-001-profile-rpc-handler-requireadmin-stub-no-role-check.md) | 🔴 CRITICAL | RBAC/Profile | `requireAdmin(ctx)` trong `profile-rpc-handler.ts` không check role — bypass hoàn toàn |
| [BUG-BE-HLD-002](./BUG-BE-HLD-002-project-rpc-requireowneroradmin-no-admin-check.md) | 🟠 HIGH | RBAC/Project | `requireOwnerOrAdmin` không check admin (dead code); `project.create` không giới hạn quyền |
| [BUG-BE-HLD-003](./BUG-BE-HLD-003-rbac-fragmented-no-policy-table.md) | 🟠 HIGH | RBAC | Không có `hasPermission()` policy table; RBAC phân mảnh 4+ cơ chế không tương thích |
| [BUG-BE-HLD-004](./BUG-BE-HLD-004-github-gitlab-cli-runs-on-backend-not-relayed.md) | 🟠 HIGH | Remote Integration | Backend tự thực thi `gh`/`glab` thay vì relay Dev Server Agent |
| [BUG-BE-HLD-005](./BUG-BE-HLD-005-gh-config-dir-never-passed-to-relay.md) | 🟠 HIGH | Remote Integration | `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user isolation không hoạt động dù có 2 luồng relay hẹp |
| [BUG-BE-HLD-006](./BUG-BE-HLD-006-admin-sessions-list-stub-empty.md) | 🟠 HIGH | Admin Panel | `GET /admin/api/sessions` là stub rỗng |
| [BUG-BE-HLD-007](./BUG-BE-HLD-007-admin-access-policies-api-missing.md) | 🟠 HIGH | Admin Panel | Toàn bộ backend API Access Policies (PoliciesPage) không tồn tại |
| [BUG-BE-HLD-008](./BUG-BE-HLD-008-workflow-provider-selection-not-implemented.md) | 🟠 HIGH | Workflow | Chọn AI provider theo từng step: 0% code |
| [BUG-BE-HLD-009](./BUG-BE-HLD-009-workflow-pause-resume-not-implemented.md) | 🟠 HIGH | Workflow | Pause/Resume (user-triggered) không tồn tại — chỉ có crash-recovery resume |
| [BUG-BE-HLD-010](./BUG-BE-HLD-010-fleet-health-monitor-no-real-metrics-still-broken.md) | 🟡 MEDIUM | Fleet | CPU/RAM/disk/latency vẫn không thu thập — re-open, tracker cũ (BE-FLEET-002) đánh dấu FIXED sai |
| [BUG-BE-HLD-011](./BUG-BE-HLD-011-session-manager-no-auto-respawn-no-idle-timeout-config.md) | 🟡 MEDIUM | Session | Auto-respawn (max 3) và `SESSION_IDLE_TIMEOUT_MS` chưa cài đặt |
| [BUG-BE-HLD-012](./BUG-BE-HLD-012-fleet-provision-cli-not-implemented.md) | 🟡 MEDIUM | Fleet | CLI `orca fleet provision` hoàn toàn không tồn tại |
| [BUG-BE-HLD-013](./BUG-BE-HLD-013-fleet-bootstrap-missing-diskcheck-sha256-verify.md) | 🟡 MEDIUM | Fleet | Bootstrap thiếu disk-check + SHA256 verify relay binary |
| [BUG-BE-HLD-014](./BUG-BE-HLD-014-ai-provider-key-rotation-not-implemented.md) | 🟡 MEDIUM | AI Provider | Key rotation (grace period, status, audit log) không tồn tại |
| [BUG-BE-HLD-015](./BUG-BE-HLD-015-ai-provider-quota-alert-not-implemented.md) | 🟡 MEDIUM | AI Provider | Cảnh báo quota 80% không tồn tại — chỉ phát hiện sau khi vượt |
| [BUG-BE-HLD-016](./BUG-BE-HLD-016-db-migration-table-naming-collision-v5-prefix.md) | 🟡 MEDIUM | Database | Migration 0004/0007 đụng độ tên bảng `orca_projects` → `orca_v5_projects`, tài liệu chưa cập nhật |
| [BUG-BE-HLD-017](./BUG-BE-HLD-017-electron-adapter-missing-platform-abstraction-asymmetric.md) | 🟡 MEDIUM | Platform | `ElectronAdapter` không tồn tại — Platform Abstraction Layer bất đối xứng |
| [BUG-BE-HLD-018](./BUG-BE-HLD-018-dev-server-git-provider-missing-operations.md) | 🟡 MEDIUM | Remote Git | `DevServerGitProvider` thiếu git log/AI commit-msg/branch-diff cho repo trên Dev Server |
| [BUG-BE-HLD-019](./BUG-BE-HLD-019-agent-ws-protocol-keepalive-closecode-version-mismatch.md) | 🟡 MEDIUM | Agent WS | Keepalive timing sai số liệu, close code 4001-4003 không tồn tại, version-check chết |
| [BUG-BE-HLD-020](./BUG-BE-HLD-020-project-devserver-binding-not-rebindable.md) | 🟡 MEDIUM | Project | Không thể rebind Dev Server cho project đã tồn tại |

## Nhóm ưu tiên fix ngay (Security — CRITICAL/HIGH)

```
1. BUG-BE-HLD-001  requireAdmin RPC stub                    ← permission bypass, ai login cũng set được policy công ty
2. BUG-BE-HLD-002  requireOwnerOrAdmin dead code             ← admin không override được project
3. BUG-BE-HLD-003  RBAC phân mảnh                            ← root cause của #1, #2
4. BUG-BE-HLD-004  GitHub/GitLab chạy trên Backend            ← vi phạm "auth never through gateway"
5. BUG-BE-HLD-005  GH_CONFIG_DIR không được truyền            ← per-user CLI isolation broken end-to-end
```

## Ghi chú quan trọng

- **BUG-BE-HLD-010** re-open lại một bug đã có (`../fleet/BUG-BE-FLEET-002-health-monitor-no-relay-metrics.md`) mà `BUG-SOLUTION-STATUS.md` đánh dấu sai là "✅ FIXED" — cần cập nhật status đó khi xử lý ticket này.
- **BUG-BE-HLD-011** liên quan `../auth/BUG-AUTH-003-session-manager-no-idle-timeout.md` (cũng đánh dấu FIXED) — cần đối chiếu lại tương tự.
- **BUG-BE-HLD-004** liên quan `../project-integration/BUG-PI-001-credential-service-missing-github.md` (đánh dấu FIXED với 1 solution đề xuất) — cần xác nhận solution đó có thật sự giải quyết vấn đề mới phát hiện (Category A vẫn chạy cục bộ trên Backend) hay không.
- 5 bug về docs-only drift đã KHÔNG được đưa vào danh sách này để giữ tập trung vào bug code thật (vd RPC namespace naming, DB Schema §10 tổng thể) — xem đầy đủ trong `audit/backend/backend-vs-design-review.md` nếu cần dọn tài liệu.

## Giải pháp (2026-08-09)

Toàn bộ 20 bug đã có solution code-level tại [`solutions/`](./solutions/00-index.md) — 12 file, căn cứ theo `specs/backend/tdd/v4`+`v5`. Xem [`solutions/00-index.md`](./solutions/00-index.md) để có bảng ánh xạ bug→solution, thứ tự merge khuyến nghị, và 4 phát hiện phụ quan trọng lộ ra trong lúc viết fix (vd `WorkflowOrchestrator.executeStep()` type-mismatch, `pty-handler.ts` không đọc userId, patch RBAC cần áp cả cho `desktop/`, `AuditLogger` insert sai cột).

## Task thực thi (2026-08-09)

12 solution đã được chia thành **33 task atomic** tại [`tasks/`](./tasks/00-index.md) (`TASK-HLD-001` → `033`), mỗi task đủ code cụ thể để 1 AI coding agent nhận và thực thi độc lập, kèm dependency graph + nhóm theo sprint khuyến nghị. Xem [`tasks/00-index.md`](./tasks/00-index.md).

## Tham khảo

- Audit report đầy đủ: [`audit/backend/backend-vs-design-review.md`](../../../../audit/backend/backend-vs-design-review.md)
- Bug index toàn cục: [`specs/BUGS-INDEX.md`](../../../BUGS-INDEX.md), [`specs/backend/bugs/BUG-SOLUTION-STATUS.md`](../BUG-SOLUTION-STATUS.md)
