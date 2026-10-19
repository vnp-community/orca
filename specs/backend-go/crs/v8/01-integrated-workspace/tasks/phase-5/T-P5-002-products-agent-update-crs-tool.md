---
task_id: T-P5-002
phase: 5
cr_id: CR-ORCA-003
solution_ref: solutions/CR-ORCA-003-solution.md
service: products-agent
target_files:
  - products-agent/internal/catalog/kg_tools.go
  - products-agent/internal/kg/handler.go
  - products-agent/internal/dispatcher/dispatcher.go
priority: P2
status: done
depends_on: T-P5-001
estimated_effort: 2–3 giờ
blocks: T-P5-003
---

# T-P5-002 — Thêm pis_update_crs_status tool vào products-agent

## Context

`products-agent` cần expose `pis_update_crs_status` tool để `orca-agent` có thể peer-invoke và update CRS status. Tool này gọi `PATCH /api/v1/kg/crs/{id}/status` trên `pikb-catalog-service`.

## Target files

```
products-agent/internal/catalog/kg_tools.go    ← thêm tool definition
products-agent/internal/kg/handler.go          ← thêm UpdateCRSStatus() method
products-agent/internal/dispatcher/dispatcher.go ← thêm routing case
```

## Instructions

### 1. `products-agent/internal/catalog/kg_tools.go`

Thêm tool definition cho `pis_update_crs_status`:

```go
{
    Name:        "pis_update_crs_status",
    Description: "Update CRS node status trong PIKB KG. Dùng sau khi implementation hoàn thành và convention verify passed.",
    IsWrite:     true,
    RiskLevel:   "medium",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "crs_id": map[string]any{
                "type":        "string",
                "description": "CRS ID, vd: CRS-045",
            },
            "status": map[string]any{
                "type": "string",
                "enum": []string{"pending", "in-progress", "done", "cancelled"},
            },
            "updated_by": map[string]any{
                "type":        "string",
                "description": "Agent ID hoặc user ID thực hiện update",
            },
        },
        "required": []string{"crs_id", "status"},
    },
},
```

### 2. `products-agent/internal/kg/handler.go`

Thêm `UpdateCRSStatus()` method:

```go
// UpdateCRSStatus calls pikb-catalog PATCH /api/v1/kg/crs/{id}/status.
// CR-ORCA-003: write path — validate status trước khi call.
func (h *KGHandler) UpdateCRSStatus(ctx context.Context, crsID, status, updatedBy string) error {
    body, err := json.Marshal(map[string]any{
        "status":     status,
        "updated_by": updatedBy,
    })
    if err != nil {
        return fmt.Errorf("UpdateCRSStatus: marshal: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
        h.catalogBaseURL+"/api/v1/kg/crs/"+url.PathEscape(crsID)+"/status",
        bytes.NewReader(body),
    )
    if err != nil {
        return fmt.Errorf("UpdateCRSStatus: build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    if h.serviceToken != "" {
        req.Header.Set("Authorization", "Bearer "+h.serviceToken)
    }

    resp, err := h.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("UpdateCRSStatus %s: %w", crsID, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusNoContent {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("UpdateCRSStatus %s: HTTP %d: %s", crsID, resp.StatusCode, b)
    }
    return nil
}
```

### 3. `products-agent/internal/dispatcher/dispatcher.go`

Thêm routing case cho `pis_update_crs_status` trong switch:

```go
case "pis_update_crs_status":
    crsID, _ := frame.Params["crs_id"].(string)
    status, _ := frame.Params["status"].(string)
    updatedBy, _ := frame.Params["updated_by"].(string)
    if crsID == "" || status == "" {
        err = fmt.Errorf("pis_update_crs_status: crs_id and status required")
        break
    }
    err = d.kgClient.UpdateCRSStatus(ctx, crsID, status, updatedBy)
    if err == nil {
        result = map[string]any{
            "updated":  true,
            "crs_id":   crsID,
            "status":   status,
        }
    }
```

## Acceptance criteria

- [ ] `pis_update_crs_status` xuất hiện trong catalog của products-agent
- [ ] `KGHandler.UpdateCRSStatus()` gọi đúng `PATCH /api/v1/kg/crs/{id}/status`
- [ ] Dispatcher route `pis_update_crs_status` tồn tại
- [ ] Missing params (`crs_id` hoặc `status`) → error rõ ràng
- [ ] `go build ./...` trong products-agent thành công

## Test command

```bash
cd products-agent  # hoặc path tương đương
go build ./...
```
