# TC-AIP-003 — Provider Health Check & Quota Management

**BL Reference:** BL-AIP-03  
**Priority:** P1  
**Type:** Integration  
**Actor:** System, Admin

---

## TC-AIP-003-01: Health check cron — mỗi 15 phút

**Priority:** P1

### Steps
1. Setup cron scheduler
2. Advance time 15 minutes
3. Verify health check executed

### Expected Results
- Health check calls each registered provider
- Status updated: healthy/degraded/error

---

## TC-AIP-003-02: Quota tracking — Usage %

**Priority:** P1

### Steps
1. Provider usage: 80,000 tokens (limit 100,000)
2. Verify quota alert

### Expected Results
- `quota.usedPercent === 80`
- Alert emitted: `provider:quotaAlert { provider, usedPercent: 80 }`
- Admin notification

---

## TC-AIP-003-03: Quota 80% alert threshold

**Priority:** P1

### Steps
1. Usage < 80%: no alert
2. Usage = 80%: alert triggered
3. Usage = 90%: alert (not repeated)

### Assertions
```
updateUsage(providerId, 79)
assert !alerts.includes('quotaAlert')

updateUsage(providerId, 80)
assert alerts.includes('quotaAlert') // triggered at exactly 80%

updateUsage(providerId, 90)
assert alerts.filter(a => a === 'quotaAlert').length === 1 // not repeated
```

---

## TC-AIP-003-04: Key rotation — 30s grace period

**Priority:** P1

### Steps
1. Admin trigger key rotation: `aiProvider.rotateKey { id, newKey: 'sk-ant-new' }`
2. Verify: old key still works during 30s grace period
3. After 30s: only new key works

### Assertions
```
rotateKey(providerId, 'sk-ant-new')

// Within 30s: old key still usable
response = await callWithOldKey(providerId)
assert response.status === 'ok'

// After 30s
advanceTime(31000)
response2 = await callWithOldKey(providerId)
assert response2.status === 'auth_failed' // old key revoked
```

