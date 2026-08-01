# BL-FLEET-04: Dev Server Onboarding Wizard

**Domain:** Fleet Management  
**Priority:** P1  
**Actor chính:** Carlos, Admin  
**Tham chiếu:** FR-16, UR-121, F28

---

## Mô tả

Guided wizard giúp user đăng ký và cấu hình dev server từ xa, từ SSH credentials đến relay được deploy và relay hoạt động.

## Wizard Steps

```
Step 1: Connect Dev Server
  - Input: hostname, SSH user, SSH key path, port (default 22)
  - Action: test SSH connection (timeout 10s)
  - Success: → Step 2
  - Failure: show error + retry button

Step 2: Detect Platform
  - Action: SSH exec "uname -s && uname -m && lsb_release -rs 2>/dev/null"
  - Shows: OS (Linux/macOS), arch (x86_64/arm64), distro
  - Auto-select relay binary variant

Step 3: Detect AI Agents
  - Action: SSH exec "which claude codex gemini openai 2>/dev/null"
  - Shows: detected agents as tags (claude ✓, codex ✗, etc.)
  - Recommend: install missing agents

Step 4: Preflight Check
  - Checks:
    - Git >= 2.25: "git --version"
    - Node.js >= 22: "node --version"
    - Disk space >= 5GB: "df -P ~/.orca"
    - Ports: probe relay default port (random 40000-50000)
    - GitHub CLI: "gh --version" (if needed)
  - Show: ✓/✗/⚠ per check
  - Allow: proceed with warnings (non-critical failures)

Step 5: Deploy Relay
  - Action: SFTP upload orca-relay binary to ~/.local/bin/orca-relay
  - Verify: SHA256 hash comparison
  - Start: ~/.local/bin/orca-relay --daemon --port <auto>
  - Test: GET http://127.0.0.1:<port>/health (via SSH tunnel)

Step 6: Register
  - Create DevServer record { id, hostname, user, relayPort, sshKeyPath, status: "online" }
  - Save to persistence
  - Show: "Dev Server registered successfully!"

Step 7: Multi-Server Checklist (nếu từ fleet import)
  - Show checklist: list server với per-server status
  - "Next server →" button
```

## State Machine

```
IDLE → CONNECTING → DETECTING → PREFLIGHT → DEPLOYING → REGISTERING → DONE
                 ↘ (error)                ↘ (error)   ↘ (error)
                   FAILED ← FAILED ← FAILED
```

## Source References

- `src/renderer/src/components/DevServerOnboardingWizard.tsx`
- `src/main/dev-server/onboarding-service.ts`
- `src/main/ssh/relay-deploy.ts`
- `src/main/dev-server/dev-server-manager.ts` — registerDevServer()
