# TC-FLEET-001 — Fleet Inventory Config (YAML)

**BL Reference:** BL-FLEET-01  
**Priority:** P1  
**Actor:** Admin, DevOps

---

## TC-FLEET-001-01: Load fleet YAML config

### Steps
1. fleet.yaml:
   ```yaml
   servers:
     - id: srv-prod-1
       host: 10.0.0.1
       tags: [production, backend]
     - id: srv-dev-1
       host: 10.0.1.1
       tags: [dev, frontend]
   ```
2. `fleet.loadConfig { yamlPath: './fleet.yaml' }`

### Expected Results
- 2 servers registered
- Tags indexed for querying

---

## TC-FLEET-001-02: Fleet YAML — Invalid format

### Steps
1. Malformed YAML loaded

### Expected Results
- Error: `{ code: 'INVALID_FLEET_CONFIG' }`

---

## TC-FLEET-001-03: Filter by tags

### Steps
1. `fleet.list { tags: ['production'] }`

### Expected Results
- Only production servers returned

