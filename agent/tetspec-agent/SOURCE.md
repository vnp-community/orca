# tetspec-agent

The tdd-agent Python package with the test-spec agent (`tsagent`), vendored from
`blcvn/tdd-agentic-ai`, branch `tasks/integrate-tetspec-agent` (commit `ca5addb`, on top of `origin/main` `224dda5`).

Orca drives it from `backend-go/services/request-service` (test specs next to the code of an executing Request) over the dev
server agent's `mcp.callTool` bridge: `testspec_start`, `testspec_get`, `run_dispatch`, `run_get`, `run_cancel`.
Design: `docs/integrate/TDD-REQ-TESTSPEC-001-testspec-in-request-execution.md`.

## What is here

- `src/tdd_agent`, `src/tsagent`: the package (CLI `tdd-agent`, MCP server, test-spec flow `testspec`, context modes `solution` / `plan`).
- `tests/`, `docs/`, `scripts/`, `deploy/tsagent/`, packaging files (`pyproject.toml`, `setup.py`, `MANIFEST.in`, `install.sh`).
- `data/ontology_v0.5/o5` and the two `data/ontology_v0.4/o4` files the package reads at runtime (without the o5 fixtures and tests).

Not copied: `data/test-pbom`, `data/test-tms`, `data/ontology_*` fixtures and `openspec/` of the source repo. Tests and acceptance
reports that read them need the source repo.

## Run

```bash
cd agent/tetspec-agent
uv venv && uv pip install -e ".[dev]"        # or: pip install -e ".[dev]"
tdd-agent --help
PYTHONPATH=src:tests pytest tests -q          # some tests need data/ of the source repo
```

Keep this copy in step with the source repo: a change to the test-spec agent is made there and copied here with the same commit id noted above.
