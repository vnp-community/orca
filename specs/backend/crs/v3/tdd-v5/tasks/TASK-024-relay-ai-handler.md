# TASK-024: Relay AI Provider Handler

**Phase:** 4 — AI Provider Management  
**Solution ref:** [SOL-V5-003](../solutions/SOL-V5-003-ai-provider.md) §7  
**Prerequisite:** None (runs on Dev Server, not Orca Server)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/relay/ai-provider-handler.ts`

**QUAN TRỌNG:** File này chạy trên **Dev Server** qua relay binary, KHÔNG phải Orca Server. Không import bất kỳ module nào từ `src/main/`.

```typescript
import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto'
import { writeFile, readFile, mkdir } from 'node:fs/promises'
import { join } from 'node:path'
import { homedir } from 'node:os'

const PROVIDER_STORE_DIR = join(homedir(), '.orca', 'ai-providers')

export const aiProviderHandlers = {
  // Store encrypted credential on dev server filesystem
  'ai.provider.writeCredential': async (params: {
    accountId: string
    encryptedBlob: string
    iv: string
  }) => {
    await mkdir(PROVIDER_STORE_DIR, { recursive: true })
    const data = JSON.stringify({ encryptedBlob: params.encryptedBlob, iv: params.iv, updatedAt: Date.now() })
    await writeFile(join(PROVIDER_STORE_DIR, `${params.accountId}.enc`), data, 'utf-8')
    return { ok: true }
  },

  // Read encrypted credential from dev server (for relay to use)
  'ai.provider.readCredential': async (params: { accountId: string }) => {
    const filePath = join(PROVIDER_STORE_DIR, `${params.accountId}.enc`)
    const raw = await readFile(filePath, 'utf-8')
    return JSON.parse(raw)
  },

  // Health check — minimal API call
  'ai.provider.healthCheck': async (params: {
    accountId: string
    provider: string
    model?: string
  }) => {
    const start = Date.now()
    try {
      const cred = await aiProviderHandlers['ai.provider.readCredential']({ accountId: params.accountId })
      // Perform minimal health check based on provider type
      // ...implementation per provider...
      return { ok: true, latencyMs: Date.now() - start }
    } catch (err: any) {
      return { ok: false, latencyMs: Date.now() - start, error: err.message }
    }
  },
}
```

**Note:** Register trong relay entry point sau TASK-024 hoàn thành.

## Acceptance Criteria

- [x] `aiProviderHandlers` export
- [x] `writeCredential`: writes to `~/.orca/ai-providers/<id>.enc`
- [x] `readCredential`: reads + parses encrypted blob
- [x] `healthCheck`: non-throwing, returns `{ ok, latencyMs }`
- [x] Không import từ `src/main/`
