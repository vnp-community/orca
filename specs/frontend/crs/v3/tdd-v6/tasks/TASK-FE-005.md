# TASK-FE-005: Verify & Fix ProfileEditor Security Locking

**Task ID:** TASK-FE-005
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-001 (Section 2.2)
**Estimated effort:** 30 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Objective

Read and verify `ProfileEditor.tsx` implements the 3 required behaviors from TDD-FE-11. Fix any gaps found:
1. Security section is locked (read-only) when `scope !== 'company'`
2. "Effective Settings" tab only appears when `scope === 'user'`
3. `ModelSelector` uses `resolvedProfile?.security?.approvedModels` for filtering

---

## Execution Results

### Gap analysis

**Gap A — Security section locking:**
- Line 71: `{scope !== 'company' && (...locked message...)}` — ✅ ALREADY CORRECT
- Renders "Managed by company admin" for non-company scope

**Gap B — "Effective Settings" tab:**
- Line 42: `{scope === 'user' && (<Tabs...>)}` — ✅ ALREADY CORRECT
- Tab only shown for user scope

**Gap C — ModelSelector approvedModels:**
- `ModelSelector` was reading `approvedModels` directly from store (worked but not testable)
- ❌ NOT passing `approvedModels` as explicit prop from `resolvedProfile`
- **FIXED**: Added `approvedModels?: string[]` prop to `ModelSelector`, with fallback to store

### Changes made

**`profile-types.ts`** — Added missing sub-objects to `OrcaProfile`:
- `agent.approvedModels?: string[]` — user-level model override
- `editor?: { theme, fontSize, fontFamily, keybindings }` — editor preferences
- `shell.startupCommands?: string[]` — shell startup scripts
- `integrations?: { githubOrg, linearWorkspace, prTemplate }` — 3rd party integrations
- `fleet?: { allowedServerTags, defaultConnectionType }` — dev server fleet
- `security.require2FA?: boolean` and `sessionTimeoutHours?: number`
- `Department.leadId?: string` and `memberCount?: number`

**`ModelSelector.tsx`** — Added `approvedModels?: string[]` prop:
- If prop provided → use prop value
- Otherwise → fall back to `useAppStore(s => s.resolvedProfile?.security?.approvedModels)`

**`ProfileEditor.tsx`** — Pass `resolvedProfile?.security?.approvedModels` to `ModelSelector`:
```tsx
<ModelSelector
  approvedModels={resolvedProfile?.security?.approvedModels}
  ...
/>
```

---

## Acceptance Criteria

- [x] `ProfileEditor` accepts `scope: 'user' | 'dept' | 'company'` prop
- [x] Security section is locked (read-only content) when `scope !== 'company'`
- [x] "Effective Settings" tab only shows when `scope === 'user'`
- [x] `ModelSelector` receives `approvedModels` from resolved profile as explicit prop
- [x] `profile-types.ts` contains `OrcaProfile`, `ResolvedProfile`, `Department` with all fields
- [x] No TypeScript errors (task scope)

---

## Output

```
Gap A (security locking): ALREADY CORRECT
Gap B (effective tab): ALREADY CORRECT
Gap C (model filter via prop): FIXED — added approvedModels prop to ModelSelector
profile-types.ts additions: editor, integrations, fleet, agent.approvedModels, security.require2FA, security.sessionTimeoutHours, Department.leadId, Department.memberCount
TypeScript errors (task scope): 0
```
