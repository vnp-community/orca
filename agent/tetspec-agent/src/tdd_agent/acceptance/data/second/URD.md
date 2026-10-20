# Payment gateway - user requirements (synthetic fixture)

## 1. Business rules
| ID | Rule |
|---|---|
| BRL-1 | A refund must never exceed the captured amount. |
| BRL-2 | A payment must be captured at most once per idempotency key. |

## 2. Requirements
| ID | Statement |
|---|---|
| REQ-1 | Merchants must be able to refund a captured payment. |
| REQ-2 | Merchants must see the settlement status of every payment. |
