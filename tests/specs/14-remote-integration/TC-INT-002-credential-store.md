# TC-INT-002 — WebCredentialStore (API Token Management)

**BL Reference:** BL-INT-02  
**Priority:** P1

---

## TC-INT-002-01: Store credential — AES-256-GCM

### Steps
1. `credStore.set { provider: 'bitbucket', token: 'bitbucket-token', userId }`

### Expected Results
- Token encrypted với AES-256-GCM
- Stored in per-user credentials store

---

## TC-INT-002-02: Retrieve credential

### Steps
1. `credStore.get { provider: 'bitbucket', userId }`

### Expected Results
- Token decrypted và returned

---

## TC-INT-002-03: User isolation — User A cannot read User B credentials

### Steps
1. User A: `credStore.set { provider: 'bitbucket', token: 'tokenA' }`
2. User B: `credStore.get { provider: 'bitbucket' }` (as User B)

### Expected Results
- User B gets `null` (no token for User B)
- Cannot access User A's token

