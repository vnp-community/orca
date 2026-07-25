# TASK-014: Fix duplicate members trong `src/main/mocks/electron.ts`

**Source:** SOL-BE-004 (CR-007 Mock Cleanup)  
**Phase:** 2 | **Effort:** S (30–45 min)  
**Depends on:** — (independent, có thể làm song song với TASK-009)

---

## Objective

Fix các TypeScript errors do duplicate class members trong `src/main/mocks/electron.ts`:
- Remove duplicate method/arrow function pairs trong `BrowserWindow`
- Di chuyển `mockSessionObject` lên trước khi được tham chiếu
- Thêm `isFocused()` nếu chưa có
- Thêm `@deprecated` JSDoc để hướng dẫn dùng `electron-node-wrapper` thay thế

---

## Context cần đọc trước

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
cat src/main/mocks/electron.ts
# Identify all duplicate member pairs
npx tsc --noEmit 2>&1 | grep "mocks/electron" | head -20
```

---

## File to modify

**`src/main/mocks/electron.ts`**

### Rules để fix:

**1. Tìm và xóa duplicate definitions:**

Pattern cần tìm và xóa (giữ lại method, xóa arrow):
```typescript
// GIỮ LẠI (method syntax):
isMaximized() { return false }

// XÓA (arrow syntax — duplicate):
isMaximized = () => false
```

Các methods cần kiểm tra duplicates:
- `isMaximized` / `isMinimized` / `isFullScreen` / `isVisible`
- `restore` / `focus` / `close` / `destroy`
- `getBounds` / `getSize` / `setSize` / `getContentSize`
- `loadURL` / `loadFile` / `show` / `hide`

**2. Move `mockSessionObject` lên trước `BrowserWindow` class:**

Nếu `webContents.session` reference một object được define sau `BrowserWindow`, di chuyển nó lên.

**3. Thêm `isFocused()` nếu chưa có:**

```typescript
// Thêm vào BrowserWindow class nếu chưa có:
isFocused() { return true }
```

**4. Thêm `@deprecated` comment:**

```typescript
/**
 * @deprecated Use src/platform/stubs/electron-node-wrapper.ts for server mode.
 * This file is kept for legacy compatibility and Electron mode testing only.
 */
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# Check no duplicate member errors
npx tsc --noEmit 2>&1 | grep "mocks/electron" | head -10
# Expected: empty (no errors)

# Run mock-specific tests
npx vitest run src/main/mocks/ 2>/dev/null || echo "No mock tests yet"

# Verify required methods exist
node -e "
const mock = require('./src/main/mocks/electron')
const win = new mock.BrowserWindow()
const required = ['isMaximized','isMinimized','isFullScreen','isVisible',
  'isFocused','isDestroyed','show','hide','focus','restore','close','destroy',
  'getBounds','loadURL','send','on','once','off']
for (const m of required) {
  if (typeof win[m] !== 'function')
    console.error('MISSING method:', m)
  else
    console.log('OK:', m)
}"
```

---

## Test file (add after fixing)

### `src/main/mocks/__tests__/electron-mock.test.ts`

```typescript
import { describe, it, expect } from 'vitest'
import { BrowserWindow, session, app, ipcMain } from '../electron'

describe('Electron mock — no duplicate members', () => {
  it('BrowserWindow can be instantiated', () => {
    expect(() => new BrowserWindow()).not.toThrow()
  })

  it('BrowserWindow has all state methods', () => {
    const win = new BrowserWindow()
    const methods = [
      'isMaximized', 'isMinimized', 'isFullScreen',
      'isVisible', 'isFocused', 'isDestroyed'
    ]
    for (const m of methods) {
      expect(typeof win[m]).toBe('function')
    }
  })

  it('All state methods return boolean', () => {
    const win = new BrowserWindow()
    expect(typeof win.isMaximized()).toBe('boolean')
    expect(typeof win.isMinimized()).toBe('boolean')
    expect(typeof win.isFullScreen()).toBe('boolean')
    expect(typeof win.isVisible()).toBe('boolean')
    expect(typeof win.isFocused()).toBe('boolean')
    expect(typeof win.isDestroyed()).toBe('boolean')
  })

  it('webContents.session is defined', () => {
    const win = new BrowserWindow()
    expect(win.webContents).toBeDefined()
    expect(win.webContents.session).toBeDefined()
  })

  it('webContents.session.fromPartition returns session-like object', () => {
    const win = new BrowserWindow()
    const s = win.webContents.session.fromPartition('persist:test')
    expect(s).toBeDefined()
    expect(typeof s.getUserAgent).toBe('function')
  })

  it('session.fromPartition works at module level', () => {
    const s = session.fromPartition('persist:test')
    expect(s).toBeDefined()
  })

  it('app.getPath returns string', () => {
    expect(typeof app.getPath('userData')).toBe('string')
  })

  it('ipcMain.handle does not throw', () => {
    expect(() =>
      ipcMain.handle('test:channel', async () => 'ok')
    ).not.toThrow()
    ipcMain.removeHandler('test:channel')
  })

  it('BrowserWindow destroy() works', () => {
    const win = new BrowserWindow()
    expect(() => win.destroy()).not.toThrow()
  })

  it('BrowserWindow getAllWindows() returns array', () => {
    expect(Array.isArray(BrowserWindow.getAllWindows())).toBe(true)
  })
})
```

---

## Done criteria

- [x] `npx tsc --noEmit` không còn lỗi `Duplicate identifier` từ `mocks/electron.ts`
- [x] `isFocused()` method tồn tại và trả về `boolean`
- [x] `webContents.session.fromPartition()` hoạt động
- [x] `@deprecated` comment được thêm vào đầu file
- [x] 10+ tests trong `electron-mock.test.ts` pass
- [x] Electron desktop mode vẫn hoạt động (không break `electron-vite build`)
