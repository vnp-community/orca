# TASK-FE-025 — Integrate SSH User Indicator vào Sidebar

**Phase:** 4 — SSH UI
**Solution:** [SOL-FE-LG-004](../solutions/SOL-FE-LG-004-ssh-ui.md) §5.6
**Depends on:** TASK-FE-024
**Blocks:** —
**Effort:** M (~30 phút)
**Status:** ✅ Done

---

## Mô tả

Tích hợp `SshUserIndicator` vào component hiển thị SSH server trong Sidebar.
Mỗi server card sẽ hiển thị `orca-{name}` và trạng thái provisioning bên dưới connection status.

---

## File cần sửa

### `src/renderer/src/components/sidebar/SshServerCard.tsx` [MODIFY]

_(Hoặc component tương đương trong sidebar hiển thị mỗi SSH server — cần tìm đúng file trong codebase)_

**Thêm imports:**

```typescript
import { SshUserIndicator } from '../ssh/SshUserIndicator'
import { useSshUserAccount } from '../../hooks/useSshUserAccount'
import { useSshProvisioning } from '../../hooks/useSshProvisioning'
import { useAuthUser } from '../../hooks/useAuthSession'
import { useAppStore } from '../../store'
```

**Thêm vào component:**

```typescript
// Trong component function:
const authUser = useAuthUser()
const { linuxUsername, provisioned, previewUsername } = useSshUserAccount(
  server.id,
  { previewFromEmail: authUser?.email }
)
const provisioningStatus = useAppStore(
  s => s.sshUserAccounts.get(server.id)?.provisioningStatus ?? { phase: 'idle' as const }
)

// Subscribe provisioning events
useSshProvisioning(server.id)
```

**Thêm vào JSX render (sau existing server status):**

```typescript
{/* SSH User Identity — chỉ hiển thị trong web mode */}
{import.meta.env.ORCA_PLATFORM === 'web' && (
  <SshUserIndicator
    serverId={server.id}
    linuxUsername={linuxUsername ?? previewUsername ?? 'orca-?'}
    provisioned={provisioned}
    provisioningStatus={provisioningStatus}
  />
)}
```

---

## Tìm đúng component để sửa

Trước khi sửa, tìm component đang render SSH servers trong sidebar:

```bash
# Tìm component render SSH server list/card
grep -r "SshConnection\|sshConnections\|ssh-server\|SshServer" \
  src/renderer/src/components/sidebar/ --include="*.tsx" -l
```

---

## Constraints

- KHÔNG sửa App.tsx hoặc Sidebar.tsx chính
- Guard `ORCA_PLATFORM === 'web'` — SSH User Indicator không hiện trong Desktop mode
- `linuxUsername ?? previewUsername ?? 'orca-?'` — luôn có fallback

---

## Acceptance Criteria (Full Phase 4)

Sau khi hoàn thành TASK-FE-021..025:

- [ ] `toLinuxUsername("alice@company.com")` === `"orca-alice"` (unit test)
- [ ] Sidebar hiển thị `👤 orca-alice` bên dưới SSH server status
- [ ] Progress bar xuất hiện khi server đang provisioning
- [ ] ✅ icon khi provisioning done
- [ ] Error alert khi provisioning thất bại
- [ ] Không hiển thị trong Desktop mode

---

## Verify

```bash
npx tsc --noEmit
npx vitest run src/renderer/src/
# Manual: web mode + login → thấy orca-{name} dưới mỗi SSH server
```
