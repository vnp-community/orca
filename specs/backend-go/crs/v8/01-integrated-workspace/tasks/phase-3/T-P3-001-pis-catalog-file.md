---
task_id: T-P3-001
phase: 3
cr_id: CR-ORCA-001
solution_ref: solutions/CR-ORCA-001-solution.md
service: orca-agent
target_file: orca-agent/internal/mcp/catalog_pis.go
priority: P1
status: done
depends_on: T-P2-003
estimated_effort: 1 giờ
blocks: T-P3-003
---

# T-P3-001 — Tạo file catalog_pis.go với 5 pis_* tool definitions

## Context

Tool catalog là danh sách tools được expose cho AI agent biết có thể gọi. Hiện tại chỉ có `orca.*` tools. Cần tạo file riêng `catalog_pis.go` chứa 5 `pis_*` tool definitions để merge vào catalog khi `ProductsAgentID` được configured.

Tách thành file riêng để `OrcaToolCatalog()` không phụ thuộc trực tiếp vào pis package (tránh import cycle tiềm ẩn).

## Target file

```
orca-agent/internal/mcp/catalog_pis.go   ← NEW FILE
```

## Instructions

1. **Tạo file mới** `orca-agent/internal/mcp/catalog_pis.go`

2. **Viết toàn bộ nội dung**:

```go
package mcp

// PISToolCatalog trả về 5 pis_* tool definitions để merge vào OrcaToolCatalog.
// CR-ORCA-001: chỉ được thêm khi Bridges.ProductsAgentID được cấu hình trong main.go.
// Tách file riêng để OrcaToolCatalog() không phụ thuộc vào pis package.
func PISToolCatalog() []Tool {
    return []Tool{
        {
            Name:        "pis_query_feature_context",
            Description: "Query full feature context từ PIKB KG: HLD, CRS, ADR liên quan đến Feature ID. Dùng trước khi implement để có đầy đủ context.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "feature_id": map[string]any{
                        "type":        "string",
                        "description": "Feature ID, vd: F27",
                    },
                },
                "required": []string{"feature_id"},
            },
        },
        {
            Name:        "pis_query_crs",
            Description: "Query CRS node theo ID. Trả về status, layer, feature_ref, relationships với HLD/ADR.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "crs_id": map[string]any{
                        "type":        "string",
                        "description": "CRS ID, vd: CRS-045",
                    },
                },
                "required": []string{"crs_id"},
            },
        },
        {
            Name:        "pis_query_hld",
            Description: "Query HLD node theo product, version, C4 level (L1–L4).",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "product_id": map[string]any{"type": "string"},
                    "version":    map[string]any{"type": "string"},
                    "level":      map[string]any{"type": "string", "description": "L1|L2|L3|L4"},
                },
                "required": []string{"product_id"},
            },
        },
        {
            Name:        "pis_query_adr",
            Description: "Query ADR node theo ID. Trả về status, supersedes, relationships.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "adr_id": map[string]any{
                        "type":        "string",
                        "description": "ADR ID, vd: ADR-021",
                    },
                },
                "required": []string{"adr_id"},
            },
        },
        {
            Name:        "pis_kg_search",
            Description: "Full-text search trên Dev Knowledge KG. Filter theo type (crs|hld|adr|tdd|bug) và product_id.",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "q":          map[string]any{"type": "string", "description": "Search query"},
                    "type":       map[string]any{"type": "string", "description": "crs|hld|adr|tdd|bug"},
                    "product_id": map[string]any{"type": "string", "description": "Filter theo product"},
                },
                "required": []string{"q"},
            },
        },
    }
}
```

3. **Kiểm tra type `Tool`** — xem file catalog hiện tại để đảm bảo struct `Tool` đã được định nghĩa trong package `mcp` với đúng fields `Name`, `Description`, `InputSchema`. Nếu field names khác thì điều chỉnh cho phù hợp.

## Acceptance criteria

- [ ] File `catalog_pis.go` tồn tại trong package `mcp`
- [ ] `PISToolCatalog()` trả về slice 5 `Tool` elements
- [ ] 5 tool names đúng: `pis_query_feature_context`, `pis_query_crs`, `pis_query_hld`, `pis_query_adr`, `pis_kg_search`
- [ ] Mỗi tool có `required` field trong `InputSchema`
- [ ] `GOWORK=off go build ./internal/mcp/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/mcp/...
```
