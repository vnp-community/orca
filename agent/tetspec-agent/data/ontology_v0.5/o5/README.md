# Ontology v0.6 runtime (v0.5 compatibility)

Shared ontology, evidence and transaction contracts support three flows:

- `harness/dynamic.py`: scoped source intake → draft TDD/KG → executed readiness → render.
- `edit/prompt_edit.py` and `edit/change_request.py`: prompt/CR → reviewed plan → atomic ChangeSet → impact/invalidation → draft render.
- `specgen/spec_change.py`: immutable KG/repo pack → propose authoring → independent model critique and assertion validation → strict staging CLI → atomic publication.

## Technology boundary

The Python code in this directory is the reference harness, validator and CLI
runtime for the ontology. It is **not** a declaration that the product
backend must use Python, nor does this repository currently contain a product
backend service or a backend package manifest. The implementation language,
framework, runtime, build tool and deployment target are project/component
facts that must come from the pinned SAD, repository snapshot or approved
technology catalogue. If those facts are absent or conflicting, generation
stays draft/blocked; the agent must not infer Python from the harness.

## Settled document workflow

Production stage-5 generation consumes **CR/URD + SRS delta + fixed SAD + the
realized TDD set**.  The agent first emits an `ImpactReport` for every baseline
TDD (`add`, `update`, or `no_change`) and waits for SA/Dev approval.  A
`TddSetRealization` then binds the approved plan to the actual post-step-4
sections.  `workflow.py` builds the canonical `ChangeSpecItem` table; every
eligible item must resolve SRS and TDD section references and a CR acceptance
criterion before an OpenSpec projection is allowed.  KG-only requests remain
diagnostic/draft and cannot become implementation-ready.

From this directory:

```sh
python3 tools/ontology5.py generate
python3 tools/run_tests.py
python3 tools/document_workflow.py impact input.json
```

The test runner needs no pytest dependency. It runs function tests and unittest cases; failures return a nonzero exit code. `FixtureRunner` is confined to tests and is not evidence of real model authoring.

`harness/render.py` renders a reader-facing TDD: a document guide and class
sections expose the design facts and source evidence, while a collapsed
canonical KG payload preserves lossless round-trip reconstruction. The payload
is compressed and the renderer still rejects changed, missing, or invented
facts. `specgen/spec_change.py` projects an eligible SRS/TDD/SAD ChangeSpec
table into the same four artifacts produced by OpenSpec propose. Its design
section quotes only KG facts selected by the SRS/TDD references, and its
proposal, tasks, delta spec, and trace manifest carry the CS→SRS→TDD→CR chain.

From the repository root, run the local PBOM regression:

```sh
python3 data/ontology_v0.5/o5/tools/run_quality_pilot.py --run-dir data/test-pbom/run-quality-verified
```

This records source-anchored oracle checks, typed node/edge counts, semantic round-trip, blocked upstream timeout edit, a **simulated test policy** for a local timeout deviation, duplicate entitlement no-op, and a genuinely new T-14 reminder CR fixture. All unreviewed input or missing contract information remains draft. The T-14 change is an evaluation fixture, not a production PBOM requirement approval.

## Evidence and readiness

`tools/quality_contract.py` versions four evidence schemas in `schemas/*.quality.schema.json`. Source lineage binds document/version/hash/line/quote; derived or proposed facts retain inputs and review state. `tools/quality5.py` executes QG-01…QG-10 graph checks. `tools/techspec_rules.py` implements the TS-01…TS-10 meanings documented in `docs/TDD_KG_TechSpec_Readiness_v0.5.md`.

The active manifest is ontology **v0.6.0**. Canonical generated artifacts use
v0.6 identifiers; `schemas/compat-v0.5/` and v0.5 TTL/registry artifacts remain
available for read and migration compatibility. `migrate5.py` defaults to v0.6
and accepts an explicit `target_version="0.5.0"` only for compatibility rollback.

Rule names or `implemented` declarations alone never establish readiness. Required checks bind their execution to the snapshot and carry outcomes/evidence. `readiness5.execute_need_validators` also emits a source/provenance-bound outcome for every mandatory knowledge need; a missing, conflicted, unknown or unsupported need blocks `implementation_ready` even when unrelated work/tests pass. Missing source, unknown contracts, truncated dependencies, unresolved conflicts or unapproved proposals block `implementation_ready`. API catalogue entries are reference-only stubs; logical attributes do not invent physical data types.

## Actual propose runner and evaluation

`specgen/authoring.py` invokes Codex CLI with pinned config, structured output, bounded attempts and read-only authoring. It persists request/schema/diagnostics/execution and performs a separate model review. An unavailable runner returns blocked; template diagnostics remain draft outside active changes. Semantic assertions bind actual artifact anchors to KG/repo evidence; surviving baseline scenarios must be retained in MODIFIED requirements.

`run_quality_pilot.py --benchmark` prepares three direct-propose and three KG-propose runs against the same intent, baseline, source excerpts and configuration. This uses the configured CLI backend and requires access to CLI state. Initial sandbox startup attempts failed. After explicit user permission, six real outputs were generated in `data/test-pbom/run-quality-authoring`; all diagnostic staging CLI checks pass. Independent human rubric is pending. Additional corrected model reviews were rejected by automatic review pending specific authorization; original review verdicts are preserved. See `benchmark-report.md` for payload/path/wording-check limitations. Do not treat failed initial executions as authored outputs.

`tools/quality_eval.py` accepts independently supplied human scores for seven dimensions, binds ratings to artifact hashes and checks D5 thresholds. `tools/prepare_d5_review.py` creates the v0.6 hash-bound review bundle and guide. Model critique and test fixtures cannot supply human acceptance scores. No real benchmark quality pass is currently claimed until the independent reviewer fills the bundle.

## Publication and compatibility

Strict OpenSpec validation runs in isolated staging before publication. KG/config/baseline/active-change/code hashes and name collisions are rechecked under the publication lock. Drafts are explicit; `allow_draft=false` blocks draft output. Source/KG/repo changes require revalidation.

The migration tool never modifies frozen v0.4. Legacy ready reports without current evidence are unverified; compatibility is draft-only. See `openspec/changes/harden-tdd-agent-semantic-quality/implementation-status.md` and its per-task legacy assessment before reconciling or archiving changes.
