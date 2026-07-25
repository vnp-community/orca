# TASK-005: Tạo file `src/main/dev-server/dev-server-relay-bridge.ts`

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §6  
**Depends on:** TASK-001  
**Blocks:** TASK-004, TASK-013

---

## Mục tiêu

Tạo class `DevServerRelayBridge` wrap SSH relay infrastructure hiện có, cung cấp interface thuần túy cho `DevServerManager`. Chỉ implement `relay-ssh` mode trong Phase 1.

---

## File cần tạo

**Path:** `src/main/dev-server/dev-server-relay-bridge.ts`

---

## Context cần tra cứu trước khi implement

Trước khi viết code, hãy đọc các file hiện có để hiểu đúng API:
- `src/relay/relay-handshake.ts` — kiểu `DaemonHandshakeCallbacks`, `RelayHandshakeInfo`
- `src/main/ssh/ssh-relay-deploy.ts` (hoặc tương đương) — hàm `deployRelay()`
- `src/main/ssh/ssh-connection-manager.ts` — `SshConnectionManager.getConnection()`
- `src/relay/relay-session.ts` — `SshRelaySession.call()`, `SshRelaySession.close()`

---

## Nội dung cần implement

```typescript
import type { SshConnectionManager } from '../ssh/ssh-connection-manager'
import type { PersistedDevServer } from '../../shared/dev-server-types'
// Import đúng từ relay infrastructure (tra cứu tên thật trong codebase):
import { deployRelay } from '../ssh/ssh-relay-deploy'
import type { SshRelaySession } from '../../relay/relay-session'

export type RelayHandshakeInfo = {
  platform: NodeJS.Platform
  arch: string
  nodeVersion: string
  relayVersion: string
}

export class DevServerRelayBridge {
  // session được expose để các method trong TASK-013, TASK-022 gọi relay calls
  session: SshRelaySession | null = null

  constructor(
    private config: PersistedDevServer,
    private sshManager: SshConnectionManager
  ) {}

  async connect(opts: { testOnly?: boolean } = {}): Promise<RelayHandshakeInfo> {
    if (this.config.connectionType === 'relay-ssh') {
      const conn = await this.sshManager.getConnection(this.config.sshTargetId!)
      const result = await deployRelay(conn, { testOnly: opts.testOnly })
      this.session = result.session
      return {
        platform: result.platform,
        arch: result.arch ?? process.arch,
        nodeVersion: result.nodeVersion ?? 'unknown',
        relayVersion: result.relayVersion
      }
    }
    // relay-websocket / direct-websocket: Phase 2 — chưa implement
    throw new Error(`Connection type '${this.config.connectionType}' not yet implemented`)
  }

  async disconnect(): Promise<void> {
    await this.session?.close()
    this.session = null
  }
}
```

> **Lưu ý:** `detectAgents()` và `callWithTimeout()` sẽ được thêm trong TASK-013 (SOL-003).

---

## Acceptance Criteria

- [x] File tồn tại tại `src/main/dev-server/dev-server-relay-bridge.ts`
- [x] `RelayHandshakeInfo` type được export
- [x] `DevServerRelayBridge` class được export
- [x] `session` là public để các task sau có thể dùng
- [x] `connect()` với `relay-ssh` gọi đúng `deployAndLaunchRelay()` và trả về `RelayHandshakeInfo`
- [x] `connect()` với type khác throw `Error` có message rõ ràng
- [x] `disconnect()` close session và set `session = null`
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

Nếu tên hàm `deployRelay` hoặc tên file không khớp với codebase thực tế, hãy:
1. Dùng `grep` để tìm hàm deploy relay trong `src/main/ssh/`
2. Dùng đúng tên đã tìm được
3. Ghi chú thay đổi vào comment trong file
