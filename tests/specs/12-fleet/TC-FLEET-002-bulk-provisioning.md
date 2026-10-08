# TC-FLEET-002 — Bulk Server Provisioning

**BL Reference:** BL-FLEET-02  
**Priority:** P1

---

## TC-FLEET-002-01: Bulk deploy relay — 3 servers

### Steps
1. `fleet.bulkProvision { serverIds: ['srv-1', 'srv-2', 'srv-3'] }`

### Expected Results
- Parallel SSH connect + relay deploy cho tất cả
- Progress: `{ srv-1: 'complete', srv-2: 'complete', srv-3: 'complete' }`

---

## TC-FLEET-002-02: Partial failure — 1 server fails

### Steps
1. srv-2 unreachable
2. `fleet.bulkProvision { serverIds: ['srv-1', 'srv-2', 'srv-3'] }`

### Expected Results
- srv-1, srv-3: provisioned
- srv-2: failed, error logged
- Overall: `{ succeeded: 2, failed: 1, failedServers: ['srv-2'] }`

