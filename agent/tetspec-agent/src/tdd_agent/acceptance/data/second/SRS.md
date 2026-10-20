# Payment gateway - software requirements (synthetic fixture)

## 3. Functional requirements
| ID | Statement | Rules | Acceptance |
|---|---|---|---|
| FR-PAY-01 | The system MUST capture an authorised payment within 5 seconds. | BRL-2 | AC-1 |
| FR-PAY-02 | The system MUST refund a captured payment up to its captured amount. | BRL-1 | AC-2 |
| FR-PAY-03 | The system SHALL expose the settlement status of a payment. | | AC-3 |

## 4. Entities
| ID | Name | Description | Attributes |
|---|---|---|---|
| ENT-1 | Payment | A captured or authorised payment | payment_id, amount, currency, status, idempotency_key |
| ENT-2 | Refund | A refund of a payment | refund_id, payment_id, amount |

**ENT-1 Payment.** Status: Authorised, Captured, Refunded.

| From | To | Event | Guard | Side effects |
|---|---|---|---|---|
| Authorised | Captured | capture | amount > 0 | emit event |
| Captured | Refunded | refund | refund <= captured | emit event |

## 5. Interfaces
| ID | Name | Satisfies | Auth | Request | Response | Errors |
|---|---|---|---|---|---|---|
| API-1 | POST /v1/payments/capture | FR-PAY-01 | bearer token | payment_id, amount | payment | 400, 409 |

## 6. Errors
| HTTP | Code | Meaning |
|---|---|---|
| 409 | `duplicate_capture` | The idempotency key was already used |
| 400 | `invalid_amount` | The amount is not positive |

Retries of a failed capture shall happen 3 times with a delay of 10 seconds.
