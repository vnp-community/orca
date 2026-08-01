# TASK-AIP-001: Fix createAccount status — pending → active sau writeCredential

**Priority:** 🔴 CRITICAL  
**Effort:** ~15 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AIP-001  
**Solution ref:** [SOLUTION-ai-providers.md](../solutions/SOLUTION-ai-providers.md)

## File cần sửa
`src/main/ai-providers/AIProviderService.ts`

## Thay đổi
Trong `writeCredential()`, sau khi relay call thành công, update status → `active`:

```typescript
async writeCredential(accountId: string, devServerId: string, encryptedBlob: string): Promise<void> {
  const bridge = this.devServerManager.getBridge(devServerId)
  if (!bridge) throw new Error(`Dev server not found: ${devServerId}`)
  
  await bridge.call('aiProvider.writeCredential', { accountId, encryptedBlob })
  
  // FIX AIP-001: Update status pending → active after successful write
  await this.repository.update(accountId, { status: 'active', updatedAt: Date.now() })
}
```

Và trong `resolveForProject()`, nếu chỉ filter `status='active'`, bỏ `status='pending'` accounts là đúng. Đảm bảo flow `createAccount` → `writeCredential` → `status=active` hoạt động.

## Verification
```bash
pnpm tsc --noEmit
# Test: createAccount → status='pending' → writeCredential → status='active'
# Test: resolveForProject chỉ trả về accounts với status='active'
```
