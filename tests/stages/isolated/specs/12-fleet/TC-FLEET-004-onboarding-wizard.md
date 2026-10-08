# TC-FLEET-004 — Dev Server Onboarding Wizard

**BL Reference:** BL-FLEET-04  
**Priority:** P1

---

## TC-FLEET-004-01: Wizard — Linux server onboarding

### Steps
1. Launch onboarding wizard
2. Input: host, username, SSH key
3. Wizard connects, detects OS=Linux
4. Preflight checks: OS, ports, disk
5. Auto-deploy relay

### Expected Results
- Platform-aware flow (Linux branch)
- Preflight: OS=Linux, port 6799 open, disk > 1GB
- Relay deployed + started
- Push notification: "Server srv-1 onboarded successfully"

---

## TC-FLEET-004-02: Preflight check — Port in use

### Steps
1. Port 6799 đang bị occupied
2. Wizard runs preflight

### Expected Results
- Preflight fail: `{ check: 'port', status: 'fail', port: 6799 }`
- User informed, option to use different port

---

## TC-FLEET-004-03: Progress bar UI

### Steps
1. Onboarding in progress

### Expected Results
- `SshProvisioningProgress` component shows:
  - Step 1: SSH connect ✓
  - Step 2: Preflight checks ✓
  - Step 3: Upload relay binary (progress %)
  - Step 4: Start relay ✓

