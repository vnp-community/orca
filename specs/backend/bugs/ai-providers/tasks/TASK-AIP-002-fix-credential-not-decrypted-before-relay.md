# TASK-AIP-002: Fix credential relay — decrypt Layer 1 trước khi gửi relay

**Priority:** 🔴 CRITICAL SECURITY  
**Effort:** ~30 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AIP-002, BUG-BE-AIP-002  
**Solution ref:** [SOLUTION-ai-providers.md](../solutions/SOLUTION-ai-providers.md)

## Mô tả

Credential được Browser encrypt 2 lớp:
1. Layer 1: SubtleCrypto (browser-side) → decrypt tại Orca Server
2. Layer 2: AES-GCM (server-side) → giữ nguyên khi gửi relay, decrypt tại Dev Server

Hiện tại, `writeCredential()` có thể đang gửi cả 2 layers chưa decrypt tới relay → Dev Server nhận được double-encrypted blob không thể dùng.

## File cần sửa
`src/main/ai-providers/AIProviderService.ts`

## Thay đổi

```typescript
async writeCredential(
  accountId:     string,
  devServerId:   string,
  encryptedBlob: string,  // Layer 1 (SubtleCrypto) + Layer 2 (AES-GCM)
  subtleCryptoKey: string // Key để decrypt Layer 1
): Promise<void> {
  // Step 1: Decrypt Layer 1 (SubtleCrypto) on Orca Server
  const layer2Blob = await decryptLayer1(encryptedBlob, subtleCryptoKey)
  
  // Step 2: Send Layer 2 blob to Dev Server via relay
  const bridge = this.devServerManager.getBridge(devServerId)
  if (!bridge) throw new Error(`Dev server not found: ${devServerId}`)
  
  await bridge.call('aiProvider.writeCredential', {
    accountId,
    encryptedBlob: layer2Blob  // Only Layer 2 — Dev Server can decrypt this
  })
}
```

## Verification
```bash
pnpm tsc --noEmit
# Test: agent trên Dev Server có thể đọc và dùng credential sau writeCredential
```
