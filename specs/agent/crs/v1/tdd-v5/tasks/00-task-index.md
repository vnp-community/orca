# Agent TDD v5 — AI Execution Task Index

**Workspace:** `/Users/binhnt/Work/blockchain/vnp-blc/orca`  
**Source dir:** `src/relay/`  
**Tests dir:** `src/relay/__tests__/`  
**Build:** `pnpm run build:agent` → `out/relay/agent.js`  
**Test:** `pnpm test -- --reporter verbose src/relay/__tests__/agent-`  
**TypeCheck:** `pnpm run typecheck:node`

---

## Task List

| Task | File(s) tạo | SOL Ref | Phase | Status |
|------|------------|---------|-------|--------|
| [TASK-01](./01-build-integration.md) | `config/scripts/build-relay.mjs` EXTEND | SOL-01 | 1 | ✅ Done |
| [TASK-02](./02-agent-logger.md) | `src/relay/agent-logger.ts` NEW | SOL-03 | 1 | ✅ Done |
| [TASK-03](./03-agent-config.md) | `src/relay/agent-config.ts` NEW | SOL-03 | 1 | ✅ Done |
| [TASK-04](./04-agent-wire.md) | `src/relay/agent-wire.ts` NEW | SOL-02 | 2 | ✅ Done |
| [TASK-05](./05-agent-tool-registry.md) | `src/relay/agent-tool-registry.ts` NEW | SOL-04 | 2 | ✅ Done |
| [TASK-06](./06-agent-rpc-dispatch.md) | `src/relay/agent-rpc-dispatch.ts` NEW | SOL-05 | 3 | ✅ Done |
| [TASK-07](./07-agent-session.md) | `src/relay/agent-session.ts` NEW | SOL-06 | 3 | ✅ Done |
| [TASK-08](./08-agent-connections.md) | `agent-connection-direct.ts` + `agent-connection-relay.ts` NEW | SOL-07 | 4 | ✅ Done |
| [TASK-09](./09-agent-entry.md) | `src/relay/agent-entry.ts` NEW | SOL-08 | 4 | ✅ Done |
| [TASK-10](./10-git-handler.md) | `src/relay/agent-git-handler.ts` NEW | SOL-09 | 5 | ✅ Done |
| [TASK-11](./11-credential-store.md) | `src/relay/agent-credential-store.ts` NEW | SOL-10 | 5 | ✅ Done |
| [TASK-12](./12-fs-extensions.md) | `src/relay/fs-agent-extensions.ts` NEW | SOL-11 | 5 | ✅ Done |
| [TASK-13](./13-tests-wire-config.md) | `__tests__/agent-wire.test.ts` + `agent-config.test.ts` | SOL-12 | 6 | ✅ Done |
| [TASK-14](./14-tests-registry-dispatch.md) | `__tests__/agent-tool-registry.test.ts` + `agent-rpc-dispatch.test.ts` | SOL-12 | 6 | ✅ Done |
| [TASK-15](./15-tests-session-connections.md) | `__tests__/agent-session.test.ts` + `agent-connection-*.test.ts` | SOL-12 | 6 | ✅ Done |
| [TASK-16](./16-tests-extensions.md) | `__tests__/agent-git-handler.test.ts` + `agent-credential-store.test.ts` + `fs-agent-extensions.test.ts` | SOL-12 | 6 | ✅ Done |
| [TASK-17](./17-typecheck-build-verify.md) | `tsc` ✅ + `build:agent` ✅ + smoke test ✅ | — | 7 | ✅ Done |


---

## Rules cho AI thực thi

1. **Thực thi theo Phase** — không bỏ qua Phase
2. **Verify sau mỗi TASK** — chạy `pnpm run typecheck:node` trước khi sang task tiếp theo
3. **Không sửa file ngoài `src/relay/` và `config/scripts/build-relay.mjs`**
4. **Không mock `node:crypto`** trong tests
5. **`shell: false`** trong mọi `spawn()` call
6. **Import paths chính xác:**
   - `MessageType` từ `'../main/ssh/relay-protocol'`
   - `AGENT_*` constants từ `'../shared/agent-wire-protocol'`
   - `readRelayFileContent` từ `'./fs-handler-file-read'`
   - `checkRgAvailable` từ `'./fs-handler-utils'`
