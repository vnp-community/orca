---
name: tdd-knowledge-writeback
description: >-
  After a task changed the project, work out which part of the design knowledge
  (tdd-agent knowledge graph) is now out of date and hand it over correctly:
  document facts (SRS/TDD) go back through the documents and a regenerate, and
  knowledge-graph-only facts (what implements and verifies a requirement, test
  receipts) go through a ChangeSet proposal with `changeset_propose`. Use it at
  the end of a task that changed behaviour, an interface, an entity, a rule or
  a decision. It only proposes: it never commits to the graph, never edits the
  SAD, and never approves anything.
---

# Updating the design knowledge after a change

The knowledge graph (KG) has two kinds of facts. Updating the wrong channel
corrupts it, so decide the kind first.

| Kind | Examples | How it is updated |
|---|---|---|
| **Document facts** (origin: URD, SRS, SAD) | a requirement's text, a business rule, an entity attribute, a decision | Edit the **document** (SRS or TDD) in the task's worktree; the KG is rebuilt from the documents by `tdd-agent generate` (done by the pipeline or the owner, not by you). The **SAD is read-only**: never edit it; if a change contradicts an ADR, report it. |
| **KG-only facts** (origin: own) | which code symbol implements a requirement, which test verifies it, a test receipt, a work item | A **ChangeSet** proposed with `changeset_propose`. |

## 1. Find the affected zone

1. List what the task changed: files, symbols, routes, tables, behaviours.
2. With `tdd-knowledge`, read the requirements, rules, entities and decisions
   that mention them (`kg_search`, then `kg_neighbors`).
3. For each, decide: **still true**, **text now wrong** (document fact), or
   **missing link** (KG-only fact). Write the list with ids; this list is the
   deliverable even if you cannot propose anything.

## 2. Document facts

- Edit only the SRS/TDD files that belong to the task's scope, as part of the
  task's change. Keep the edit minimal and keep the existing table layout.
- A changed requirement needs its verification column updated too; a column that
  still describes the old behaviour makes later checks fail.
- Do not touch the SAD. If an ADR forbids what you did, stop and report the id
  and the conflict.
- Say in the task result: "documents changed: <files>; regenerate needed".

## 3. KG-only facts: propose a ChangeSet

1. Read the head with `kg_snapshots` (limit 1) and note the snapshot id.
2. Build a ChangeSet with `ops` of kind `add`, `link`, `modify`, `unlink`
   (`remove`, `merge` and `supersede` are for the owner), a unique `id`, and
   `base_version` set to the version you read. Each `modify` states `path`,
   `before` and `after` and fails if `before` does not match.
3. Use only classes and relations the ontology registers (for example
   `TestReceipt`, `AcceptanceReceipt`, `ChangeSpecItem`, `RepoSnapshot` and
   relations such as `implements`, `verifies`, `has_scenario`); an unknown
   class makes the proposal fail validation.
4. Call `changeset_propose {changeset, base_snapshot}`. It **dry-runs** the
   ChangeSet, stores a proposal and returns `proposal_id`, `ok` and a `report`.
   It never commits. If `ok` is false (`SAD_READONLY`, `VERSION_CONFLICT`,
   `PRECONDITION_FAILED`, `VALIDATION_FAILED`), fix the cause or report it;
   do not retry blindly.
5. Record `proposal_id`, `ok` and the touched ids in the task result.

## 4. Effects to report

Committing a ChangeSet marks the touched elements, and elements connected to
them, as needing review again. In a hub-shaped graph that can be most of the
connected part. State the number of touched ids in your result so the owner can
judge before committing.

## Never

- Never run `tdd-agent kg import`, `generate`, `review`, `change` or any
  approval; never call `run_start`, `run_resume` or `approval_submit`.
- Never edit `.tdd-agent/`, the SAD, or an element's text in a ChangeSet when
  the text comes from a document; change the document instead.
- Never claim the KG is updated: say "proposed" with the proposal id, or
  "documents changed, regenerate needed".
