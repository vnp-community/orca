# LedgerLite - Software Requirements Specification (SRS) v1.0

LedgerLite is a small double-entry ledger service for a retail-finance team: it keeps customer accounts, moves money between them with idempotent transfers, enforces per-account limits and records every movement in an append-only journal. It is a single Go binary with an in-memory store.

## 1. Introduction

### 1.1. Purpose

This SRS states what LedgerLite must do, in testable terms, and names the code and the tests that realise each requirement. The architecture and decisions are in `docs/SAD.md`.

### 1.2. Scope

In scope for v1.0: accounts, transfers between accounts in a single currency, idempotency, overdraft and daily-limit rules, the journal, a JSON HTTP API.

Out of scope for v1.0 (planned or not planned):

- Account statements with running balance and date filters.
- Transfer fees and any currency other than the single implicit currency.
- Persistent storage: state is lost on restart.
- Authentication and authorisation of API callers.

### 1.3. Conventions

- Money is an integer count of minor units (for example cents). There is no floating point anywhere.
- A "day" is a UTC calendar day.
- Ids: FR functional requirement, BR business rule, NFR non-functional requirement, ENT entity, API operation, CT constraint. Component ids (SVC) and decisions (ADR) are defined in the SAD.
- Every requirement row lists where it is implemented and which tests verify it. Source files carry the requirement id in a comment next to the code.

## 2. Functional requirements

| ID | Statement | Traces to | Component | Implemented in | Verified by | Acceptance |
| --- | --- | --- | --- | --- | --- | --- |
| FR-001 | The system MUST create an account from an owner name, an overdraft flag, an optional daily limit and an optional initial deposit, and return its generated id. | REQ-001, REQ-004 | SVC-002 | `internal/ledger/service.go::Service.CreateAccount`; `internal/httpapi/handler.go::createAccount` | TestCreateAccountWithInitialDeposit; TestCreateAccountRejectsInvalidInput; TestCreateAndGetAccount | GIVEN owner alice and initial deposit 5000 WHEN the account is created THEN its balance is 5000 and the funding account is debited 5000. |
| FR-002 | The system MUST return an account with its current balance, and 404 account_not_found for an unknown id. | REQ-002 | SVC-002 | `internal/ledger/service.go::Service.Account`; `internal/httpapi/handler.go::getAccount` | TestCreateAndGetAccount | GIVEN an account with balance 2500 WHEN it is read THEN balance is 2500; GIVEN an unknown id THEN the response is 404. |
| FR-003 | The system MUST post each transfer as two journal entries, a debit of the source and a credit of the destination for the same amount, atomically. | REQ-003, BO-01 | SVC-002 | `internal/ledger/service.go::Service.transferLocked`; `internal/ledger/store.go::MemoryStore.Post` | TestTransferPostsBalancedEntries | GIVEN a with 1000 and b with 0 WHEN a transfers 400 to b THEN a has 600, b has 400 and the two entries sum to zero. |
| FR-004 | The system MUST require an Idempotency-Key on every transfer and, for a repeated key with the same request, return the original transfer without posting again. | REQ-003, BO-02 | SVC-002 | `internal/ledger/service.go::Service.transferLocked`; `internal/httpapi/handler.go::postTransfer` | TestIdempotency; TestTransferIdempotencyOverHTTP | GIVEN a transfer was made with key pay-1 WHEN the same request is sent again THEN the response is 200 with header Idempotent-Replayed true and no extra money moves. |
| FR-005 | The system MUST list the journal entries, all of them or those of one account, in append order. | REQ-005, BO-04 | SVC-002 | `internal/ledger/service.go::Service.Journal`; `internal/ledger/store.go::MemoryStore.EntriesSince`; `internal/httpapi/handler.go::getJournal` | TestTransferLookupJournalAndHealth | GIVEN one transfer of 200 into b WHEN the journal of b is read THEN it has one entry with delta 200. |
| FR-006 | The system MUST return a transfer by id, and 404 transfer_not_found for an unknown id. | REQ-005 | SVC-002 | `internal/ledger/service.go::Service.GetTransfer`; `internal/httpapi/handler.go::getTransfer` | TestTransferLookupJournalAndHealth | GIVEN a posted transfer WHEN it is read by id THEN amount, source and destination match the request. |
| FR-007 | The system MUST expose a liveness endpoint that answers 200 without touching the store. | REQ-006 | SVC-001 | `internal/httpapi/handler.go::New` | TestTransferLookupJournalAndHealth | GIVEN the server is running WHEN GET /healthz is called THEN status is ok. |

## 3. Business rules

