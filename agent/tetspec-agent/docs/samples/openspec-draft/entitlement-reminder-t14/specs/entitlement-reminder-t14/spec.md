## ADDED Requirements

### Requirement: Subscription Entitlement reminder milestones
The system SHALL alert subscription Entitlements at T-60, T-30, T-14, T-7 and T-0, sending each milestone exactly once.

#### Scenario: T-14 reminder is sent once
- **GIVEN** a subscription Entitlement that expires in 14 days
- **WHEN** the daily job runs
- **THEN** exactly one T-14 reminder is sent
