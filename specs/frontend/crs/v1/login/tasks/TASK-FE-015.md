# TASK-FE-015 — Tạo `useAdminStats.ts` + `AdminDashboard.tsx` + Tests

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md) §4.4, §4.5, §3.2
**Depends on:** TASK-FE-013, TASK-FE-014
**Blocks:** TASK-FE-020
**Effort:** M (~40 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo dashboard page với:
- Stat cards (Users, Active Sessions, SSH, Devices)
- Active sessions table
- Auto-refresh stats mỗi 30 giây

---

## Files cần tạo

### `src/renderer/src/hooks/useAdminStats.ts` [NEW]

```typescript
// Poll /admin/api/stats mỗi 30 giây
// Return: { stats: AdminStats | null, isLoading, error, refresh }
const POLL_INTERVAL_MS = 30_000

export function useAdminStats() {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchAdminStats()
      setStats(data)
      setError(null)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
    const timer = setInterval(refresh, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  return { stats, isLoading, error, refresh }
}
```

### `src/renderer/src/components/admin/AdminDashboard.tsx` [NEW]

Implement theo spec tại [SOL-FE-LG-003 §4.5](../solutions/SOL-FE-LG-003-admin-panel.md).

- 4 StatCard components với: Users, Active, SSH Conn., Devices
- Loading state: `<div role="status" aria-label="Loading">Loading...</div>`
- Table hiển thị active sessions với: user email, IP, started, last seen

### `src/renderer/src/components/admin/__tests__/AdminDashboard.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-003 §3.2](../solutions/SOL-FE-LG-003-admin-panel.md).

Test cases (3 tests):
- Renders stat cards với đúng số liệu sau load
- Renders active sessions table với user email
- Loading state trước khi data

---

## Verify

```bash
npx vitest run src/renderer/src/components/admin/__tests__/AdminDashboard.test.tsx
# Expected: 3 pass
```
