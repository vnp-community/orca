# BUG-WT-002 [BACKEND]: `ProfileAwareAgentSpawner.spawn()` inject credentials qua `Object.assign(profileEnv, provider.credentials)` — API keys rò rỉ vào process.env của Dev Server

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WT-002  
**Note:** WorkspaceService.ts: remove credential injection from spawner  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/main/project/ProfileAwareAgentSpawner.ts:101`:
```typescript
// Merge provider credentials into env (e.g. API keys)
Object.assign(profileEnv, provider.credentials)
```

`provider.credentials` chứa `{ ANTHROPIC_API_KEY: 'sk-ant-xxx', ... }` được inject vào env của `relay.call('agent.exec', { env: profileEnv })`.

Dev Server khi spawn agent process với `env` từ relay → API keys được lưu trong:
1. `node-pty.spawn({ env })` — OK (env của process)
2. Nhưng Dev Server relay có thể log `env` trong debug output
3. Nếu env của PTY process bị inspect (e.g., `/proc/<pid>/environ`), keys bị lộ

Quan trọng hơn: `provider.credentials` đến từ `AIProviderService.resolveForProject()` — credentials này là gì? Từ encrypted Dev Server storage? Hay là plaintext từ Orca Server?

Xem BUG-AIP-002: credentials không được decrypt đúng trước khi relay → hiện tại `provider.credentials` có thể là `{}` (empty) vì flow bị broken.

## Vấn đề bổ sung

`src/main/project/ProfileAwareAgentSpawner.ts:96`:
```typescript
const provider = await this.providerService.resolveForProject(projectId, preferredModel)
```

`AIProviderResolver` interface (line 21-26):
```typescript
interface AIProviderResolver {
  resolveForProject(
    projectId: string,
    preferredModel: string | undefined
  ): Promise<{ providerId: string; modelId: string; credentials: Record<string, string> } | null>
}
```

Nhưng `AIProviderService.resolveForProject()` signature thực tế (từ AIProviderService.ts:315):
```typescript
async resolveForProject(
  devServerId: string,
  projectId: string,
  userId: string,
  modelHint?: string
): Promise<AIProviderAccount | null>
```

**SIGNATURE MISMATCH**: Interface trong `ProfileAwareAgentSpawner` khác với class implementation thực tế!
- Interface: `(projectId, preferredModel)`
- Implementation: `(devServerId, projectId, userId, modelHint)`

→ `providerService.resolveForProject(projectId, preferredModel)` sẽ **FAIL** vì thiếu `devServerId` và `userId`.

## Fix đề xuất

Phải update interface hoặc adapter:
```typescript
// Trong ProfileAwareAgentSpawner.spawn():
const project = await this.router.getProject(projectId)
const provider = await this.providerService.resolveForProject(
  project.devServerId,  // thiếu
  projectId,
  userId,               // thiếu
  preferredModel
)
```

## Files liên quan

- `src/main/project/ProfileAwareAgentSpawner.ts:22-26,96,101`: interface mismatch + credential leak
- `src/main/ai-providers/AIProviderService.ts:315`: actual signature
