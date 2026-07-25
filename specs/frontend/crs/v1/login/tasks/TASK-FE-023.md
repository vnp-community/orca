# TASK-FE-023 — Tạo `SshProvisioningProgress.tsx` + `SshUserIndicator.tsx` + Tests

**Phase:** 4 — SSH UI
**Solution:** [SOL-FE-LG-004](../solutions/SOL-FE-LG-004-ssh-ui.md) §5.3, §5.4, §4.1, §4.2
**Depends on:** TASK-FE-022
**Blocks:** TASK-FE-024, TASK-FE-025
**Effort:** M (~35 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo 2 UI components cho SSH user identity và provisioning status:
- `SshProvisioningProgress`: progress bar với step description
- `SshUserIndicator`: container hiển thị username + trạng thái provisioning

---

## Files cần tạo

### `src/renderer/src/components/ssh/SshProvisioningProgress.tsx` [NEW]

```typescript
type Props = { step: string; progress: number }  // progress: 0–100

export function SshProvisioningProgress({ step, progress }: Props) {
  return (
    <div className="ssh-provisioning-progress">
      <p className="ssh-provisioning-progress__step">{step}</p>
      <div
        role="progressbar"
        aria-valuenow={progress}
        aria-valuemin={0}
        aria-valuemax={100}
        className="ssh-provisioning-progress__bar"
      >
        <div
          className="ssh-provisioning-progress__fill"
          style={{ width: `${progress}%` }}
        />
      </div>
    </div>
  )
}
```

### `src/renderer/src/components/ssh/SshUserIndicator.tsx` [NEW]

Implement theo spec đầy đủ tại [SOL-FE-LG-004 §5.3](../solutions/SOL-FE-LG-004-ssh-ui.md).

Behavior theo `provisioningStatus.phase`:
- `idle`: Hiển thị username (predicted), không có progress bar
- `checking`: Hiển thị spinner hoặc "Checking..."
- `provisioning`: Hiển thị username + `<SshProvisioningProgress>`
- `done`: Hiển thị username + ✅ icon
- `error`: Hiển thị `<div role="alert">` với error message

### `src/renderer/src/components/ssh/__tests__/SshUserIndicator.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-004 §4.1](../solutions/SOL-FE-LG-004-ssh-ui.md).

Test cases (4 tests):
- Renders username khi provisioned (done)
- Shows progressbar khi provisioning
- Error alert khi phase=error
- No progressbar khi idle

### `src/renderer/src/components/ssh/__tests__/SshProvisioningProgress.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-004 §4.2](../solutions/SOL-FE-LG-004-ssh-ui.md).

Test cases (3 tests):
- Progressbar aria-valuenow = progress value
- Renders step description text
- 100% complete state

---

## Verify

```bash
npx vitest run src/renderer/src/components/ssh/__tests__/
# Expected: 7 pass (4 + 3)
```
