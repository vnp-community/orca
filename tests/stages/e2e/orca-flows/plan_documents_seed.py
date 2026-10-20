#!/usr/bin/env python3
"""Plants an OpenSpec change on a native Request that waits for plan approval, so the Documents view of CR-REQ-045 can be read without a real
OpenSpec run (the fake claude cannot author a change, and a real one costs 7 to 13 USD).

  python3 plan_documents_seed.py seed  <request-id>    # prints a final JSON line: {"secret", "omittedPath", "omittedSize", "changeId", "versions", "prunedVersion"}
  python3 plan_documents_seed.py clean <request-id>    # removes every row the seed made and restores what it changed

!!! FAKE DATA. It writes into the request database of the local docker stack (`docker exec orca-go-postgres`). Never use it on a shared stack. !!!

What it does, in one transaction: marks the Request's succeeded Plan runs as `agent_proposal` runs (so the service reads them as authored versions),
sets `solution_engine = 'openspec'`, inserts the `openspec_changes` row (status ready) and one `openspec_change_revisions` row per Plan run. The files
are a small change (proposal, design, one spec with scenarios, tasks.md with the machine region), a spec kept only as an omitted marker of 300000
bytes, and a FAKE secret (an AWS-shaped key and a password line) left UNMASKED in design.md: the service must mask it when it reads the file. With
three or more Plan runs the first version keeps no text (as one past the text window does) and the others keep it, each with its own tasks.md.
`clean` restores the modes and the engine from a state file written by `seed` (defaults when it is gone) and deletes only rows marked `seed`.
"""
from __future__ import annotations

import hashlib
import json
import subprocess
import sys
import tempfile
import uuid
from pathlib import Path

CHANGE_ID = "seed-fee-rounding"
BRANCH = "orca/seed-fee-rounding"
COMMIT_PREFIX = "seed-"
# An AWS-shaped key is one of the shapes the service masks (common/secretscan); every character is fake.
FAKE_SECRET = "AKIAFAKESEEDPLANDOC1"
FAKE_PASSWORD_LINE = "password = seed-fake-password-1"
OMITTED_PATH = "specs/audit-trail/spec.md"
OMITTED_SIZE = 300_000
OMITTED_SHA = hashlib.sha256(b"seed: a spec too large to keep").hexdigest()


def omitted_marker() -> str:
    return f"<omitted size={OMITTED_SIZE} sha256={OMITTED_SHA}>"


def tasks_md(version: int, request_ref: str) -> str:
    extra = "\n  - [ ] T1.4 Cover the rounding of a halved fee with a table test\n" if version > 1 else "\n"
    return (f"## Tasks\n\n<!-- orca:begin plan request={request_ref} schema=1 -->\n## TASKS\n"
            "- [ ] T1.1 Add the fee table (ac=AC-1)\n  - [ ] T1.2 Round the fee half up (ac=AC-1)\n  - [ ] T1.3 Show the fee before confirming" + extra +
            "<!-- orca:end plan -->\n")


def bundle(version: int, request_ref: str) -> dict[str, str]:
    """The files of one version of the planted change. Version 1 differs from the later ones in tasks.md and design.md, so the comparison finds changes."""
    return {
        "proposal.md": f"## Why\nTransfers over the daily limit are not charged a fee (seed version {version}).\n\n## What Changes\n- Add a transfer fee.\n",
        "design.md": (f"## Context\nThe ledger charges nothing today (seed version {version}).\n\n## Decisions\n"
                      f"- Keep the fee table in code. Debug credential left in by mistake: {FAKE_SECRET}\n- {FAKE_PASSWORD_LINE}\n"
                      + ("- Round half up on the smallest currency unit.\n" if version > 1 else "")),
        "specs/ledger/spec.md": ("## ADDED Requirements\n### Requirement: A transfer carries a fee\nThe ledger SHALL charge the fee table amount.\n\n"
                                 "#### Scenario: AC-1 fee is charged\n- **WHEN** a transfer is made\n- **THEN** the fee is added\n"),
        OMITTED_PATH: omitted_marker(),
        "tasks.md": tasks_md(version, request_ref),
    }


def file_index(files: dict[str, str]) -> list[dict]:
    """The index the service stores per version (domain.NewRevisionFiles): sorted by path; an omitted marker takes its size and hash from itself."""
    out = []
    for path in sorted(files):
        text = files[path]
        if text == omitted_marker():
            out.append({"path": path, "size": OMITTED_SIZE, "omitted": True, "sha256": OMITTED_SHA})
        else:
            out.append({"path": path, "size": len(text.encode("utf-8")), "omitted": False, "sha256": hashlib.sha256(text.encode("utf-8")).hexdigest()})
    return out


def bundle_digest(files: dict[str, str]) -> str:
    return hashlib.sha256(json.dumps(file_index(files), sort_keys=True).encode()).hexdigest()


def psql(sql: str) -> str:
    done = subprocess.run(["docker", "exec", "-i", "orca-go-postgres", "psql", "-U", "orca", "-d", "request", "-v", "ON_ERROR_STOP=1", "-tA"],
                          input=sql, capture_output=True, text=True, timeout=60)
    if done.returncode != 0:
        raise SystemExit(f"psql failed: {done.stderr.strip()[:400]}")
    return done.stdout


