# BUG-022: `aiProvider.resolve` wscompat channel returns the raw `ProviderAccount` proto (snake_case field names, numeric `type` enum) instead of a frontend-consumable camelCase shape

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_ai_provider.go` (`handleAiProviderResolve`, and 4 sibling handlers with the identical pattern)
**Severity:** 🟠 High — blocks SOL-FE-PW-004 (Agent Panel web orchestration fix), the first-ever real caller of `aiProvider.resolve`
**Status:** ✅ Root cause confirmed via source read. ✅ Fixed for `aiProvider.resolve` specifically (the channel SOL-FE-PW-004 needs) + unit-tested. Sibling handlers left as-is — see "Related, NOT fixed here" below.

## Root cause

`handleAiProviderResolve` (`channels_ai_provider.go:187-222`) ends with `return resp.GetAccount(), nil` — the **raw `*aiproviderv1.ProviderAccount` proto struct**, same footgun already called out in comments elsewhere in this same file's package (`channels_infra_fleet.go`, `channels_tenant_project.go`, `channels_dev_server_access_control.go`, `channels_workflow.go`: *"Result any serializes via plain encoding/json, not protojson — returning a raw proto struct silently ships \`json:"snake_case"\` field names"*) — and the same bug class as BUG-020 (`git.status`).

Confirmed via the generated struct tags (`proto/gen/go/orca/aiprovider/v1/aiprovider.pb.go:89-109`): protoc-gen-go's plain `encoding/json` struct tags are **snake_case** (`json:"tenant_id,omitempty"`, `json:"dev_server_id,omitempty"`, `json:"model_hint,omitempty"`, `json:"base_url,omitempty"`, `json:"is_default,omitempty"`, `json:"last_health_check_at,omitempty"`, `json:"created_by,omitempty"`) — only single-word fields (`id`, `type`, `status`, `label`, `models`) happen to look camelCase. Every other `aiProvider.*` request-arg struct in this same file is hand-written with correct camelCase tags (`json:"devServerId"`, `json:"modelHint"`, ...), but the **response** was never given the same treatment.

Additionally, `Type ProviderType` (`int32`-backed enum, no custom `MarshalJSON`) serializes as a **bare number** (e.g. `1`), not the enum name string (`"PROVIDER_TYPE_ANTHROPIC"`) that `aiProvider.create`'s own request decoding expects back (`aiproviderv1.ProviderType_value[in.Type]` — a string key lookup) — so the request and response sides of this same field are asymmetric.

## Why it was never caught

`aiProvider.resolve` has **zero frontend callers today** (confirmed via repo-wide grep) — it was implemented (SOL-005) ahead of any consumer, per TASK-AG-01-07's documented intent that the Agent Panel would be its first real caller. SOL-FE-PW-004 is that first caller, which is what surfaced this.

## Fix

Added `providerAccountView` (camelCase mirror of `ProviderAccount`, `type` mapped through `aiproviderv1.ProviderType_name[...]` to its enum-name string, trimmed of the `PROVIDER_TYPE_` prefix and lowercased — e.g. `"anthropic"`, `"openai"` — matching the lowercase style `aiProvider.create`'s own request already accepts nowhere else, chosen simply as the most frontend-ergonomic string form since no existing caller constrains it) and `toProviderAccountView(*aiproviderv1.ProviderAccount) providerAccountView`, applied to `handleAiProviderResolve`'s return only.

## Related, NOT fixed here

`handleAiProviderCreate`, `handleAiProviderList`, `handleAiProviderUpdate`, `handleAiProviderWriteCredential` return the same raw proto (`resp.GetAccount()`/`resp.GetAccounts()`) and have the identical bug — **deliberately left unfixed in this pass**. Unlike `resolve`, these 4 already have a real, shipped caller (`useAIProviders.ts` / `ProviderForm.tsx`, the Settings > AI Providers page) whose frontend `AIProviderAccount` type (`types/ai-provider-types.ts`) expects a **materially different shape** than the backend proto even beyond casing — `provider`/`model`/`scope`/`scopeRefId`/`createdAt` (number) have no corresponding proto field at all (proto has `type`/`model_hint`+`models[]`/no scope concept/`created_by`+`last_health_check_at` strings). That settings page's contract needs its own dedicated audit and fix, not a mechanical rename — out of scope for unblocking SOL-FE-PW-004.

## Testing

- `TestAiProviderResolveChannel_MapsToCamelCaseView`: asserts the JSON response has `id`/`devServerId`/`modelHint`/`models`/`type` (lowercase enum name string), never the raw proto's snake_case keys.

## Related

- [BUG-020](./BUG-020-git-status-wscompat-channel-returns-raw-proto-not-frontend-shape.md) / [BUG-021](./BUG-021-git-channels-decode-wrong-worktree-key-systemic.md) — the same "raw proto over the wire" and "wrong JSON key" bug classes, found earlier in `git.*` channels in this same session.
- [SOL-FE-PW-004](../../../specs/frontend/bugs/project-workspace/solutions/SOL-FE-PW-004-agent-panel-web-orchestration.md) — the fix this bug was found while implementing.
