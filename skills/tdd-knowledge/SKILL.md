---
name: tdd-knowledge
description: >-
  Ground a task in the project's design knowledge before proposing or writing
  anything: look up the requirements, business rules, entities and
  architecture decisions that apply (tdd-agent knowledge graph over MCP,
  read-only tools `kg_search`, `kg_get_element`, `kg_neighbors`,
  `kg_resolve_scope`, `kg_query`, `source_get`). Use it at the start of a
  Request analysis, solution, plan or coding task whenever the MCP server
  `tdd-agent` is available, and cite the element ids and the snapshot in your
  answer. Do not use it to edit knowledge: use `tdd-knowledge-writeback`.
---

# Design knowledge from the tdd-agent knowledge graph

The knowledge graph (KG) is built from the project's URD, SRS and SAD. It tells
you what the documents require and decided. It does not describe the code: use
the code-intelligence tools for that. Everything here is read-only.

## Before you start

1. The tools are named `mcp__tdd-agent__<tool>`. If none exists, say the design
   knowledge was not available and continue without inventing any; never guess
   an element id.
2. Every result carries `snapshot`. Report it with what you cite; if two calls
   return different snapshots the KG changed under you: repeat the first call.
3. Results may carry `truncated` and `next`: repeat the call with
   `offset=<next>` only if the missing part matters. Do not page through a long
   listing "to be complete".

## Find what applies (in this order)

1. **Seeds.** Collect the ids written in the task (`FR-…`, `NFR-…`, `ENT-…`,
   `BRL-…`, `ADR-…`) and 2 to 4 key phrases of the task.
2. **`kg_search`** `{query, classes?, limit?}`: an id (in full or in part) ranks
   first; text matches follow. Use `classes` to narrow:
   `Requirement`, `BusinessRule`, `Entity`, `Decision`, `Constraint`,
   `QualityTarget`, `Component`. `Attribute` elements belong to an `Entity`
   (`ENT-17:attribute:api_url`): read them through their entity.
   Ids such as `UC-…` and `EIF-…` are not elements in this KG: search them as
   text and do not cite them as ids.
3. **`kg_get_element`** `{id}` for the exact statement; quote it, do not
   paraphrase a requirement.
4. **`kg_neighbors`** `{ids, relations?, direction?}` to see what a requirement
   traces to. Each edge carries `provenance` (document block such as
   `line:432`, the quoted source text, `verified`). Prefer this over
   `kg_resolve_scope` for a first look.
5. **`kg_resolve_scope`** `{seeds, budget}` only when you need the full
   evidence closure of one or two seeds, and **always pass `budget`**. A hub
   rule such as `BRL-64` links ten requirements, so the closure of
   `FR-SYS-07` alone is far larger than a prompt. If `blockers` is not empty,
   report it.
6. **`kg_query`** `{name: "Q1".."Q9"}` runs the typed queries (inputs for an
   agent, gaps, drift). Use the `gap` and `drift` results to say what the
   documents leave open; do not treat an empty result as proof.
7. **`source_get`** `{id}` returns a pinned source document as stored; use it
   to read a table row in context, not to copy whole documents.

## What to look for

- **Requirements** the work changes or must not break.
- **Business rules** and **quality targets** that constrain the solution.
- **Decisions and constraints** (ADR): a proposal that contradicts one must say
  so, name the id, and explain why it is acceptable or propose the change to
  the decision. Example: a decision may reject "return 501" as a design.
- **Entities** and their attributes the change touches.
- **Gaps**: required information the documents do not give. List them as open
  questions; never fill them with assumptions.

## How to report

- Cite ids inline (`FR-SYS-07`, `ADR-12`) and end with
  `Knowledge: snapshot <id>; read: <ids>; gaps: <ids or none>`.
- Quote at most one short sentence per element; the ids are the reference.
- Say which part of your answer rests on the KG and which on the code.

## Never

- Never modify `docs/` SRS, SAD, URD or `.tdd-agent/` from this skill.
- Never present an element as a requirement of this task unless you read it.
- Never cite more than you read; if a call failed, say so.