| ID | Rule | Traces to | Enforced in | Verified by | Acceptance |
| --- | --- | --- | --- | --- | --- |
| BR-001 | The entries of every transfer MUST sum to zero, so the sum of all account balances is always zero. | BO-01 | `internal/ledger/store.go::MemoryStore.Post` | TestTransferPostsBalancedEntries; TestStoreRejectsUnbalancedPosting | GIVEN any sequence of transfers THEN the sum of all journal deltas is 0. |
| BR-002 | An account without the overdraft flag MUST NOT go below zero; a transfer that would do so is rejected with insufficient_funds. | BO-01 | `internal/ledger/service.go::Service.transferLocked` | TestInsufficientFundsAndOverdraft; TestTransferErrorMapping | GIVEN a balance of 100 and no overdraft WHEN 101 is sent THEN the transfer is rejected with 422 insufficient_funds. |
| BR-003 | The total sent by an account in one UTC day MUST NOT exceed its daily limit; a transfer that reaches the limit exactly is allowed, and received money does not count. | BO-03, REQ-004 | `internal/ledger/service.go::Service.sentToday`; `internal/ledger/service.go::Service.transferLocked` | TestDailyLimitIsInclusiveAndResetsAtUTCMidnight; TestDailyLimitCountsOnlyOutgoing; TestDailyLimitOverHTTP | GIVEN a limit of 1000 and 600 already sent WHEN 400 is sent THEN it succeeds; WHEN 1 more is sent THEN it is rejected with 422 daily_limit_exceeded. |
| BR-004 | An idempotency key MUST be bound to exactly one source, destination and amount; reusing it with a different request MUST be rejected with 409 idempotency_conflict. | BO-02 | `internal/ledger/service.go::Service.transferLocked` | TestIdempotency; TestTransferIdempotencyOverHTTP | GIVEN key pay-1 used for amount 300 WHEN it is used with amount 301 THEN the response is 409. |
| BR-005 | The journal MUST be append-only: no entry is ever updated or deleted, and readers receive copies. | BO-04 | `internal/ledger/store.go::Store` interface | TestJournalIsAppendOnly | GIVEN a returned journal slice WHEN a caller edits it THEN a later read is unchanged. |
| BR-006 | An amount MUST be a positive integer of minor units and the source and destination MUST differ. | REQ-003 | `internal/ledger/service.go::Service.transferLocked` | TestTransferValidation; TestTransferErrorMapping | GIVEN amount 0 or source equal to destination THEN the transfer is rejected with 400. |

## 4. Non-functional requirements

| ID | Target | Measure | Verified by | Acceptance |
| --- | --- | --- | --- | --- |
| NFR-001 | Transfer throughput of at least 10000 transfers per second on one core with the in-memory store and 100 sender accounts. | Go benchmark ns per op at most 100000 | BenchmarkTransfer | GIVEN `go test ./internal/ledger -bench Transfer` THEN ns/op is at most 100000. |
| NFR-002 | Concurrent transfers MUST never break BR-001 or BR-002. | Race detector clean, invariants hold after 200 concurrent transfers | TestConcurrentTransfersKeepInvariants | GIVEN `go test -race ./...` THEN all tests pass. |
| NFR-003 | The service MUST depend only on the Go standard library. | go.mod has no require directive | go.mod inspection | GIVEN `go list -m all` THEN only the main module is listed. |
| NFR-004 | Money MUST never be handled as floating point; fractional amounts MUST be rejected at the API. | Decoder rejects 10.5 for an int64 field | TestTransferErrorMapping | GIVEN amount 10.5 WHEN posted THEN the response is 400 invalid_json. |
| NFR-005 | Every HTTP request MUST be logged as one structured line with method, path, status and duration in milliseconds. | slog JSON output | code inspection of `internal/httpapi/handler.go::logRequests` | GIVEN any request THEN one log line with those four fields is written. |
| NFR-006 | Every error response MUST carry a stable machine-readable code in `error.code`. | Table of error codes | TestTransferErrorMapping; TestDailyLimitOverHTTP | GIVEN a rejected transfer THEN `error.code` is one of the documented codes. |

## 5. Data entities

| ID | Name | Attributes | Description |
| --- | --- | --- | --- |
| ENT-001 | Account | id, owner, balance, allow_overdraft, daily_limit, created_at | A holder of a balance. daily_limit 0 means unlimited; only the funding account has it. |
| ENT-002 | Transfer | id, idempotency_key, from, to, amount, created_at | A request to move money, unique per idempotency key. |
| ENT-003 | JournalEntry | seq, transfer_id, account_id, delta, at | One signed leg of a transfer; seq is global and strictly increasing. |

## 6. External interfaces

