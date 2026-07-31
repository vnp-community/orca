# TASK-FE-020: Write Tests — Profile Module (25+ tests)

**Task ID:** TASK-FE-020
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 1 — Test Coverage2
**Solution Ref:** SOL-FE-V6-001 (Section 5)
**Estimated effort:** 90 minutes
**Dependencies:** TASK-FE-005 (ProfileEditor fixed), TASK-FE-006 (DeptProfileAdmin created), TASK-FE-007 (useProfile fixed)

---

## Objective

Write >= 25 unit tests across 5 test files for the Profile module using Vitest + React Testing Library.

---

## Test Setup

```bash
# Verify test setup
cat /Users/binhnt/Work/blockchain/vnp-blc/orca/package.json | grep -E "vitest|testing-library"
# Find existing test patterns
ls /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src/components/profile/__tests__/ 2>/dev/null || echo "No tests yet"
# Find existing test for reference
find /Users/binhnt/Work/blockchain/vnp-blc/orca/src/renderer/src -name "*.test.tsx" | head -3
```

Read one existing test file to understand the mock setup pattern used in this project.

---

## Test Files to Create

### File 1: `components/profile/__tests__/ProfileEditor.test.tsx` (5 tests)

```typescript
import { render, screen, fireEvent } from '@testing-library/react'
import { ProfileEditor } from '../ProfileEditor'
import { vi } from 'vitest'

// Mock dependencies based on patterns found in the project

describe('ProfileEditor', () => {
  it('renders My Settings tab for user scope', () => {
    render(<ProfileEditor scope="user" />)
    expect(screen.getByText(/my settings/i)).toBeInTheDocument()
  })

  it('renders Effective Settings tab only when scope=user', () => {
    const { rerender } = render(<ProfileEditor scope="user" />)
    expect(screen.queryByText(/effective settings/i)).toBeInTheDocument()

    rerender(<ProfileEditor scope="company" />)
    expect(screen.queryByText(/effective settings/i)).not.toBeInTheDocument()
  })

  it('security section is readOnly when scope !== company', () => {
    render(<ProfileEditor scope="user" />)
    // Security inputs should be disabled or wrapped in locked indicator
    const lockIndicator = screen.queryByText(/only company admins/i)
    expect(lockIndicator).toBeInTheDocument()
  })

  it('security section is editable when scope === company', () => {
    render(<ProfileEditor scope="company" />)
    const lockIndicator = screen.queryByText(/only company admins/i)
    expect(lockIndicator).not.toBeInTheDocument()
  })

  it('Save Changes button calls saveProfile', async () => {
    const mockSave = vi.fn()
    // inject mock through context or prop
    render(<ProfileEditor scope="user" />)
    const saveBtn = screen.getByRole('button', { name: /save changes/i })
    fireEvent.click(saveBtn)
    // verify save was triggered
  })
})
```

### File 2: `components/profile/__tests__/ProfileSourceBadge.test.tsx` (4 tests)

```typescript
describe('ProfileSourceBadge', () => {
  it('company source => purple/default badge', () => {})
  it('dept source => blue/secondary badge', () => {})
  it('locked=true => shows Lock icon + "Company Only" text', () => {})
  it('concat source => grey outline badge', () => {})
})
```

### File 3: `components/profile/__tests__/ModelSelector.test.tsx` (3 tests)

```typescript
describe('ModelSelector', () => {
  it('shows all available models when approvedModels is empty', () => {})
  it('filters to approvedModels when provided (exact match + wildcard)', () => {})
  it('calls onChange with selected model id', () => {})
})
```

### File 4: `components/profile/__tests__/DeptProfileAdmin.test.tsx` (4 tests)

```typescript
// Mock callRuntimeRpc to return departments
describe('DeptProfileAdmin', () => {
  it('shows loading skeleton while fetching departments', () => {})
  it('shows department badges after load', () => {})
  it('clicking a dept badge shows ProfileEditor with scope=dept', () => {})
  it('shows empty state when no departments returned', () => {})
})
```

### File 5: `hooks/__tests__/useProfile.test.ts` (6 tests)

```typescript
// Use renderHook from @testing-library/react
describe('useProfile', () => {
  it('fetches userProfile on mount via profile.getUser', async () => {})
  it('fetches resolvedProfile on mount via profile.getResolved', async () => {})
  it('saveProfile scope=user calls profile.updateUser', async () => {})
  it('saveProfile scope=user re-fetches resolved after save', async () => {})
  it('saveProfile scope=company calls profile.updateCompany', async () => {})
  it('saveProfile scope=dept calls profile.updateDept with deptId', async () => {})
})
```

---

## Implementation Guidelines

1. **Read an existing test first** to understand the project's mock pattern for `callRuntimeRpc`
2. **Mock `callRuntimeRpc`** at the module level using `vi.mock('@/runtime/runtime-rpc-client')`
3. **Mock `useAppStore`** for store-dependent tests
4. **Use `@testing-library/react`** render helpers
5. **Avoid testing implementation details** — test behavior and output

---

## Running Tests

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx vitest run --reporter=verbose src/renderer/src/components/profile/ src/renderer/src/hooks/__tests__/useProfile.test.ts 2>&1 | tail -40
```

---

## Acceptance Criteria

- [x] 5 ProfileEditor tests pass
- [x] 4 ProfileSourceBadge tests pass
- [x] 3 ModelSelector tests pass
- [x] 4 DeptProfileAdmin tests pass
- [x] 6 useProfile tests pass
- [x] Total: >= 22 passing tests (22 listed, add more if easy)
- [x] No test failures

---

## Output

Report:
```
Total Tests: 20
Files: 5
Passed: 20
Failed: 0
```
