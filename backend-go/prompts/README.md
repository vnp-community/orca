# prompts

Every prompt a backend service sends to the agent is a template in this module. Go code here only loads data and renders;
the wording lives in `.tmpl` files so a change to a sentence is a diff to a text file. CR-REQ-042, TASK-REQ-042-06.

Module path `github.com/stablyai/orca-go/prompts`. It imports `common/secretscan` and nothing from a service's `internal/`.

## Layout

| Package | What it renders |
|---|---|
| `classify_request` | classify a new Request (type, size, urgency); random boundary instead of a fence |
| `analysis` | Solution (`solution.tmpl` for ai.complete, `agent_solution.tmpl` for a read-only agent), `diagnosis.tmpl`, `findings.tmpl`, `answer.tmpl` |
| `clarification_answers` | the `<clarifications>` block with the reviewer's earlier answers |
| `plan_native` | Plan as one JSON document; `_prior_plan.tmpl` shows the Plan being revised (CR-REQ-045) |
| `plan_openspec` | the OpenSpec change an agent authors; `_revise_in_place.tmpl` tells it to edit the files of the previous version (CR-REQ-045); `TaskRange` is the task count per Request size |
| `task_prompt_template` | the standing instruction stored with each task of an OpenSpec plan |
| `solution_openspec` | restricts an agent to one OpenSpec change directory |
| `task_execution` | the prompt that runs one task (`execute.tmpl`) and the result-block instruction of a contract run (`result_block_suffix.tmpl`); `{{outputs.<taskId>.*}}` is resolved by task-service after `Render` |
| `task_decompose` | ai.complete prompt that breaks a task into subtasks |
| `task_agent_prompt` | ai.complete prompt that asks for the agent prompt of a task |
| `git_commit_message` | ai.complete prompt for a commit message, and the file-stats body used when the diff is too large |
| `git_pull_request` | ai.complete prompt for a PR title and description |
| `git_agent_prompt` | prompt typed into the agent session started in a worktree created from an issue; `Sanitize` strips HTML and cuts each field to 4000 bytes |
| `request_context` | partials shared by the flows above: `_request_block`, `_project`, `_feedback`, `_retry_note`, `_approved_decision`, `_language_rule`, plus the length caps |
| `output_language` | the closed set of output languages (`vi`, `en`) the `analysis`, `plan_native` and `plan_openspec` inputs carry in `Language`; see "Output language" |
| `fence` | `Untrusted`, `Closing`, `Truncate` |
| `redact` | `Secrets` (always on) and `Redactor` (secrets always, PII per tenant, fails closed) |
| `render` | the template set every flow uses and its function map |

## Add a flow or a template

1. New directory named after the flow, one Go package of the same name (underscores kept).
2. `//go:embed *.tmpl`, `const ID = "<flow>/<document>"`, a pure-data `Input` struct (strings, ints, bools, slices; never a service type), and
   `func Render(in Input) (string, error)` calling `render.NewSet(funcs, files, request_context.Files).Execute("<file>.tmpl", in)`.
3. One file per document kind. Text shared by several kinds goes in a partial whose name starts with `_`; text shared by several flows
   goes in `request_context`. File names are unique across the files of one set.
4. Logic that is awkward in a template (a cap over a whole block, choosing a count, entropy) stays in Go in the flow's package, never in a service.
5. The service keeps a thin function mapping its domain types to `Input`, with a test, and records the template ID where provenance is kept.
6. Add a golden for the new function (below) before the first caller uses it.

## Template rules

- `text/template` with `missingkey=error`: a field the data does not have fails the render, it never prints `<no value>`.
- Text written by an outside party (Request fields, project name, feedback, earlier artifacts, answers) goes through `fence` in the template.
  `fence` neutralises the closing tag of every block that wraps user text (`</request`, `</prior_artifacts`, `</clarifications`,
  `</feedback`, `</project`), so the text cannot end its block and pose as instructions. A block with its own tag uses `closing`.
- Length caps are named functions (`bodyRunes`, `artifactRunes`, ...) from the constant next to the data they cap. Cut before fencing, by runes, never bytes.
- Text a person typed that may hold a credential goes through `redact` as well as `fence` (clarification answers do).
- Whitespace is part of the prompt. A partial that ends with a newline ends its file with that newline; one that must not ends with
  `{{- /* ... */ -}}`. Control the lines around `{{if}}`/`{{end}}` with `-}}`. The golden files show any slip.
- Required fields (`ChangeID`, `ResultNonce`, ...) are checked in `Render` with `render.Require`, so an empty one is an error, not a prompt that sends the agent to the wrong place.
- Never put a secret, API key or token in an `Input`. There is no field for one on purpose.

