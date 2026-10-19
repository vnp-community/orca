# LedgerLite (ORCA e2e fixture)

A small double-entry ledger service in Go (standard library only) plus the design documents tdd-agent needs to build a knowledge graph. ORCA's e2e copies it into a throwaway git repo and runs Requests (analysis, plan, execution on the native and OpenSpec engines) against it.

The code is a clean, green baseline. The three open work items the e2e asks ORCA to do (a bug, a task, a change request) live only as text in [`docs/e2e-scenarios.md`](docs/e2e-scenarios.md).

## Layout

| Path | What it is |
| --- | --- |
| `cmd/ledgerd/` | Server binary (`-addr`, `-daily-limit`) |
| `internal/ledger/` | Ledger core: accounts, idempotent transfers, daily limit, append-only journal, `Store` interface + in-memory store |
| `internal/httpapi/` | JSON HTTP API on `net/http` (API-001..API-006) |
| `docs/URD.md`, `docs/SRS.md`, `docs/SAD.md` | tdd-agent source documents (ids BO/REQ, FR/BR/NFR/ENT/API/CT, SVC/ADR). Requirement rows name the code (`file::Symbol`) and the Go tests that verify them; the ids also appear in code comments |
| `docs/e2e-scenarios.md` | Bug / task / change_request scenarios: request text, expected type and size, definition of done |
| `.tdd-agent/config.toml` | tdd-agent project config that ORCA looks for at the repo root (without it plans fail with `KNOWLEDGE_CONFIG_MISSING`) |
| `.tdd-agent/.gitignore`, `.gitignore` | Keep generated state out of git |

Source size: about 560 lines of Go excluding tests; 4 packages, no third-party dependencies, Go 1.22+.

## What is generated, and why it is not committed

Generated at e2e time, not committed:

- `.tdd-agent/knowledge.db`, `operational.db`, `last-generate.json`, `logs/`, `runs/` (ignored by `.tdd-agent/.gitignore`)
- `docs/tdd/TDD-LEDGERLITE.md` (ignored by `.gitignore`)
- `.mcp.json`: it contains the absolute path of the repo, so it cannot be portable. It is created per temp repo and should be committed there (so task worktrees get it).

Why not commit the graph: generation is cheap and offline (about 1 s, no model call because `prose = false` in the config) and, in two clean runs, produced identical element and relation tables. A committed SQLite file would add a 400 KB binary that goes stale as soon as the e2e edits `docs/SRS.md` or `docs/SAD.md`.

## How ORCA's e2e uses it

```bash
FIXTURE=<orca>/tests/stages/e2e/fixtures/ledgerlite-finance
WORK="$(mktemp -d)/ledgerlite" && mkdir -p "$WORK" && cp -R "$FIXTURE/." "$WORK/" && cd "$WORK"

# 1. design knowledge (graph + MCP entry), before the single commit
tdd-agent generate                      # reads docs/URD.md, SRS.md, SAD.md from .tdd-agent/config.toml
tdd-agent mcp enable --read-only        # writes .mcp.json (server "tdd-agent"); config.toml stays unchanged

# 2. the throwaway repo, one commit
git init -q -b main && git add -A && git commit -q -m "LedgerLite baseline"

# 3. baseline is green
GOWORK=off go test ./... -race -count=1
```

`GOWORK=off` is only needed when a `go.work` exists in a parent directory (it does in the orca checkout; it does not in a temp dir).

For scenario 1 apply the bug patch from `docs/e2e-scenarios.md` and commit it before creating the Request.

## Commands

Verified here (tdd-agent installed from `blcvn/tdd-agentic-ai` commit `224dda5`, Go 1.25.6, macOS arm64):

```bash
GOWORK=off go vet ./... && GOWORK=off go test ./... -race -count=1      # PASS (2 packages)
GOWORK=off go test ./internal/ledger -run xxx -bench Transfer           # ~8 us/op
tdd-agent generate --human            # -> 67 elements, 78 relations, 0 gaps, 0 conflicts; readiness "blocked" (see below)
tdd-agent kg stats                    # elements/relations by class
tdd-agent kg search "idempotency"
tdd-agent mcp enable --read-only      # writes .mcp.json
tdd-agent -C "$WORK" mcp serve        # stdio MCP server, exposes the 13 read-only tools
tdd-agent doctor --human              # all checks ok (claude, openspec, documents, git ignore)
```

The MCP server was exercised with the MCP Python client that ships in tdd-agent's own virtualenv: `initialize`, `list_tools`, then `kg_search`, `kg_get_element`, `kg_neighbors` and `kg_resolve_scope` returned real data. The read-only tools are `kg_search`, `kg_get_element`, `kg_neighbors`, `kg_resolve_scope`, `kg_query`, `kg_snapshots`, `source_get`, `impact_get`, `readiness_get`, `trace_get`, `run_get`, `run_list`, `metrics_get`.

Not verified (not run here): `tdd-agent generate` with prose enabled (model call), `tdd-agent change ...`, `mcp serve --transport http`, and how a given ORCA agent attaches the MCP server (this README only guarantees a valid `.mcp.json` and a working `tdd-agent -C <repo> mcp serve`).

## Environment

- `tdd-agent` on `PATH` (`uv tool install git+ssh://git@github.com/blcvn/tdd-agentic-ai.git`). Python is managed by `uv`.
- Go 1.22+ on `PATH`; `python3` (3.9+, standard library only) for the black-box checks in the scenarios.
- `openspec` on `PATH` and `claude` on `PATH` are only needed by `tdd-agent doctor` and by ORCA's OpenSpec engine, not by `generate` or `mcp serve`.
- No credentials and no network are needed for generation or for the MCP server.
- ORCA side (see `docs/guides/request/knowledge-hub-setup.md`): the four `tdd-*` skills installed on the dev server, and `AGENT_WORK_DIR` containing the temp repo and its sibling worktrees.

## Known limits of the generated knowledge

- Readiness is `blocked` with 3 open needs (`purpose`, `work`, `validation`): the documents define no Capability, WorkItem or TestCase elements. The graph itself is complete (0 gaps) and fully usable over MCP. The tdd-agent-specific class mapping `[project.id_classes]` that would allow a `TC-` test-case id is documented but rejected by the installed build (`CONFIG_UNKNOWN_KEY`), so test links live in each row's `verified_by` property instead of separate TestCase elements.
- `generate` creates `satisfies`, `traces_to` and `has_attribute` relations. The `implements` and `verifies` relations of the ontology run between WorkItem, ChangeSpecItem and TestReceipt and are created by tdd-agent's change flow or by `changeset_propose`, not by `generate`. Code locations are therefore recorded as properties (`implemented_in` as `path::Symbol`, `verified_by` as test names) and as requirement ids in code comments.
- POST operations in the SRS carry `Idempotency` and `Concurrency` columns on purpose: without them tdd-agent demotes the declared method to text and opens a gap.
