# Pilot metrics for Request 0220d370-6f65-4842-9e55-d9d94f07f2b3

## Analysis and plan runs

| run | kind | engine | status | seconds | turns | tool calls | KG calls | cost USD |
|---|---|---|---|---|---|---|---|---|
| 1f132c10 | solution | native | failed REQUEST_SOLUTION_INTERNAL_ERROR | 473 |  |  |  |  |
| 5f7ca2fc | solution | native | succeeded | 428 | 79 | 78 | 21 | 4.45 |
| b5abf8a9 | solution | native | succeeded | 414 | 59 | 58 | 23 | 2.89 |
| 9734d599 | plan | openspec | failed REQUEST_OPENSPEC_INVALID_OUTPUT | 1548 | 233 | 231 | 46 | 12.73 |
| f1c99151 | plan | openspec | failed REQUEST_OPENSPEC_AGENT_FAILED | 0 |  |  |  |  |
| 124adc3e | plan | openspec | failed REQUEST_OPENSPEC_TIMEOUT | 901 |  | 136 | 25 |  |
| 1a38ae54 | plan | openspec | failed REQUEST_ENGINE_NO_CONNECTION | 0 |  |  |  |  |
| 47a6bcdd | plan | openspec | failed REQUEST_OPENSPEC_INVALID_OUTPUT | 1330 | 215 | 213 | 50 | 13.19 |
| 73d8af7b | plan | openspec | succeeded | 797 | 128 | 127 | 36 | 7.35 |
| bbf195d9 | solution | native | succeeded | 368 | 49 | 48 | 21 | 2.57 |
| 1a12a5bb | plan | openspec | succeeded | 543 | 118 | 117 | 27 | 7.38 |

Total reported by the agent for these runs: **$50.57**. Task runs report no cost yet.

## Tasks

| phase | task | status | attempts | failures |
|---|---|---|---|---|
| The six `/ai/conversation/*` route | Implement add, update, list and delete with client-id idempo | done | 1 |  |
| The six `/ai/conversation/*` route | Add the AI_ENABLED flag and the subtree middleware that answ | done | 1 |  |
| The six `/ai/conversation/*` route | Add the handler integration tests against the local stack mo | done | 1 |  |
| The six `/ai/conversation/*` route | Implement chat/list and mount the six routes on the router | done | 1 |  |
| The six `/ai/conversation/*` route | Implement the chat turn: resolve, assemble history, call aic | done | 1 |  |
| Data model, model resolution and t | Implement ResolveModel with the personal to project to organ | done | 1 |  |
| Data model, model resolution and t | Register the new table in both ownership registers and add t | done | 1 |  |
| Data model, model resolution and t | Publish the AIService gRPC contract restricted by the identi | done | 1 |  |
| Data model, model resolution and t | Extend the SEC-07 secret test to the AI model-source key and | done | 1 |  |
| Data model, model resolution and t | Add the additive migration for ai_conversation columns, ai_c | done | 1 |  |
| Data model, model resolution and t | Implement ListMessages and AppendTurn on the AI usecase | done | 1 |  |
| Data model, model resolution and t | Extend the AI domain types and add the conversation and mess | done | 1 |  |
| Gateway exposure, registers, docum | Revise the documents the change makes false, naming FR-SYS-0 | blocked | 0 |  |
| Gateway exposure, registers, docum | Move the five route and UI-contract registers to the new sta | blocked | 0 |  |
| Gateway exposure, registers, docum | Route the /ai/conversation subtree to system-setting with a  | open | 0 |  |
| Gateway exposure, registers, docum | Record only Claude and Bedrock as deferred and remove the AI | blocked | 0 |  |
| Gateway exposure, registers, docum | Add and run the end-to-end acceptance suite on the real loca | blocked | 0 |  |
| Gateway exposure, registers, docum | Verify a chat turn against an approved real provider in an e | blocked | 0 |  |
| Shared `aiclient`: OpenAI-compatib | Run the aiclient unit suite for the PH-1 scenarios | done | 1 |  |
| Shared `aiclient`: OpenAI-compatib | Create the aiclient package with Client, Provider, ChatReque | done | 1 |  |
| Shared `aiclient`: OpenAI-compatib | Build the HTTP client through netguard AdminTarget and the t | done | 1 |  |
| Shared `aiclient`: OpenAI-compatib | Mask the provider key everywhere but the Authorization heade | done | 1 |  |
| Shared `aiclient`: OpenAI-compatib | Retry at most twice on 429, 5xx and timeout, honouring a cap | done | 1 |  |
| Shared `aiclient`: OpenAI-compatib | Return ErrProviderUnsupported for Claude and AWS Bedrock wit | done | 1 |  |
| Per-organisation quota, cost ceili | Check the quota before model resolution and refuse an exhaus | done | 1 |  |
| Per-organisation quota, cost ceili | Write the ledger row and the period counter in the same tran | open | 2 | env_defect, env_defect |
| Per-organisation quota, cost ceili | Run the quota suite including the no-provider-call proof | open | 0 |  |
| Per-organisation quota, cost ceili | Add the static price table and the pre-call token estimator | done | 1 |  |
| Per-organisation quota, cost ceili | Add the quota and ledger migration and register the tables a | done | 1 |  |
| Per-organisation quota, cost ceili | Validate the request organisation against the caller's membe | done | 1 |  |

22 of 30 task(s) done or in review; 2 failed attempt(s) recorded.