- Template functions: `fence` (neutralise every closing tag of a block that wraps user text), `closing` (one named tag), `truncate` (first n runes),
  `redact` (mask credentials; there is no switch to turn it off). `redact.Redactor` is the per-tenant variant (PII too) and fails closed when the
  tenant setting cannot be read. Order matters: `truncate`, then `redact`, then `fence`, so a cut never lands inside a mask or an escaped tag.

## Output language

`analysis.Input`, `plan_native.Input` and `plan_openspec.Input` carry `Language output_language.Language` (CR-REQ-046). A set Language adds one
instruction block ahead of the data (`<request>`): the first line is always `Output language: <Name>.`, then the rules for what stays in English.
`request_context/_language_rule.tmpl` serves the JSON prompts (keys, `kind`, enum values, ids, labels, paths and code stay in English);
`plan_openspec/_language_rule_openspec.tmpl` serves the OpenSpec author (grammar headings, `SHALL`/`MUST`, task line labels, `ORCA_RESULT_*` stay in English).
The partial files have different names because a set requires unique file names across the `fs.FS` it parses.

- The zero Language emits nothing, so every prompt that predates the setting is byte for byte what it was; the golden files are the proof.
  Production always passes a non-empty Language (`REQUEST_DEFAULT_OUTPUT_LANGUAGE`, default `vi`).
- The instruction never contains the text `"kind":"<kind>"`: the stub model finds the read-only kind by that string ahead of `<request>`.
- Templates write `{{.Language.Name}}`, never a language name, so a new language is one line in `output_language.go`.

## Package and prompt IDs

Every flow exports `const ID = "<package>/<document>"` (the analysis package exports one per kind). The ID is what provenance records next to a
generated artifact, so it is part of the contract: renaming a package or a document changes the ID, which needs the same review as a wording change.

| ID | Package |
|---|---|
| `classify_request/classify` | `classify_request` |
| `analysis/solution`, `analysis/agent_solution`, `analysis/diagnosis`, `analysis/findings`, `analysis/answer` | `analysis` |
| `clarification_answers/answers` | `clarification_answers` |
| `plan_native/plan` | `plan_native` |
| `plan_openspec/plan` | `plan_openspec` |
| `task_prompt_template/task` | `task_prompt_template` |
| `solution_openspec/solution` | `solution_openspec` |
| `task_execution/execute`, `task_execution/result_block_suffix` | `task_execution` |
| `task_decompose/decompose` | `task_decompose` |
| `task_agent_prompt/generate` | `task_agent_prompt` |
| `git_commit_message/commit_message`, `git_commit_message/stats_only` | `git_commit_message` |
| `git_pull_request/fields` | `git_pull_request` |
| `git_agent_prompt/issue` | `git_agent_prompt` |

## Golden rule

The golden files are the contract. Each service keeps `testdata/prompt-golden/<function>/<case>.txt` next to the tests that render its
prompts, and `backend-go/ci/check-prompt-golden.sh` (`make check-prompt-golden`) fails when a rendered prompt differs by a byte.

- A difference you did not mean is a bug in the template or the mapping: fix that, never the golden.
- An intended wording change: run `backend-go/ci/check-prompt-golden.sh --update`, read `git diff -- '*/testdata/prompt-golden/*'`, and commit the
  golden change in the same commit as the template so the reviewer sees both. Say why in the commit message; prompt behaviour is reviewed like code.
- Do not edit a golden by hand to make a test pass.
Goldens are generated, never typed: `--update` runs the services' `TestPromptGolden_*` tests with `-update`, which rewrites each `.txt` from the real
render. They may change only by that explicit, reviewed regeneration for a wording change someone meant. Never regenerate to make a red CI green: a red
golden is the signal that a prompt changed. The golden files are the evidence that moving a prompt into this module (TASK-REQ-042-06, 07) did not change one byte.

## Tests

`go test ./... -race` in this directory. Each flow has tests for: every template rendering for a full `Input`, hostile text (a closing tag in every
field) staying inside its block, optional blocks left out when empty, and a missing required field returning `render.ErrMissingField`.

## Size budgets

`prompt_size_budget_test.go` (package `prompts_test`, in this directory) renders every flow with a deliberately large fixture and fails when the
prompt is bigger than the flow's budget in `maxPromptBytes`. Fixture: a 200,000-rune Request body and 60 acceptance criteria, 30 earlier
Solutions of 60,000 runes each with 10,000 runes of feedback, 50 clarification answers (5,000-rune question, 20,000-rune answer), 50,000-rune
feedback and retry note, an 80,000-rune approved analysis, a 60,000-rune task description with 30 dependencies, 400,000-rune diffs, 2,000 changed
files, 50 issue comments, a 40,000-rune prior Plan and 200 file names of a previous OpenSpec change. The text mixes ASCII with multi-byte letters, so a cap counted in bytes instead of runes would show.

