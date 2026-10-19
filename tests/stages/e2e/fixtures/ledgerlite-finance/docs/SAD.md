# LedgerLite - Software Architecture Document (SAD) v1.0

This document describes the architecture that realises `docs/SRS.md`. Requirement ids (FR, BR, NFR) and entity ids (ENT) are defined in the SRS and only referenced here.

## 1. Introduction

LedgerLite is one Go process. A thin HTTP layer calls a ledger service that validates and posts transfers into a store behind an interface. The store is in memory in v1.0.

Id conventions of this document: SVC component, ADR architecture decision.

## 2. Architectural drivers

The decisions below are driven by four qualities, in order: correctness of money (BR-001, BR-002, BR-004), concurrency safety (NFR-002), simplicity of deployment (CT-001, NFR-003) and the ability to swap the store later without touching business rules.

## 3. Components

| ID | Component | Responsibility | Package | Satisfies |
| --- | --- | --- | --- | --- |
| SVC-001 | HTTP API | Decode requests, call the ledger service, map domain errors to HTTP status and error codes, log each request. | `internal/httpapi` | FR-007, NFR-004, NFR-005, NFR-006 |
| SVC-002 | Ledger service | Validate transfers, apply the business rules, serialize writes with one mutex and post balanced entries. | `internal/ledger/service.go` | FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, BR-002, BR-003, BR-004, BR-006, NFR-002 |
| SVC-003 | Store | Atomic persistence port with an in-memory implementation: accounts, transfers, idempotency index and the append-only journal. | `internal/ledger/store.go` | BR-001, BR-005, NFR-001 |
| SVC-004 | Server binary | Parse flags, wire the components, serve HTTP and shut down gracefully. | `cmd/ledgerd` | CT-001 |

Dependencies: SVC-004 builds SVC-001 on top of SVC-002 on top of SVC-003. SVC-001 never touches the store directly.

## 4. Data model

The logical entities are ENT-001 Account, ENT-002 Transfer and ENT-003 JournalEntry (see SRS section 5). The balance of an account is materialised in the account record and updated in the same atomic step that appends the journal entries of the transfer, so reads never scan the journal.

The funding account `acc_funding` is created at start-up with the overdraft flag set and no daily limit. Initial deposits are transfers from it, which keeps the sum of balances at zero.

## 5. Runtime view: posting a transfer

1. The HTTP layer decodes the body (unknown fields and fractional amounts are rejected) and reads the Idempotency-Key header.
2. The service takes its write mutex.
3. It looks the key up: same request returns the stored transfer, a different request is a conflict.
4. It validates amount and accounts, then the overdraft rule, then the daily limit over the entries of the current UTC day.
5. It builds one transfer and two entries and calls the store, which re-checks that the entries sum to zero and applies everything under its own lock.

## 6. Architecture decisions

| ID | Decision | Rationale | Alternatives | Tradeoffs |
| --- | --- | --- | --- | --- |
| ADR-001 | Represent money as int64 minor units. | Exact arithmetic and trivial overflow-free comparisons for the amounts in scope; satisfies NFR-004. | float64 amounts; a decimal library. | One implicit currency and no sub-minor-unit precision; a currency change needs a new decision. |
| ADR-002 | Put the store behind a Store interface with an in-memory implementation. | Keeps business rules independent of persistence and makes tests fast and deterministic. | Embedded SQLite; direct maps in the service. | State is lost on restart; a durable store is future work. |
| ADR-003 | Serialize all writes with one service-level mutex. | Makes the check-then-post sequence of BR-002 and BR-003 race-free (NFR-002) with the least code. | Per-account locks; optimistic versioning. | Writes do not scale beyond one core; adequate for NFR-001. |
| ADR-004 | Draw deposits from a system funding account instead of creating money. | Keeps BR-001 universally true and the journal complete. | Allow single-leg credit entries. | The funding balance is a large negative number by design. |
| ADR-005 | Define the daily limit window as the UTC calendar day of the server clock. | Simple, deterministic and testable with an injected clock (BR-003). | Rolling 24 hours; account-local time zone. | Customers outside UTC see the reset at a non-midnight local time. |
| ADR-006 | Use only net/http with Go 1.22 pattern routing and log/slog. | No third-party dependencies (CT-001, NFR-003). | chi or gin; zap. | Hand-written JSON error mapping. |

## 7. Quality attributes

| Quality attribute | Tactic | Components | Requirements |
| --- | --- | --- | --- |
| Correctness | Balanced postings re-checked in the store; integer money. | SVC-002, SVC-003 | BR-001, NFR-004 |
| Concurrency safety | Single write mutex, race detector in tests. | SVC-002 | NFR-002 |
| Performance | Per-account index in the store so the limit check scans only today's entries of one account. | SVC-003 | NFR-001 |
| Operability | One structured log line per request, stable error codes. | SVC-001 | NFR-005, NFR-006 |

## 8. Deployment

One statically linked binary `ledgerd` (`go build ./cmd/ledgerd`), configured by flags `-addr` and `-daily-limit`. It can run in a container or directly on a host. There is no persistent volume; a restart empties the ledger, so v1.0 is suitable for demos, tests and as a reference service only.

## 9. Known gaps

- No durable store, no authentication, no pagination of the journal.
- Account statements and transfer fees are not designed yet.
