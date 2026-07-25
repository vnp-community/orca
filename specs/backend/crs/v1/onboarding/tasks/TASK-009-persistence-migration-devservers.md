# TASK-009: Sửa `src/main/persistence.ts` — Migration `devServers: []`

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §9  
**Depends on:** TASK-002  
**Blocks:** (không)

---

## Mục tiêu

Thêm migration trong `normalizeLoadedState()` (hoặc hàm tương đương) để đảm bảo `state.devServers` luôn tồn tại sau khi load từ disk, kể cả với state cũ chưa có field này.

---

## File cần sửa

**Path:** `src/main/persistence.ts`

---

## Thay đổi cần thực hiện

### Tìm hàm normalize/migrate state

Tìm hàm như `normalizeLoadedState()`, `migrateState()`, hoặc tương đương trong file. Thêm migration cho `devServers`:

```typescript
function migrateDevServers(state: PersistedState): PersistedState {
  // v0 → v1: nếu chưa có devServers, khởi tạo rỗng
  // (user phải add thủ công — không tự tạo)
  if (!state.devServers) {
    state.devServers = []
  }
  return state
}
```

Gọi migration này trong pipeline normalize:

```typescript
function normalizeLoadedState(raw: unknown): PersistedState {
  // ...existing migrations...
  state = migrateDevServers(state)   // NEW — thêm vào cuối hoặc đúng thứ tự
  return state
}
```

---

## Acceptance Criteria

- [x] State cũ (không có `devServers`) được normalize thành `devServers: []`
- [x] State mới (đã có `devServers`) không bị overwrite
- [x] Migration không ảnh hưởng đến các fields khác của state
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Đọc file `src/main/persistence.ts` để hiểu pattern migration hiện tại
2. Đặt migration theo đúng thứ tự với các migration khác (thường: từ cũ → mới)
3. Đừng dùng `JSON.parse` trực tiếp — theo pattern đã có trong file
