# BUG-AIP-001: `createAccount` trả về `status='pending'` nhưng `resolveForProject` chỉ filter `status='active'`

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AIP-001  
**Implementation:** AIProviderService.ts: updateAccount(active) after writeCredential  

## Mức độ: 🔴 CRITICAL

## Tóm tắt

`src/main/ai-providers/AIProviderService.ts:114`:
```typescript
// createAccount():
status: 'pending',  ← initial status khi tạo account
```

`src/main/ai-providers/AIProviderService.ts:322`:
```typescript
// resolveForProject():
const active = all.filter(a => a.status === 'active')  ← chỉ filter 'active'
```

**Mọi account mới tạo sẽ có status='pending' và KHÔNG BAO GIỜ được resolve để dùng.**

Khi nào status được set thành 'active'?
- `ProviderHealthChecker` tại `src/main/ai-providers/ProviderHealthChecker.ts:66`:
  ```typescript
  if (result.ok) { newStatus = 'active' }
  ```
- → HealthChecker chạy mỗi 15 phút, check xem `testConnection()` có pass không
- → Sau 15 phút HealthCheck pass → status = 'active'

**Vấn đề**: Account mới được tạo sẽ không hoạt động trong **tối thiểu 15 phút**.

## Ảnh hưởng

1. Admin đăng ký provider account → ngay lập tức thử spawn agent → FAIL "NoProviderAvailable"
2. Không có feedback rõ ràng ("pending" không show trong UI → user không biết phải chờ)
3. Nếu Dev Server chưa kết nối vào Orca → `testConnection` fail → status không bao giờ = 'active'

## Fix đề xuất

Option 1 — Tự động test ngay sau khi create:
```typescript
async createAccount(params): Promise<AIProviderAccount> {
  // ... INSERT ...
  // Trigger immediate health check
  void this.testConnection(id).then(result => {
    if (result.ok) this.updateAccount(id, { status: 'active' })
    else this.updateAccount(id, { status: 'invalid' })
  })
  return account
}
```

Option 2 — Thêm 'pending' vào resolveForProject filter:
```typescript
const active = all.filter(a => a.status === 'active' || a.status === 'pending')
```
(nhưng rủi ro dùng account chưa được verify)

## Files liên quan

- `src/main/ai-providers/AIProviderService.ts:114`: status='pending' initial
- `src/main/ai-providers/AIProviderService.ts:322`: filter active only
- `src/main/ai-providers/ProviderHealthChecker.ts:66`: set active only after health check
