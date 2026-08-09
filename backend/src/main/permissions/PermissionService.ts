/**
 * PermissionService — Role → resource → action policy table trung tâm (BUG-BE-HLD-003).
 *
 * Thay thế dần 4 cơ chế RBAC phân mảnh:
 *   1. resolveUserPermissions() (rbac-types.ts) — giữ nguyên, phục vụ fleet/server access,
 *      không có khái niệm resource/action nên KHÔNG migrate (đúng theo phạm vi khác — server
 *      pairing token, không phải RPC permission check).
 *   2. requireAdmin (admin-middleware.ts, HTTP route)      → migrate ở Bước B (chưa làm — optional).
 *   3. requireAdmin (profile-rpc-handler.ts, RPC — đã fix ở TASK-HLD-002 dùng getUserRole trực tiếp)
 *      → migrate sang PermissionService.hasPermission() ở Bước C (chưa làm — optional).
 *   4. requireOwnerOrAdmin (project-rpc-handler.ts, đã fix ở TASK-HLD-003) → migrate ở Bước C (chưa làm).
 *   TaskGrantService.resolvePermission() — GIỮ NGUYÊN, đặc thù cho BFS ancestor + apply_tree
 *   của task graph (đúng như ticket 003 đề xuất), nhưng expose thêm 1 adapter mỏng qua cùng
 *   interface `hasPermission` để nơi gọi không cần biết đây là task graph hay resource thường.
 *
 * Chỉ Bước A (file này) được implement trong TASK-HLD-005 — Bước B/C là PR riêng
 * sau khi Bước A đã review xong (đổi cách admin-router.ts đăng ký route).
 *
 * @module main/permissions/PermissionService
 */
import type { OrcaUser } from '../../shared/rbac-types'
import type { AuthUserStore } from '../auth/auth-user-store'

export type PermissionResource = 'company' | 'department' | 'project' | 'task'
export type PermissionAction = 'read' | 'write' | 'create' | 'delete' | 'admin'

export type PermissionContext = {
  /** Role trong phạm vi resource (vd ProjectMemberRole 'owner'|'member'|'viewer'). */
  resourceRole?: string
}

export class PermissionService {
  constructor(private readonly userStore: Pick<AuthUserStore, 'getUser'>) {}

  async getGlobalRole(userId: string): Promise<OrcaUser['role'] | null> {
    const user = await this.userStore.getUser(userId)
    return user?.role ?? null
  }

  /** Điểm quyết định RBAC duy nhất — mọi domain (Admin/Profile/Project/...) gọi qua đây. */
  async hasPermission(
    userId: string,
    resource: PermissionResource,
    action: PermissionAction,
    context?: PermissionContext
  ): Promise<boolean> {
    const globalRole = await this.getGlobalRole(userId)
    if (!globalRole) {return false}
    if (globalRole === 'admin') {return true} // global admin override mọi resource (F32)

    switch (resource) {
      case 'company':
      case 'department':
        // Company/dept mutation là admin-only bất kể resourceRole (F32/F33 security lock).
        return action === 'read'
      case 'project':
        if (action === 'read') {return true} // membership đã được assertAccess() check trước
        return context?.resourceRole === 'owner'
      default:
        return false
    }
  }
}