| ID | Name | Operation | Auth | Request | Response | Errors | Idempotency | Concurrency | Satisfies |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| API-001 | Create account | POST /accounts | none | JSON owner, allow_overdraft, daily_limit, initial_deposit | 201 account | 400 invalid_account | Not idempotent: every call creates a new account. | Writes are serialized by the service mutex (ADR-003). | FR-001 |
| API-002 | Get account | GET /accounts/{id} | none | path id | 200 account | 404 account_not_found | n/a | n/a | FR-002 |
| API-003 | Create transfer | POST /transfers | none | header Idempotency-Key; JSON from, to, amount | 201 transfer, or 200 on replay | 400 invalid_amount, same_account, idempotency_key_required, invalid_json; 404 account_not_found; 409 idempotency_conflict; 422 insufficient_funds, daily_limit_exceeded | Idempotent by Idempotency-Key (FR-004, BR-004). | Check-then-post runs under the service mutex (ADR-003, NFR-002). | FR-003, FR-004, BR-002, BR-003 |
| API-004 | Get transfer | GET /transfers/{id} | none | path id | 200 transfer | 404 transfer_not_found | n/a | n/a | FR-006 |
| API-005 | List journal | GET /journal | none | optional query account | 200 entries list | none | n/a | n/a | FR-005 |
| API-006 | Liveness | GET /healthz | none | none | 200 status ok | none | n/a | n/a | FR-007 |

## 7. Constraints

| ID | Constraint | Rationale |
| --- | --- | --- |
| CT-001 | The code MUST be Go 1.22 or later using only the standard library. | Smallest possible deployment and supply-chain surface. |

## 8. Verification

Each row is one Go test or benchmark (tests are named, not numbered; the requirement tables above carry the same links). Run them all with `go test -race ./...`; the benchmark with `go test ./internal/ledger -bench Transfer`.

| Test | Level | Kind | Expected | Automation | Covers | Location |
| --- | --- | --- | --- | --- | --- | --- |
| TestCreateAccountWithInitialDeposit | unit | functional | Balance equals the deposit and the funding account is debited by it. | automated | FR-001 | `internal/ledger/service_test.go` |
| TestCreateAccountRejectsInvalidInput | unit | functional | Blank owner, negative limit and negative deposit are rejected. | automated | FR-001 | `internal/ledger/service_test.go` |
| TestTransferPostsBalancedEntries | unit | functional | Two entries that sum to zero and the expected balances. | automated | FR-003, BR-001 | `internal/ledger/service_test.go` |
| TestInsufficientFundsAndOverdraft | unit | functional | Overdraft is refused unless the account allows it. | automated | BR-002 | `internal/ledger/service_test.go` |
| TestTransferValidation | unit | functional | Zero, negative, same-account, unknown-account and missing-key requests are rejected. | automated | BR-006, FR-004 | `internal/ledger/service_test.go` |
| TestIdempotency | unit | functional | A replay returns the same transfer and moves no money; a changed amount conflicts. | automated | FR-004, BR-004 | `internal/ledger/service_test.go` |
| TestDailyLimitIsInclusiveAndResetsAtUTCMidnight | unit | functional | Reaching the limit exactly is allowed, one more is refused, the next UTC day resets. | automated | BR-003 | `internal/ledger/service_test.go` |
| TestDailyLimitCountsOnlyOutgoing | unit | functional | Received money does not count against the limit. | automated | BR-003 | `internal/ledger/service_test.go` |
| TestJournalIsAppendOnly | unit | functional | Editing a returned slice does not change the journal. | automated | BR-005, FR-005 | `internal/ledger/service_test.go` |
| TestStoreRejectsUnbalancedPosting | unit | functional | The store refuses entries that do not sum to zero. | automated | BR-001 | `internal/ledger/service_test.go` |
| TestConcurrentTransfersKeepInvariants | unit | non_functional | 200 concurrent transfers leave balances non-negative and conserved. | automated | NFR-002 | `internal/ledger/service_test.go` |
| BenchmarkTransfer | unit | performance | At most 100000 ns per transfer. | automated | NFR-001 | `internal/ledger/service_test.go` |
| TestCreateAndGetAccount | integration | functional | 201 with the id, 200 with the balance, 404 for unknown, 400 for blank owner. | automated | FR-001, FR-002, API-001, API-002 | `internal/httpapi/handler_test.go` |
| TestTransferIdempotencyOverHTTP | integration | functional | 201, then 200 with Idempotent-Replayed, then 409, and 400 without a key. | automated | FR-004, BR-004, API-003 | `internal/httpapi/handler_test.go` |
| TestTransferErrorMapping | integration | functional | Fractional and unknown fields give 400 invalid_json; business errors map to their codes. | automated | NFR-004, NFR-006, BR-006, BR-002 | `internal/httpapi/handler_test.go` |
| TestDailyLimitOverHTTP | integration | functional | Exactly the limit gives 201, one more gives 422 daily_limit_exceeded. | automated | BR-003, API-003 | `internal/httpapi/handler_test.go` |
| TestTransferLookupJournalAndHealth | integration | functional | Transfer lookup, journal listing, empty journal and liveness. | automated | FR-005, FR-006, FR-007, API-004, API-005, API-006 | `internal/httpapi/handler_test.go` |
