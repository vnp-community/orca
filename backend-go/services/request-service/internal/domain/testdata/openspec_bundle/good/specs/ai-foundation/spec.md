## ADDED Requirements

### Requirement: FR-SYS-07 AI conversation answers for real, not a 501 stub
FR-SYS-07 SHALL let a signed-in user send a chat turn through `POST /ai/conversation/chat` against a configured,
enabled AI model source and SHALL persist the turn so it can be listed back through
`POST /ai/conversation/chat/list`, replacing the previous truthful-501 behavior for a connected model source.

#### Scenario: AC-AI-01 chat through a configured OpenAI-compatible source is answered and listed
- **GIVEN** `AI_ENABLED` is on and an OpenAI-compatible model source (a mock-ext test provider) is configured and enabled
- **WHEN** a user sends a chat turn through `POST /ai/conversation/chat`
- **THEN** the user receives the provider's reply and the turn is stored, and both the user's and the reply's turns
  appear in `POST /ai/conversation/chat/list`

#### Scenario: AC-AI-02 chat through an approved real LLM succeeds
- **GIVEN** an approved real LLM (OpenAI, DeepSeek or ZhiPu) is configured as the resolved model source
- **WHEN** a user sends a chat turn through `POST /ai/conversation/chat`
- **THEN** the user receives a successful reply

### Requirement: AI_ENABLED gates every AI conversation route
Every route under `/ai/conversation/*` (`add`, `chat`, `update`, `list`, `delete`, `chat/list`) SHALL check the
`AI_ENABLED` flag inside `AIService` before doing any other work, and SHALL answer `503 Service Unavailable` on every
call while the flag is off.

#### Scenario: AC-AI-05 AI disabled answers 503 on every conversation route
- **GIVEN** `AI_ENABLED` is off
- **WHEN** any `/ai/conversation/*` route is called
- **THEN** the response is `503 Service Unavailable` and no conversation row, message row or provider call is made

### Requirement: Provider API keys are never exposed in clear
An AI model source's API key SHALL be stored sealed (AES-GCM, `internal/secret`) and SHALL never appear in clear in
a UI response, an API response, an audit log or an application log, matching the existing masking of
`ai_model_source.app_key`.

#### Scenario: AC-AI-03 API key stays masked everywhere it could leak
- **GIVEN** a model source has a configured API key
- **WHEN** the source is read back through any API, shown in a UI screen, or any request involving it is logged
- **THEN** the key is never present in clear in the response body or the log/span/audit output

### Requirement: An organization's exhausted AI quota blocks further chat
`AIService.AppendTurn` SHALL check the calling organization's `ai_usage_counter` against its configured limit before
calling a provider, and SHALL refuse the request with a clear quota-exhausted message, without calling the provider,
once the limit is reached.

#### Scenario: AC-AI-04 exhausted organization quota refuses the chat without calling the provider
- **GIVEN** an organization has exhausted its configured AI quota
- **WHEN** a user of that organization sends a chat turn through `POST /ai/conversation/chat`
- **THEN** the user receives a clear quota-exhausted message and no outbound call to the model provider is made

### Requirement: aiclient retries a transient provider error honoring Retry-After
`shared/pkg/aiclient` SHALL retry a provider call at most 2 times when the provider answers a transient error with a
`Retry-After` header, waiting the duration the header names (delta-seconds or an HTTP date) between attempts, and
SHALL return the provider's error once the retries are exhausted.

#### Scenario: AC-AI-06 provider with Retry-After is retried twice then fails
- **GIVEN** a provider answers a transient error with a `Retry-After` header on every attempt
- **WHEN** `aiclient` calls the provider
- **THEN** it waits as `Retry-After` names and retries at most 2 times, then returns the provider's error

### Requirement: aiclient blocks requests to disallowed internal network targets
`shared/pkg/aiclient` SHALL route every outbound provider call through `shared/pkg/netguard` (`AdminTarget` profile)
and SHALL refuse a call whose resolved address falls in a blocked internal/loopback/link-local/metadata range.

#### Scenario: AC-AI-07 provider address resolving to a blocked internal range is refused
- **GIVEN** a model source's `api_url` resolves to an address in a disallowed internal network range
- **WHEN** `aiclient` attempts to call that provider
- **THEN** the call is blocked before any request reaches the network

### Requirement: Model source resolution follows project, then organization, then system default
`AIService.ResolveModel` SHALL resolve the `ai_model_source` a chat uses by checking `ai_model_scope_config` for the
calling project first, then the calling organization, then the system-default scope (both project and organization
null), and SHALL use the first match found.

#### Scenario: A project-level scope config resolves ahead of organization and system default
- **GIVEN** `ai_model_scope_config` has a row for the calling project, a different row for its organization, and a
  system-default row
- **WHEN** `AIService.ResolveModel` is called for that project
- **THEN** the project row's `ai_model_source` is used
