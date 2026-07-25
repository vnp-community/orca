# Solutions — Onboarding CR v1

Thư mục này chứa **Backend Technical Solutions** cho từng Change Request trong [`docs/crs/v1/onboarding/`](../../../../../docs/crs/v1/onboarding/).

Mỗi solution được thiết kế theo chuẩn TDD của Orca (xem [`specs/backend/tdd/`](../../../tdd/)).

---

## Trạng thái

| CR ID | Solution File | Status | Phase |
|-------|--------------|--------|-------|
| CR-OB-001 | (Xem README này) | ✅ Architecture defined | — |
| CR-OB-002 | [SOL-002-dev-server-manager.md](./SOL-002-dev-server-manager.md) | ✅ Implemented | Phase 1 |
| CR-OB-003 | [SOL-003-remote-agent-detection.md](./SOL-003-remote-agent-detection.md) | ✅ Implemented | Phase 1 |
| CR-OB-004 | [SOL-004-platform-aware-wizard.md](./SOL-004-platform-aware-wizard.md) | ✅ Implemented | Phase 2 |
| CR-OB-005 | [SOL-005-remote-preflight.md](./SOL-005-remote-preflight.md) | ✅ Implemented | Phase 2 |
| CR-OB-006 | [SOL-006-remote-repo.md](./SOL-006-remote-repo.md) | ✅ Implemented (relay git.clone deferred) | Phase 2 |
| CR-OB-007 | [SOL-007-windows-terminal-remote.md](./SOL-007-windows-terminal-remote.md) | ✅ Implemented | Phase 3 |
| CR-OB-008 | [SOL-008-web-push-notifications.md](./SOL-008-web-push-notifications.md) | ✅ Implemented | Phase 3 |
| CR-OB-009 | [SOL-009-multiserver-checklist.md](./SOL-009-multiserver-checklist.md) | ✅ Implemented | Phase 3 |

---

## Thứ tự triển khai

```
Phase 1 (Foundation):
  SOL-002 → DevServerManager + schema
  SOL-003 → Remote agent detection

Phase 2 (Wizard Steps):
  SOL-004 → Platform-aware wizard logic
  SOL-005 → Remote preflight (gh/git)
  SOL-006 → Remote repo/folder

Phase 3 (Polish):
  SOL-007 → Remote Windows capabilities
  SOL-008 → Web Push notifications
  SOL-009 → Multi-server checklist
```

---

## TDD References

| TDD | Link | Liên quan |
|-----|------|-----------|
| TDD-05 | [05-ssh-relay.md](../../../tdd/05-ssh-relay.md) | SSH relay, SshConnection, RelaySession |
| TDD-06 | [06-persistence.md](../../../tdd/06-persistence.md) | Store schema, migrations |
| TDD-09 | [09-ipc-handlers.md](../../../tdd/09-ipc-handlers.md) | IPC handler pattern |
| TDD-11 | [11-web-server-mode.md](../../../tdd/11-web-server-mode.md) | Web server, HTTP, WebSocket |
