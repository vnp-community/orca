# TASK-011: Tạo file `src/shared/agent-detection-commands.ts`

**Phase:** 1 — Remote Agent Detection  
**Solution:** [SOL-003](../solutions/SOL-003-remote-agent-detection.md) §5  
**Depends on:** (không có — pure type/util file)  
**Blocks:** TASK-013, TASK-014

---

## Mục tiêu

Tạo file shared chứa `AgentDetectionCommand` type và hàm `buildAgentDetectionCommands()` để convert từ agent catalog sang format relay có thể xử lý.

---

## File cần tạo

**Path:** `src/shared/agent-detection-commands.ts`

---

## Context cần tra cứu trước khi implement

Tìm agent catalog hiện tại:
- Grep `TUI_AGENT_CONFIG` hoặc `AGENT_CATALOG` trong `src/shared/` và `src/renderer/`
- Xác định cấu trúc của catalog entry: `id`, `command`, `requiredCommands`, `unsupportedRuntimes`

---

## Nội dung cần implement

```typescript
// Pure type file — không import UI-only fields

export type AgentDetectionCommand = {
  id: string
  cmd: string
  requiredCommands?: readonly string[]
  unsupportedRuntimes?: readonly ('darwin' | 'win32' | 'linux' | 'wsl')[]
}

/**
 * Builds agent detection commands từ catalog.
 * Chỉ include fields cần thiết cho relay — KHÔNG include UI-only fields
 * (label, description, icon, installUrl, etc.)
 */
export function buildAgentDetectionCommands(): AgentDetectionCommand[] {
  // Import catalog raw data (thay thế bằng đúng import path sau khi tra cứu):
  // import { AGENT_CATALOG_RAW } from './tui-agent-config'  // hoặc tương đương
  return AGENT_CATALOG_RAW.map(entry => ({
    id: entry.id,
    cmd: entry.command,
    requiredCommands: entry.requiredCommands,
    unsupportedRuntimes: entry.unsupportedRuntimes
  }))
}
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/shared/agent-detection-commands.ts`
- [x] `AgentDetectionCommand` type được export
- [x] `buildAgentDetectionCommands()` được export và trả về array đúng format
- [x] Không include UI-only fields (label, description, icon, installUrl) trong output
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

1. Dùng `grep -r "TUI_AGENT_CONFIG\|AGENT_CATALOG\|agentCatalog" src/` để tìm catalog
2. Đọc file catalog để hiểu structure thực tế
3. Chỉ map các fields cần thiết để relay detect agent
