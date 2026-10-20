## ADDED Requirements

### Requirement: BRL-48 Cảnh báo Entitlement theo mốc
Hệ thống SHALL cảnh báo Entitlement tại T-60, T-30, T-14, T-7, T-0 bằng job hằng ngày và Alert `entitlement_expiring`.

#### Scenario: Nhắc T-14 theo BRL-48
- **GIVEN** một Entitlement thuê bao hết hạn sau 14 ngày
- **WHEN** job hằng ngày chạy
- **THEN** gửi đúng một nhắc nhở T-14

### Requirement: FR-AST-13 Cảnh báo Entitlement thuê bao
Hệ thống SHALL cảnh báo Entitlement thuê bao tại T-60, T-30, T-14, T-7, T-0 và mỗi mốc SHALL được gửi đúng một lần (`reminded_milestones`).

#### Scenario: Nhắc T-14 theo FR-AST-13
- **GIVEN** một Entitlement thuê bao hết hạn sau 14 ngày
- **WHEN** job hằng ngày chạy
- **THEN** gửi đúng một nhắc nhở T-14
