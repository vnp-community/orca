> DRAFT: see trace-manifest.json for blockers.

## Why

Entitlement thuê bao trước đây chỉ được cảnh báo tại T-60, T-30, T-7, T-0; CR-PBOM-1 yêu cầu thêm mốc nhắc T-14.

## What Changes

- Sửa `internal/entitlement/reminder.go::Milestones` để thêm mốc T-14.
- BRL-48 đổi thành: Cảnh báo Entitlement T-60, T-30, T-14, T-7, T-0.
- FR-AST-13 đổi thành: Hệ thống phải cảnh báo Entitlement thuê bao tại T-60, T-30, T-14, T-7, T-0.

## Capabilities

### New Capabilities
- `add-t14-reminder`: CR-PBOM-1: Thêm mốc nhắc T-14 cho Entitlement thuê bao.

### Modified Capabilities

## Impact

- Owner của capability: pbom-team.
- Purpose và scope: CR-PBOM-1: Thêm mốc nhắc T-14 cho Entitlement thuê bao.
- Code target: `internal/entitlement/reminder.go::Milestones`.
