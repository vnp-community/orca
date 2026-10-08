---
orca_schema: 1
kind: task
id: TSK-142.1.1
digest: "sha256:4274df3beb0b7cd0dc7bbb59139031652af65b78c5f14a9a8f7c8b452ae75b43"
---
# TSK-142.1.1

<!-- orca:begin document digest=sha256:4274df3beb0b7cd0dc7bbb59139031652af65b78c5f14a9a8f7c8b452ae75b43 -->
## Document
- acceptance:
  - #1: 200 OK
- checks:
  - #1:
    - description: go test
    - id: c1
    - kind: test
- exempt_from_coverage: false
- kind: task
- objective: Thêm route
- satisfies:
  - #1: AC-1
- schema_version: 1

```orca-json
{"acceptance":["200 OK"],"checks":[{"description":"go test","id":"c1","kind":"test"}],"exempt_from_coverage":false,"kind":"task","objective":"Thêm route","satisfies":["AC-1"],"schema_version":1}
```
<!-- orca:end document -->
