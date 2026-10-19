## ADDED Requirements

### Requirement: Model sources bind to a user, project, organisation or system

The system SHALL extend `ai_model_source` with nullable `organization_id` and `project_id` so a model source
can be bound to one user, one project, one organisation, or the whole system, and SHALL resolve the model for
a chat turn in the order personal → project → organisation → system, taking only sources whose `status` is
enabled. The `ownerType` value SHALL be compared after `TrimSpace`, because the SPA's enum carries a trailing
space in `SYSTEM `. A caller with no resolvable source SHALL receive 422 with a plain Vietnamese message.

#### Scenario: The narrowest binding wins
- **GIVEN** an enabled personal source, an enabled project source and an enabled system source available to
  the caller
- **WHEN** a chat turn is sent without an explicit `chatModelId`
- **THEN** the personal source is used

#### Scenario: A project-bound source serves that project only
- **GIVEN** a source bound to project P of organisation O
- **WHEN** a member of organisation O who is not in project P resolves a model
- **THEN** the project-bound source is not selected, and resolution falls through to the organisation and
  system sources

#### Scenario: A disabled source is never selected
- **GIVEN** the only matching source has `status` disabled
- **WHEN** a chat turn is sent
- **THEN** the response is 422 with a message that no AI model is configured, and no provider is called

#### Scenario: `ownerType` with a trailing space still matches
- **GIVEN** a source stored with `ownerType` `SYSTEM ` as the SPA enum writes it
- **WHEN** the resolver compares the owner type
- **THEN** it matches `SYSTEM` after trimming, and the permission check applies as for a system source

### Requirement: The API key is encrypted at rest and never returned in clear (AC-3)

The system SHALL store the model source's API key as `enc:v1:` AES-256-GCM with the key taken from the
environment, as `INT-14`, `BRL-46` and `NFR-SE-04` require. Every read path — the UI, every API response, every
log line, trace span, audit record and metric label — SHALL expose only a masked value. The clear key SHALL
exist only in memory inside ms-system-setting, between decryption and the `Authorization` header of the
provider call, and SHALL NOT cross a service boundary in any form.

#### Scenario: AC-3 the key is not in any API response
- **GIVEN** a model source saved with an API key
- **WHEN** the source is read back through the system, organisation, project and personal list and detail
  routes
- **THEN** every response carries a masked value and no response contains the clear key

#### Scenario: AC-3 the key is not in the stored row in clear
- **GIVEN** a model source saved with an API key
- **WHEN** the row is read directly from `ai_model_source`
- **THEN** the key column holds an `enc:v1:` ciphertext

#### Scenario: AC-3 the key is not in logs, spans or audit records
- **GIVEN** a model source is created, edited and then used for a chat turn that fails
- **WHEN** the service logs, trace spans, operation-log rows and metric labels produced by those actions are
  inspected
- **THEN** none of them contains the clear key or any substring of it beyond the fixed mask

#### Scenario: AC-3 a save that omits the key keeps the stored one
- **GIVEN** an existing source whose key the UI shows masked
- **WHEN** the user saves the form back with the masked placeholder unchanged
- **THEN** the stored ciphertext is kept and the mask is not written over the real key

### Requirement: Only ms-system-setting holds and uses AI provider keys

The system SHALL keep AI provider keys in ms-system-setting alone and SHALL make the outbound AI call from
there. Other services SHALL ask for a completion through `AIService.Chat` on the internal gRPC channel rather
than for a key; `AIService.ResolveModel` SHALL return provider metadata with the key masked. The allowed
callers of each method SHALL be listed in `backend/shared/proto/tms/identity/v1/acl.yaml`, the register
`grpcx.ParseACL` enforces, and no other service SHALL hold or persist an AI key. This keeps a single egress
path for AI as `ADR-15` requires, and is the reading of `ADR-09` recorded in design.md.

#### Scenario: An unauthorised caller cannot resolve a model
- **GIVEN** a service that is not on the method's caller list in `acl.yaml`
- **WHEN** it calls `ResolveModel` or `Chat`
- **THEN** the call is rejected by the gRPC ACL and no key, masked or clear, is returned

#### Scenario: No other service stores the key
- **WHEN** the repository is checked for AI key columns or AI key environment variables outside
  ms-system-setting
- **THEN** none is found
