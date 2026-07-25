# TASK-001: Extend `SshTarget` type với fleet metadata

**Source:** SOL-001  
**Phase:** 1 | **Effort:** XS (<30 min)  
**Depends on:** —

---

## Objective

Thêm 6 optional fields vào `SshTarget` type trong `src/shared/ssh-types.ts` để support fleet management metadata.

---

## File to modify

**`src/shared/ssh-types.ts`**

---

## Implementation

Locate type `SshTarget` (hoặc `interface SshTarget`) trong file. Thêm các fields sau vào cuối của type definition, **trước dấu đóng `}`**:

```typescript
  // ── Fleet Management Fields (NEW) ─────────────────────────
  /** Project this server belongs to. e.g. "vnp-blc", "vnp-ai-ops", "vnp-claw" */
  project?: string
  /** Team owning this server. e.g. "backend", "frontend", "ai-platform" */
  team?: string
  /** Deployment environment */
  environment?: 'development' | 'staging' | 'production'
  /** Free-form tags for flexible grouping */
  tags?: string[]
  /** Repos available on this server */
  repos?: Array<{
    path: string    // e.g. /srv/projects/vnp-blc
    name: string    // e.g. vnp-blc
    url?: string    // git remote URL (optional)
    branch?: string // default branch
  }>
  /** Path to the fleet config file that imported this target */
  fleetConfigSource?: string
  /** Stable ID from fleet config (used to detect re-imports and avoid duplicates) */
  fleetId?: string
```

---

## Verification

```bash
# TypeScript compile check
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "ssh-types|SshTarget" | head -20
```

Expected: no new errors related to `SshTarget`.

---

## Done criteria

- [x] `SshTarget` type có đủ 7 optional fields mới
- [x] Không có TypeScript compile error
- [x] Existing code không bị break (all fields optional)

**Status: ✅ DONE** — `src/shared/ssh-types.ts` updated. Fields: `project`, `team`, `environment`, `tags`, `repos`, `fleetConfigSource`, `fleetId`.
