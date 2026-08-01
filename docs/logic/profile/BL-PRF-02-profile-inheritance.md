# BL-PRF-02 — Profile Inheritance Resolution

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PRF-02 |
| **Tên** | Profile Inheritance Resolution |
| **Domain** | Profile Management |
| **Actor** | System (auto) |
| **Priority** | P0 |

---

## Mô tả

Khi hệ thống cần profile của một user (để spawn agent, inject env, load settings), `ProfileResolver` tự động merge 3 tầng profile: Company → Department → User. Kết quả là một `ResolvedProfile` hoàn chỉnh.

---

## Algorithm

```typescript
async function resolveProfile(userId: string): Promise<ResolvedProfile> {
  // 1. Cache check (TTL = 60s)
  const cached = ProfileCache.get(userId)
  if (cached && !cached.isStale()) return cached.value

  // 2. Load 3 layers
  const user = await db.users.findById(userId)
  const dept = user.departmentId
    ? await db.departments.findById(user.departmentId)
    : null
  const company = await db.company.findFirst()

  // 3. Parse JSON (fallback to {})
  const companyProfile: OrcaProfile = safeParseJson(company?.profileJson) ?? {}
  const deptProfile:    OrcaProfile = safeParseJson(dept?.profileJson) ?? {}
  const userProfile:    OrcaProfile = safeParseJson(user.profileJson) ?? {}

  // 4. Deep merge: company ← dept ← user
  const resolved = deepMergeProfiles(companyProfile, deptProfile, userProfile)

  // 5. Apply security lock (company security cannot be overridden)
  resolved.security = companyProfile.security ?? {}

  // 6. Merge array fields (union, not replace)
  resolved.shell = {
    ...resolved.shell,
    pathAdditions: [
      ...(companyProfile.shell?.pathAdditions ?? []),
      ...(deptProfile.shell?.pathAdditions ?? []),
      ...(userProfile.shell?.pathAdditions ?? []),
    ],
    envVars: {
      ...(companyProfile.shell?.envVars ?? {}),
      ...(deptProfile.shell?.envVars ?? {}),   // dept overrides company
      ...(userProfile.shell?.envVars ?? {}),   // user overrides dept
    }
  }

  // 7. Validate against approvedModels
  if (company.approvedModels?.length > 0) {
    const preferred = resolved.agent?.preferredModel
    if (preferred && !company.approvedModels.includes(preferred)) {
      resolved.agent.preferredModel = company.approvedModels[0]
      resolved.agent._modelFallbackReason = `'${preferred}' not in approved list`
    }
  }

  // 8. Cache result
  ProfileCache.set(userId, resolved, TTL_SECONDS = 60)

  return resolved
}
```

---

## Merge Rules chi tiết

| Field type | Merge strategy |
|------------|---------------|
| Scalar (string, number, bool) | Last non-null wins: user > dept > company |
| Object | Recursive deep merge |
| Array (pathAdditions) | **Concatenate** (company + dept + user) — no dedup |
| Map (envVars) | **Override merge**: user keys override dept, dept override company |
| `security.*` | **Company only** — dept/user values discarded |
| `agent.approvedModels` | **Company only** — user/dept cannot expand list |
| `fleet.allowedServerTags` | **Intersect**: user subset ⊆ dept ⊆ company |

---

## Cache Invalidation

| Event | Invalidate |
|-------|------------|
| Company profile updated | ALL user caches |
| Department profile updated | All users in that dept |
| User profile updated | That user only |
| User changes department | That user only |

```typescript
// Cache key: `profile:${userId}`
// On company update: flush all profile:* keys
// On dept update: flush profile:* for all dept members
// Implementation: Redis or in-memory Map with TTL
```

---

## Source Attribution

Resolved profile giữ metadata về nguồn gốc mỗi field (cho UI):

```typescript
interface ResolvedProfileWithMeta extends ResolvedProfile {
  _sources: {
    [fieldPath: string]: 'company' | 'department' | 'user'
  }
}

// Example:
// { 'agent.preferredModel': 'department', 'editor.theme': 'user' }
```

UI dùng `_sources` để hiển thị "Inherited from Department" vs "✏️ Personal setting".

---

## Tiêu chí chấp nhận

- [ ] `resolveProfile()` merge đúng 3 tầng
- [ ] Security fields luôn từ company (không bị override)
- [ ] `pathAdditions` được concatenate (không replace)
- [ ] `envVars` user keys override dept keys
- [ ] `approvedModels` validation với fallback
- [ ] Cache hit < 1ms; cache miss < 10ms
- [ ] Cache invalidation đúng theo scope
- [ ] `_sources` metadata tracking đúng