Measured on 2026-10-20 (bytes of the rendered prompt) and the budget set about 10 to 20 percent above it. The analysis, `plan_native` and `plan_openspec` fixtures set `Language: output_language.Vietnamese` (CR-REQ-046), which adds 582 to 1,059 bytes. The two Plan flows were measured again with the revision context of CR-REQ-045 (prior Plan cut at 20,000 runes, 200 file names), which adds 24,403 and 28,807 bytes, so their budgets rose from 72,000 and 75,000:

| Flow | Measured | Budget | What bounds it |
|---|---:|---:|---|
| `classify_request/classify` | 29,843 | 34,000 | body capped at 20,000 runes |
| `analysis/solution` | 174,719 | 200,000 | body 12,000 runes, earlier Solutions 40,000 runes in total, answers uncapped in count |
| `analysis/agent_solution` | 176,145 | 200,000 | same |
| `analysis/diagnosis` | 174,422 | 200,000 | same |
| `analysis/findings` | 174,217 | 200,000 | same |
| `analysis/answer` | 174,080 | 200,000 | same |
| `clarification_answers/answers` | 81,749 | 95,000 | 300 + 1,000 runes per answer; the number of answers is not capped |
| `plan_native/plan` | 87,956 | 100,000 | analysis (the approved Solution) is not capped here; the prior plan of a revision is cut at 20,000 runes |
| `plan_openspec/plan` | 95,916 | 108,000 | analysis not capped; retry note 3,000 runes; a revision lists up to 200 file names |
| `task_prompt_template/task` | 26,462 | 30,000 | one line per criterion, not capped in count |
| `solution_openspec/solution` | 15,040 | 17,000 | body capped at 12,000 runes |
| `task_execution/execute` | 434,634 | 490,000 | the author's task text is sent as written (no cap) |
| `task_execution/result_block_suffix` | 1,235 | 2,000 | fixed text plus the nonce |
| `task_decompose/decompose` | 212,110 | 240,000 | task text sent as written |
| `task_agent_prompt/generate` | 145,860 | 165,000 | task text sent as written |
| `git_commit_message/commit_message` | 497,209 | 560,000 | the diff is sent as written; the service switches to the file list above 50 files |
| `git_commit_message/stats_only` | 334,067 | 380,000 | one line per changed file |
| `git_pull_request/fields` | 484,620 | 545,000 | the diff is sent as written |
| `git_agent_prompt/issue` | 208,573 | 235,000 | each field cut to 4,000 bytes; the number of comments is not capped |

Flows marked "not capped" are budgeted as fixture plus template text: the number is the size a realistic worst case produces, not a guarantee the
template enforces. Raising a budget needs a reason in the PR (a bigger prompt costs money and leaves the agent less of its own context); lowering it
is always welcome. When a flow gets a new field, extend its fixture in the same change.

## CI checks

`make check-prompts` (in `backend-go/`) is what the `prompt-catalog` job of `.github/workflows/backend-go-request-contracts.yml` runs, after
`go vet`, `gofmt` and `go test ./... -race` of this module (the size budget included). The job triggers on changes to `backend-go/prompts/**`,
`backend-go/common/**` (so `common/agentrelay`), `backend-go/services/**` and `backend-go/ci/**`.

| Check | Script | Fails when | Limits |
|---|---|---|---|
| Single relay | `ci/check-agent-relay-single-source.sh` | a non-test Go file outside `common/agentrelay` and `infra-fleet-service` uses `agent.execPrompt` / `ai.complete` as a `Method:` or names them in a quoted literal not in its short reasoned allowlist | grep: a name assembled at run time or hand-built params without the method name are not seen |
| Prompt outside the catalog | `ci/check-prompt-outside-catalog.sh` | a non-test Go file outside this module holds `<feedback>`, `<request>`, `<prior_artifacts>`, `<clarifications>`, `<project>` or `<approved_decision>` on a code line | heuristic: a prompt that uses none of these tags, builds a tag at run time or sits in a test is not seen; the contract-test harness is excluded with a reason |
| Golden | `ci/check-prompt-golden.sh` | a rendered prompt differs by a byte from its golden file, or no golden test ran | covers the functions with a `TestPromptGolden_` test; a new prompt function without one is not covered until you add it |
| Self-test | `ci/check-prompt-guards_test.sh` | one of the two guards above passes a violation planted in a temp copy (or fails the clean tree) | proves the guards bite; it does not prove the heuristics are complete |

Both guards keep their allowlist inside the script with a reason per entry, and fail on an entry that no longer matches anything, so the list can only
shrink. Add an entry only for a file that mentions a name without sending or building a prompt.

## Consumers

A service needs `require github.com/stablyai/orca-go/prompts v0.0.0` and `replace github.com/stablyai/orca-go/prompts => ../../prompts` in its
`go.mod` (like `common` and `proto`), and its Dockerfile copies `prompts` next to `common` because `go.work` lists the module.
