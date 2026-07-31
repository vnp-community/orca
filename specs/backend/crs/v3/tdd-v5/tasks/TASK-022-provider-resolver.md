# TASK-022: ProviderResolver Priority Logic

**Phase:** 4 — AI Provider Management  
**Solution ref:** [SOL-V5-003](../solutions/SOL-V5-003-ai-provider.md) §5  
**Prerequisite:** TASK-021 (AIProviderService)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/ai-providers/ProviderResolver.ts`

Priority resolution (TDD-16 §4):
1. User-scope accounts (scope='user', scopeRefId=userId) matching modelHint
2. Project-scope accounts (scope='project', scopeRefId=projectId) matching modelHint
3. Server-scope accounts (scope='server') matching modelHint
4. Repeat for each scope without modelHint filter
5. Return null if none found

```typescript
export interface ResolveOptions {
  devServerId: string
  projectId: string
  userId: string
  modelHint?: string
}

export class ProviderResolver {
  constructor(private readonly service: AIProviderService) {}
  
  async resolve(options: ResolveOptions): Promise<AIProviderAccount> { ... }
}
```

Filter accounts:
- `status === 'active'`
- Quota check: `account.quotaLimitDay === 0` (unlimited) OR `usageToday.tokens < account.quotaLimitDay`

## Acceptance Criteria

- [x] `ProviderResolver` class export
- [x] Priority order: user > project > server scope
- [x] ModelHint filter applied first, then fallback without
- [x] Only active accounts considered
- [x] Quota check included
- [x] Throws `NO_PROVIDER_AVAILABLE` if none found
