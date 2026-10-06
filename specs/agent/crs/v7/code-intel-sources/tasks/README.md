# code-intel-sources (v7) tasks: index (agent)

> 📋 Proposed, chưa triển khai. Tất cả `Status: [ ] TODO`. Test: `cd /opt/repos/orca/agent && pnpm exec vitest run <file>`.

| Task | Tiêu đề | Depends on |
|---|---|---|
| AG-CV-TASK-037-01 | Thu fixture thật (Cypher, `check --cycles`) trên repo mẫu | GitNexus cài sẵn |
| 037-02 | Validate tham số, method table 55 s, handler theo `kind` | 037-01, SOL-001 |
| 037-03 | `check --cycles --json`: whitelist + tệp tạm + parser | 037-01, 037-02 |
| 037-04 | `layerImports` (4 cặp, khử trùng) | 037-01, 037-02, SOL-002 |
| 037-05 | `importInDegree` + `fileSizes` | 037-04 |
| 037-06 | `unusedExports` → `SymbolRef` | 037-02, SOL-002 |
| 037-07 | Phân trang, cache, hợp đồng JSON | 037-03..06 |

Solution: [`../solutions/AG-CV-SOL-037-structural-facts.md`](../solutions/AG-CV-SOL-037-structural-facts.md).
