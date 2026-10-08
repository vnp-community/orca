# TC-PRF-002 — Profile Inheritance Resolution (3-layer merge)

**BL Reference:** BL-PRF-02  
**Priority:** P0  
**Type:** Unit + Integration  
**Actor:** System

---

## Preconditions

Company profile:
```json
{ "agent": { "defaultModel": "claude-opus-4", "maxTokens": 8000 },
  "security": { "lockedModels": ["gpt-3.5"] },
  "envVars": { "COMPANY_ENV": "production", "LOG_LEVEL": "info" } }
```

Department profile (parent=company):
```json
{ "agent": { "maxTokens": 16000 },
  "envVars": { "DEPT_ENV": "engineering" } }
```

User profile (parent=dept):
```json
{ "agent": { "defaultModel": "gpt-4" },
  "envVars": { "USER_PREF": "dark", "LOG_LEVEL": "debug" } }
```

---

## TC-PRF-002-01: User wins over Dept over Company — agent.defaultModel

**Priority:** P0

### Steps
1. `ProfileResolver.resolve(userId)`

### Expected Results
- `effective.agent.defaultModel === 'gpt-4'` (User wins)

---

## TC-PRF-002-02: Dept wins over Company — agent.maxTokens

**Priority:** P0

### Steps
1. `ProfileResolver.resolve(userId)`

### Expected Results
- `effective.agent.maxTokens === 16000` (Dept overrides Company)

---

## TC-PRF-002-03: envVars override merge — User > Dept > Company

**Priority:** P0

### Expected Results
```
effective.envVars = {
  COMPANY_ENV: 'production',    // Company, not overridden
  LOG_LEVEL: 'debug',           // User overrides Company's 'info'
  DEPT_ENV: 'engineering',      // Dept, not overridden
  USER_PREF: 'dark'             // User only
}
```

---

## TC-PRF-002-04: security.lockedModels — Company level LOCK

**Priority:** P0  
**Security:** CRITICAL — User CANNOT override security fields

### Steps
1. User profile: `security: { lockedModels: [] }` (tries to override)
2. `ProfileResolver.resolve(userId)`

### Expected Results
- `effective.security.lockedModels === ['gpt-3.5']` (Company lock wins)
- User's security override IGNORED

### Assertions
```
resolved = ProfileResolver.resolve(userId)
assert resolved.security.lockedModels.includes('gpt-3.5')
// User cannot bypass this
```

---

## TC-PRF-002-05: pathAdditions — Concatenation (tất cả tầng append)

**Priority:** P0

### Preconditions
- Company: `pathAdditions: ['/company/bin']`
- Dept: `pathAdditions: ['/dept/bin']`
- User: `pathAdditions: ['/user/bin']`

### Expected Results
```
effective.pathAdditions = ['/company/bin', '/dept/bin', '/user/bin']
```

---

## TC-PRF-002-06: Cache TTL 60s

**Priority:** P1

### Steps
1. Resolve profile → cache hit
2. Within 60s: resolve again → served from cache
3. Parent profile updated
4. Next resolve (after cache expires) → fresh resolve

### Assertions
```
spy.reset()
resolve1 = ProfileResolver.resolve(userId)
resolve2 = ProfileResolver.resolve(userId) // same call, within 60s

assert spy.callCount === 1 // cached, DB only hit once

// Update parent, advance time 61s
updateParentProfile(deptId, { agent: { maxTokens: 32000 } })
advanceTime(61000)

resolve3 = ProfileResolver.resolve(userId)
assert resolve3.agent.maxTokens === 32000 // fresh from DB
```

---

## TC-PRF-002-07: Cache invalidation khi parent thay đổi

**Priority:** P1

### Steps
1. Resolve → cached
2. Company profile updated (parent)
3. Next resolve → cache INVALIDATED

### Expected Results
- Cache entry for userId invalidated immediately on parent change

