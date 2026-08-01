# BUG-FE-ORCH-001: Không có IPC bridge cho `agent.start`, `agent.stop`, `agent.resume` — UI không gọi được AgentManager

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-01) mô tả:
```
[Renderer] click "Start Agent" trên worktree card
    contextBridge.invoke('agent.start', { worktreeId, agentType, trustPreset })
```

Thực tế grep `src/renderer/src`:
```
invoke('agent.start', ...) → No results
contextBridge.*agent.start → No results
ipcRenderer.invoke.*agent  → No results
```

Renderer **không có code nào gọi `agent.start`/`agent.stop`/`agent.resume`** qua IPC.

## Thực tế trong UI

Renderer chỉ dùng **Terminal-based agent launch**:
- User tạo terminal → type agent command thủ công
- Worktree card "Start Agent" button → trigger `terminal.create` với agentBinary command
- Agent status được track qua PTY hooks (`agent-hooks.ts`) trong local terminal

Không có remote agent orchestration flow nào từ UI.

## Ảnh hưởng

1. Kể cả khi AgentManager được implement (BUG-AG-ORCH-005), UI không có cách gọi nó
2. IPC handler `agent.start`, `agent.stop`, `agent.resume` cần được thêm vào preload/main
3. Agent card UI chỉ show status của LOCAL agents, không phải remote agent sessions

## Code cần thêm

**Preload** (`src/preload/`):
```typescript
agentStart: (opts) => ipcRenderer.invoke('agent:start', opts),
agentStop: (opts) => ipcRenderer.invoke('agent:stop', opts),
agentResume: (opts) => ipcRenderer.invoke('agent:resume', opts),
```

**Main IPC** (`src/main/ipc/agent-orchestration-ipc.ts`):
```typescript
ipcMain.handle('agent:start', async (_, opts) => agentManager.start(opts))
ipcMain.handle('agent:stop', async (_, opts) => agentManager.stop(opts))
ipcMain.handle('agent:resume', async (_, opts) => agentManager.resume(opts))
```

## Liên quan đến luồng

- **BL-AG-01**: Start Agent — IPC bridge missing
- **BL-AG-02**: Stop Agent — IPC bridge missing  
- **BL-AG-03**: Resume Session — IPC bridge missing
