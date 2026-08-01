# ADR-007 — 3-Layer Profile Hierarchy with Deep-Merge Strategy

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-007 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C3.10, C4.7 |
| **Code Ref** | `src/main/orca-profiles/` (hiện là cloud-auth profiles), cần tạo mới |
| **Feature Ref** | F33 |

---

## Bối cảnh

Trong Web mode, mỗi developer thuộc về Company và/hoặc Department. Tất cả cần cấu hình AI agent (model, trust preset, env vars, MCP servers...). Nếu admin phải cấu hình từng người → không scale (100+ developers).

**Yêu cầu:**
- Company admin set baseline (security, approved models)
- Team lead override cho team
- Individual developer override cho cá nhân
- Conflict resolution rõ ràng (ai thắng ai?)
- Security-critical fields không được override xuống tầng thấp hơn

---

## Quyết định

### OrcaProfile Schema

```typescript
interface OrcaProfile {
  agent: {
    preferredModel: string          // e.g. 'claude-opus-4-5'
    trustPreset: 'minimal' | 'standard' | 'full'
    mcpServers: McpServerConfig[]   // MCP tool servers
    customInstructions: string
  }
  editor: {
    defaultEditor: string           // 'vscode' | 'cursor' | 'vim' | ...
    tabSize: number
    theme: string
  }
  shell: {
    defaultShell: string            // '/bin/bash' | '/bin/zsh'
    pathAdditions: string[]         // PATH prepends
    envVars: Record<string, string>
  }
  security: {
    approvedModels: string[]        // LOCKED at company level
    disallowedCommands: string[]    // LOCKED at company level
    requireReviewBeforeCommit: boolean
  }
  mcp: {
    servers: McpServerConfig[]
  }
}
```

### Deep-Merge Algorithm

```
resolvedProfile = deepMerge(companyProfile, deptProfile, userProfile)
```

**Rules per field type:**

| Field type | Merge strategy | Ví dụ |
|-----------|---------------|-------|
| Scalar (`preferredModel`) | Override (User > Dept > Company) | User wins |
| `envVars` object | Key-level override merge (User > Dept > Company) | User key wins |
| `pathAdditions` array | Concatenation (all appended) | Company + Dept + User |
| `mcpServers` array | Union by server name (User overrides same name) | Merged |
| `security.*` | LOCKED — Company only, children cannot override | Company wins always |

### Lock mechanism

```typescript
interface ProfileMergeOptions {
  lockedSections: Array<'security'>  // Company declares these locked
}

function deepMerge(
  company: OrcaProfile,
  dept: Partial<OrcaProfile>,
  user: Partial<OrcaProfile>,
  options: ProfileMergeOptions
): ResolvedProfile {
  const locked = options.lockedSections
  // For locked sections: always use company value
  // For others: user overrides dept overrides company
}
```

### Cache & Invalidation

```typescript
class ProfileResolver {
  private cache = new Map<string, { resolved: ResolvedProfile; expiresAt: number }>()

  resolve(userId: string): ResolvedProfile {
    const cached = this.cache.get(userId)
    if (cached && Date.now() < cached.expiresAt) return cached.resolved
    const resolved = this.computeResolvedProfile(userId)
    this.cache.set(userId, { resolved, expiresAt: Date.now() + TTL_MS })
    return resolved
  }

  invalidate(companyId?: string, deptId?: string, userId?: string): void {
    // Cascade invalidate: company change → invalidate all users
  }
}

const TTL_MS = 60_000  // 60 seconds
```

### Source Attribution

```typescript
interface ResolvedProfile extends OrcaProfile {
  _sources: {
    [fieldPath: string]: 'company' | 'dept' | 'user'
  }
}
// UI shows source badge on each field
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **3-layer deep-merge + lock** ✅ | Giống Git config (system/global/local) — familiar pattern |
| Per-user full config copy | Không kế thừa; admin phải update 100+ copies khi thay đổi |
| Inheritance chain (OOP) | Khó visualize; circular dep risk |
| RBAC policy engine (OPA) | Overkill cho config; performance overhead |

---

## Liên hệ codebase hiện tại

`src/main/orca-profiles/` hiện là cloud-based profile auth (không liên quan). Cần tạo mới:
- `src/main/profile/OrcaProfile.ts` — schema + types
- `src/main/profile/ProfileResolver.ts` — deep-merge + cache
- `src/main/profile/ProfileService.ts` — CRUD company/dept/user profiles
- DB: migration 0006 (`orca_company`, `orca_departments`, `orca_users.profile_json`)

---

## Trạng thái Implementation

❌ Chưa implement (v5.0 proposed)  
🎯 Migration 0006 cần được tạo trước  
🎯 ProfileResolver service  
🎯 RPC methods: profile.get, profile.update, profile.resolve
