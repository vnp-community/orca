# LedgerLite - User Requirements Document (URD) v1.0

LedgerLite serves the payments-operations team of a small retail lender. This URD states what the business and its users need, before any solution is chosen. The SRS (`docs/SRS.md`) turns each need into testable requirements.

## 1. Business objectives

| ID | Objective | Indicator |
| --- | --- | --- |
| BO-01 | Customers can move money between accounts without losing or creating money. | 0 unbalanced postings and 0 negative balances on non-overdraft accounts in the daily reconciliation. |
| BO-02 | A client that retries a payment never pays twice. | 0 duplicate postings per idempotency key. |
| BO-03 | Risk exposure per account is bounded each day. | 0 accounts that sent more than their daily limit in a UTC day. |
| BO-04 | Every movement of money can be reconstructed afterwards. | 100% of balances re-derivable from the journal. |

## 2. User requirements

| ID | Requirement | Actor | Priority |
| --- | --- | --- | --- |
| REQ-001 | A teller can open an account for a customer, optionally with an opening deposit, and decide whether it may overdraw. | Teller | Must |
| REQ-002 | A teller can see the current balance of an account. | Teller | Must |
| REQ-003 | A customer can transfer money to another account, and sending the same payment request twice has the effect of sending it once. | Customer | Must |
| REQ-004 | A risk officer can give each account a daily limit on what it may send. | Risk officer | Must |
| REQ-005 | An auditor can list every movement, of the whole ledger or of one account, and look up one transfer. | Auditor | Must |
| REQ-006 | Operations can check that the service is alive. | Operator | Should |
| REQ-007 | A customer can obtain a statement of an account for a date range, with a running balance. Planned, not part of v1.0. | Customer | Could |

## 3. Assumptions

- All accounts are in one currency; amounts are integers of its minor unit.
- Callers are trusted internal systems; authentication is handled in front of the service.