def rows(tenant: str, sql: str) -> list[list[str]]:
    out = psql(f"begin; set local app.tenant_id = '{tenant}'; {sql}; commit;")
    return [ln.split("|") for ln in out.splitlines() if ln and ln not in ("BEGIN", "SET", "COMMIT")]


def tenant_of(request_id: str) -> str:
    found = [ln for ln in psql(f"select tenant_id from request.requests where id = '{request_id}'").splitlines() if len(ln) == 36]
    if not found:
        raise SystemExit(f"no Request {request_id}")
    return found[0]


def state_file(request_id: str) -> Path:
    return Path(tempfile.gettempdir()) / f"plan-documents-seed-{request_id}.json"


def lit(value: str) -> str:
    """A dollar-quoted SQL literal; the tag cannot occur in the data this file builds."""
    return f"$s${value}$s$"


def seed(request_id: str) -> dict:
    tenant = tenant_of(request_id)
    req = rows(tenant, f"select status, number, coalesce(solution_engine, '') from request.requests where id = '{request_id}'")[0]
    if req[0] != "awaiting_plan_approval":
        raise SystemExit(f"Request {request_id} is {req[0]!r}; the seed needs awaiting_plan_approval")
    runs = rows(tenant, f"select id, mode from request.analysis_runs where request_id = '{request_id}' and kind = 'plan' and status = 'succeeded' order by started_at, id")
    if not runs:
        raise SystemExit("the Request has no succeeded Plan run")
    if rows(tenant, f"select 1 from request.openspec_changes where request_id = '{request_id}'"):
        raise SystemExit("the Request already has an OpenSpec change; clean it first")
    state_file(request_id).write_text(json.dumps({"engine": req[2], "modes": {r[0]: r[1] for r in runs}}))
    total = len(runs)
    request_ref = f"REQ-{req[1]}"
    pruned = 1 if total >= 3 else 0
    statements = []
    newest = bundle(total, request_ref)
    statements.append(
        "insert into request.openspec_changes (id, tenant_id, request_id, change_id, branch, status, tasks_sync_state, commit_sha, kg_available, bundle_json, redacted_secrets) "
        f"values ('{uuid.uuid4()}', '{tenant}', '{request_id}', '{CHANGE_ID}', '{BRANCH}', 'ready', 'in_sync', '{COMMIT_PREFIX}{total}', false, {lit(json.dumps(newest))}::jsonb, 0)")
    for number, (run_id, _mode) in enumerate(runs, start=1):
        files = bundle(number, request_ref)
        text = "NULL" if number == pruned else f"{lit(json.dumps(files))}::jsonb"
        statements.append(
            "insert into request.openspec_change_revisions (id, tenant_id, request_id, revision, run_id, commit_sha, tasks_md_digest, bundle_digest, files_json, bundle_json) "
            f"values ('{uuid.uuid4()}', '{tenant}', '{request_id}', {number}, '{run_id}', '{COMMIT_PREFIX}{number}', '{hashlib.sha256(files['tasks.md'].encode()).hexdigest()}', "
            f"'{bundle_digest(files)}', {lit(json.dumps(file_index(files)))}::jsonb, {text})")
    statements.append(f"update request.analysis_runs set mode = 'agent_proposal' where request_id = '{request_id}' and kind = 'plan' and status = 'succeeded'")
    statements.append(f"update request.requests set solution_engine = 'openspec' where id = '{request_id}'")
    psql(f"begin; set local app.tenant_id = '{tenant}'; " + "; ".join(statements) + "; commit;")
    return {"secret": FAKE_SECRET, "omittedPath": OMITTED_PATH, "omittedSize": OMITTED_SIZE, "changeId": CHANGE_ID, "versions": total, "prunedVersion": pruned}


def clean(request_id: str) -> None:
    tenant = tenant_of(request_id)
    saved = {}
    try:
        saved = json.loads(state_file(request_id).read_text())
    except (OSError, ValueError):
        pass
    statements = [f"delete from request.openspec_change_revisions where request_id = '{request_id}' and commit_sha like '{COMMIT_PREFIX}%'",
                  f"delete from request.openspec_changes where request_id = '{request_id}' and change_id = '{CHANGE_ID}'"]
    for run_id, mode in (saved.get("modes") or {}).items():
        statements.append(f"update request.analysis_runs set mode = '{mode}' where id = '{run_id}' and request_id = '{request_id}'")
    engine = saved.get("engine", "")
    statements.append(f"update request.requests set solution_engine = {('NULL' if not engine else lit(engine))} where id = '{request_id}' and solution_engine = 'openspec'")
    psql(f"begin; set local app.tenant_id = '{tenant}'; " + "; ".join(statements) + "; commit;")
    state_file(request_id).unlink(missing_ok=True)


def main(argv: list[str]) -> int:
    if len(argv) != 3 or argv[1] not in ("seed", "clean"):
        print(__doc__, file=sys.stderr)
        return 2
    if argv[1] == "seed":
        print("plan_documents_seed.py plants FAKE data in the request database of the local docker stack", file=sys.stderr)
        print(json.dumps(seed(argv[2])))
    else:
        clean(argv[2])
        print(json.dumps({"cleaned": argv[2]}))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
