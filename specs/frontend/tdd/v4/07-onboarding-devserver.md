# TDD-FE-07: Onboarding & Dev Server UI

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/components/onboarding/`, `src/renderer/src/components/dev-server/`

---

## 1. Dev Server UI Components

```
components/dev-server/
├─ DevServerCard.tsx          ← Card hiển thị status, actions
├─ DevServerList.tsx          ← Danh sách tất cả dev servers
├─ DevServerDialog.tsx        ← Add/Edit dev server dialog
├─ DevServerStatusBadge.tsx   ← Status indicator (connected/error/etc.)
└─ DevServerConnectionForm.tsx← Form fields per connection type
```

---

## 2. DevServerCard

```tsx
function DevServerCard({ devServer }: { devServer: DevServer }) {
  return (
    <div className="dev-server-card">
      <DevServerStatusBadge status={devServer.status} />
      <h3>{devServer.name}</h3>
      <p>{devServer.host}</p>
      <ConnectionTypeLabel type={devServer.connectionType} />

      {/* Platform info (khi connected) */}
      {devServer.platform && (
        <PlatformBadge platform={devServer.platform} arch={devServer.arch} />
      )}

      {/* SSH User Indicator (v4.0) */}
      <SshUserIndicator serverId={devServer.id} />

      <CardActions>
        <ConnectButton devServerId={devServer.id} />
        <DisconnectButton devServerId={devServer.id} />
        <EditButton />
        <DeleteButton />
      </CardActions>
    </div>
  )
}
```

---

## 3. DevServerStatusBadge

```tsx
function DevServerStatusBadge({ status }: { status: DevServerStatus }) {
  const colors = {
    'connected':    'green',
    'connecting':   'yellow',
    'disconnected': 'gray',
    'error':        'red'
  }
  // Animated pulse dot + status text
}
```

---

## 4. DevServerDialog (Add/Edit)

```tsx
function DevServerDialog({ onClose }: { onClose: () => void }) {
  // Connection type selector:
  //   - direct-websocket (agent trên remote server kết nối vào Orca)
  //   - relay-ssh (Orca SSH vào remote server qua relay)
  //   - relay-websocket (Orca kết nối qua relay server)
  //
  // Fields per type:
  //   direct-websocket: name only (token tự generate)
  //   relay-ssh:        name, host, port, ssh user, ssh key
  //   relay-websocket:  name, relay URL
}
```

---

## 5. Onboarding Wizard

```tsx
// components/onboarding/
// Step-by-step wizard cho first-time setup

components/onboarding/
├─ OnboardingWizard.tsx       ← Multi-step container
├─ DevServerStep.tsx          ← Step: Add dev server
├─ AgentStep.tsx              ← Step: Install agent on dev server
├─ VerifyStep.tsx             ← Step: Verify connection
└─ CompletionStep.tsx         ← Step: Done!
```

---

## 6. Agent Detection Hook

```typescript
// src/renderer/src/hooks/useRemoteAgentDetection.ts

function useRemoteAgentDetection(serverId: string): {
  detected:    boolean
  agents:      AgentKind[]   // claude, codex, cursor, etc.
  loading:     boolean
  error:       string | null
  refresh:     () => void
}

// Cache: module-level Map<serverId, { result, cachedAt }>
// TTL: 60 seconds
// Invokes: window.api.detectRemoteAgents(serverId)
```

---

## 7. Remote Directory Browser

```tsx
// components/remote-browser/RemoteDirectoryBrowser.tsx
// Browse filesystem trên remote dev server (qua agent tools)

function RemoteDirectoryBrowser({ serverId, rootPath }: Props) {
  // Tree view: expand/collapse directories
  // File click: open in editor (remote file)
  // Actions: create file, delete, rename
}
```

---

## 8. Connection Type Flows

### direct-websocket

```
1. User click "Add Server" → DevServerDialog
2. Enter name → Submit
3. POST /api/agent-token { name, devServerId }  [from UI via IPC]
4. Display: "Copy this command to your remote server:"
   AGENT_TOKEN=<token> ORCA_URL=wss://... systemctl start orca-agent
5. UI polls status → 'connecting' → 'connected' (khi agent connects)
```

### relay-ssh

```
1. User enter host, port, user, key
2. Click "Test Connection" → ConnectionTestResult
3. Click "Connect" → SSH tunnel established
4. Agent tools available
```

---

## 9. DevServer IPC Events (Zustand binding)

```typescript
// Tự động sync từ backend qua IPC/WebSocket:
ipc.on('devServer:added',         handleAdded)
ipc.on('devServer:removed',       handleRemoved)
ipc.on('devServer:statusChanged', handleStatusChanged)

// Store actions:
store.addDevServer(ds)
store.removeDevServer(id)
store.updateDevServerStatus(id, status, error)
```
