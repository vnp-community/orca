# TC-INT-001 — CLI Auth Proxy (GitHub/GitLab qua SSH Relay)

**BL Reference:** BL-INT-01  
**Priority:** P1  
**Actor:** Carlos, Alex

---

## TC-INT-001-01: GitHub CLI auth qua SSH relay

### Steps
1. Carlos connects Dev Server qua SSH
2. `gh auth login` proxied qua relay

### Expected Results
- `preflight.check` gửi qua SSH relay tới Dev Server
- `gh auth status` checks on Dev Server
- `GH_CONFIG_DIR` isolated per session (không shared giữa users)

---

## TC-INT-001-02: Session isolation — GH_CONFIG_DIR

### Steps
1. User A có GitHub session A
2. User B có GitHub session B
3. Verify configs không shared

### Expected Results
- User A: `GH_CONFIG_DIR=/tmp/orca-gh-userA`
- User B: `GH_CONFIG_DIR=/tmp/orca-gh-userB`
- Separate config dirs

