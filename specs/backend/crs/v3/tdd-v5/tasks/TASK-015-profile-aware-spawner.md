# TASK-015: ProfileAwareAgentSpawner

**Phase:** 3 — Project Binding  
**Solution ref:** [SOL-V5-002](../solutions/SOL-V5-002-project-binding.md) §6  
**Prerequisite:** TASK-014 (ProjectServerRouter), TASK-008 (ProfileResolver)  
**Status:** ✅ DONE — 2026-07-28

---

## File cần tạo: `src/main/project/ProfileAwareAgentSpawner.ts`

Implement theo SOL-V5-002 §6:

- Constructor: `(router: ProjectServerRouter, profileResolver: ProfileResolver, providerService: AIProviderService)`
- `spawn(options: AgentSpawnOptions)` → calls relay `agent.exec` với full profile context
- Inject env: `resolvedProfile.shell?.envVars`, `PATH` từ `pathAdditions`, `ORCA_*` vars
- Resolve AI provider từ `providerService.resolveForProject(..., resolvedProfile.agent?.preferredModel)`

**Note:** `AIProviderService` sẽ được tạo ở Phase 4. Dùng `import type` để avoid circular, hoặc nhận provider service qua interface `{ resolveForProject(...) }`.

## Acceptance Criteria

- [x] `ProfileAwareAgentSpawner` export
- [x] `spawn()` inject profile envVars vào agent
- [x] `spawn()` prepend pathAdditions vào PATH
- [x] Không TypeScript errors
