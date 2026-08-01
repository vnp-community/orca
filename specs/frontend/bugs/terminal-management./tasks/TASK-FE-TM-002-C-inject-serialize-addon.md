# TASK-FE-TM-002-C: TerminalPane — inject `SerializeAddon` vào transport (TM-002)

**Domain:** terminal-management.  
**Solution Ref:** SOL-FE-TM-002 Phần 3  
**Bug:** BUG-FE-TM-002  
**Priority:** 🟠 P1  
**Estimated:** 20 phút  
**Status:** ✅ DONE — Implemented via serializeBuffer in disconnect()

---

## Mục tiêu

Trong `TerminalPane.tsx`, tạo `SerializeAddon` instance, load vào xterm terminal, và pass vào transport factory.

---

## Files cần sửa

- `src/renderer/src/components/terminal-pane/TerminalPane.tsx`

---

## Các bước thực thi

### Bước 1: Tạo SerializeAddon

Trong phần setup xterm terminal (trong `useEffect` hoặc setup function):

```typescript
import { SerializeAddon } from '@xterm/addon-serialize'

// Sau khi tạo terminal instance:
const serializeAddon = new SerializeAddon()
term.loadAddon(serializeAddon)
```

### Bước 2: Pass vào transport factory

Khi tạo `RemoteRuntimePtyTransport` instance:

```typescript
const transport = createRemoteRuntimePtyTransport({
  worktreeId,
  // ... existing options ...
  serializeAddon,    // ← thêm dòng này
})
```

### Bước 3: Verify xterm-addon-serialize đã install

```bash
grep "@xterm/addon-serialize" package.json
# Nếu chưa có:
# npm install @xterm/addon-serialize
```

---

## Verify

```bash
grep -n "SerializeAddon\|addon-serialize" \
  src/renderer/src/components/terminal-pane/TerminalPane.tsx
```

## Depends on
TASK-FE-TM-002-A (transport accepts serializeAddon option)
