# F28 — Dev Server Onboarding

| Trường | Giá trị |
|--------|---------|
| **ID** | F28 |
| **Tên** | Dev Server Onboarding |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [onboarding CR-OB-001~009](../crs/v1/onboarding/) |
| **TDD** | [TDD-13: Dev Server Onboarding](../specs/backend/tdd/13-dev-server-onboarding.md) |
| **Phiên bản** | v3.0+ |
| **ADR References** | ADR-004 |
| **HLD References** | C3.5 |

---

## Mô tả

Orca cung cấp **guided onboarding wizard** để đăng ký và cấu hình dev servers từ xa — detect agent, preflight check, tự động deploy relay, và thông báo push khi hoàn thành.

---

## Tính năng chi tiết

### Dev Server Registration
- Đăng ký nhiều remote dev servers (`AddInstanceForm`)
- Instance switcher để chuyển giữa các servers (`OrcaInstanceSwitcher`)
- `DevServerManager` — manage server registry

### Platform-Aware Wizard
- Tự động phát hiện OS của remote server (Linux/Mac/Windows)
- Branching wizard steps theo platform
- `SshProvisioningProgress` — progress bar với steps
- `SshUserIndicator` — hiển thị linux username hiện tại

### Remote Agent Detection
- `detectAgentOnRemote()` — SSH vào remote, check if Orca relay running
- `FleetRemoteCommands` — execute remote commands qua SSH

### Preflight Check System
- Kiểm tra prerequisites trên remote server trước khi deploy
- System requirements: OS, ports, disk space, SSH access
- `FleetBootstrapService` — orchestrate preflight + bootstrap

### Auto Relay Deployment
- Tự động copy và install Orca relay binary lên remote
- Setup systemd/launchd service để relay auto-start
- Key authorization

### Web Push Notifications
- `WebPushManager` — gửi push notification khi onboarding complete
- `/api/push` endpoint — subscribe/unsubscribe
- Notification khi agent finish task, server status change

### Multi-Dev-Server Checklist
- Checklist UI cho multi-server onboarding
- Progress tracking per server

---

## Luồng người dùng

```
1. User click "Add Dev Server"
2. Fill SSH connection info (host, port, key)
3. Orca connects và runs preflight checks
4. Platform detection → show platform-specific steps
5. Deploy relay binary → authorize SSH key
6. Show "provisioning progress" UI
7. Complete → push notification + server available in fleet
```

---

## Tiêu chí chấp nhận

- [x] Đăng ký dev server với SSH credentials
- [x] Platform-aware wizard (Linux/Mac/Windows branching)
- [x] Remote agent detection via SSH
- [x] Preflight check system
- [x] Auto relay deployment
- [x] SshProvisioningProgress UI component
- [x] Web push notification khi hoàn thành
- [x] Multi-dev-server checklist

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Fleet bootstrap | `src/main/ssh/fleet-bootstrap-service.ts` |
| Fleet remote commands | `src/main/ssh/fleet-remote-commands.ts` |
| Web push manager | `src/server/web-push-manager.ts` |
| Add instance form | `src/renderer/src/web/AddInstanceForm.tsx` |
| Instance switcher | `src/renderer/src/web/OrcaInstanceSwitcher.tsx` |
| Provisioning progress | `src/renderer/src/components/ssh/SshProvisioningProgress.tsx` |
| SSH user indicator | `src/renderer/src/components/ssh/SshUserIndicator.tsx` |
| Web connect | `src/renderer/src/web/WebConnect.tsx` |

**Tests:** via onboarding integration tests
