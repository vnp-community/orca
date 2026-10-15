## ADDED Requirements

### Requirement: Per-organisation AI quota and cost ceiling

The system SHALL hold, per organisation and per calendar-month period, a token limit and an optional cost
ceiling in `ai_org_budget(organization_id, period, token_limit, cost_limit_micro)`, and SHALL record every
completed turn in an append-only `ai_usage_ledger`. Usage SHALL be counted in the token numbers the provider
reports; the cost SHALL be derived from a static price table per provider and model. An organisation with no
quota row SHALL use the platform default.

#### Scenario: A quota row and a default
- **GIVEN** organisation A has a quota row for the current month and organisation B has none
- **WHEN** a member of each sends a chat turn
- **THEN** A is measured against its own limits and B against the platform default, and both decisions are
  taken from the current calendar month's period

#### Scenario: A new month starts a new period
- **GIVEN** an organisation that exhausted its token limit last month
- **WHEN** the calendar month changes and a member sends a chat turn
- **THEN** the usage counted is that of the new period and the turn is allowed

### Requirement: The quota is checked before any provider call (AC-4)

The system SHALL run the quota check before resolving the model and before any provider call, in the same
database as the ledger. When the organisation is over its token limit or its cost ceiling the request SHALL be
refused with HTTP 429 and a plain Vietnamese message naming the exhausted quota, and no provider request SHALL
be made and no key decrypted.

#### Scenario: AC-4 an exhausted organisation gets a clear message and no provider call
- **GIVEN** an organisation whose AI quota for the current period is exhausted
- **WHEN** a member of it posts a turn to `POST /ai/conversation/chat`
- **THEN** the response is HTTP 429 with a plain Vietnamese message that the organisation's AI quota is used
  up, the provider receives no request at all, and no assistant turn is stored

#### Scenario: AC-4 the cost ceiling refuses the turn as the token limit does
- **GIVEN** an organisation still under its token limit but over its cost ceiling for the period
- **WHEN** a member posts a chat turn
- **THEN** the response is HTTP 429 with the exhausted-quota message and the provider is not called

#### Scenario: AC-4 the check runs before model resolution
- **GIVEN** an organisation over its quota and a model source whose key would need decrypting
- **WHEN** a member posts a chat turn
- **THEN** the refusal happens before the model is resolved, so no key is decrypted

### Requirement: Usage is recorded in the same transaction as the turn

After a successful provider call the system SHALL insert the ledger row and update the period counter in the
same transaction as the `ai_conversation_message` rows, using `backend/shared/pkg/uow`, so a stored turn can
never be unmetered and a metered turn can never be unstored. The ledger SHALL be append-only: no update, no
delete.

#### Scenario: A successful turn is stored and metered atomically
- **WHEN** a chat turn completes and the transaction commits
- **THEN** the two message rows, one ledger row and the updated period counter are all present

#### Scenario: A failed commit leaves no partial state
- **GIVEN** the ledger insert fails
- **WHEN** the turn's transaction is rolled back
- **THEN** no message row and no counter change remain for that turn

### Requirement: The organisation of a turn is the caller's own

The system SHALL take the organisation of a chat turn from the `organizationId` the SPA already sends on the
chat request and SHALL verify the caller's membership of it before counting any usage against it. A request
naming an organisation the caller does not belong to SHALL be refused and SHALL NOT consume another
organisation's quota.

#### Scenario: A non-member cannot spend an organisation's quota
- **GIVEN** a caller who is not a member of organisation A
- **WHEN** the caller posts a chat turn carrying organisation A's id
- **THEN** the request is refused, no provider call is made and A's ledger is unchanged
