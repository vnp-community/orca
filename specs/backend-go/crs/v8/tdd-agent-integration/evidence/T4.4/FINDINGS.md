# T4.4: what the first real run found (2026-10-15)

Request #1234 (BL-H01, size L) on the pilot project, the real Claude, the local stack. Every item below was invisible to the unit
tests and to the fakes that stood in for git-gateway, infra-fleet and the agent; each was found by a real run and fixed with a test
at the layer that failed. Dollar figures are what the agent reported for the run that hit the problem.

## Defects fixed in code

| # | What happened | Where it was | Commit | Cost of finding it |
|---|---|---|---|---|
| 1 | The plan check failed: `Stat` of a file the plan will create came back as a relay failure ("could not be checked") | infra-fleet turned every agent error of an `fs.*` call into an internal one; git-gateway's relay `Stat` could not tell a missing path | 0439ecf69, e3d243913 | $12.73 + $13.19 (two full plan runs) |
| 2 | Even with that fixed, every path read as missing | git-gateway read the agent's `fs.stat` answer (`size`, `mtime`, `isDir`) into fields named differently, so `exists` was always false | e3d243913 | found by the probe, no cost |
| 3 | A missing directory from `fs.readDir` was a relay failure | the agent gave it no stable code (fs.stat already had `PathNotFound`) | e3d243913 | probe |
| 4 | A commit returned no sha, and a failed commit looked done | the agent answers `git.commit` as `{success}` and git-gateway read it into a struct that has only a sha | e3d243913 | probe |
| 5 | A branch always looked new, then (after the first fix) always looked present; the task worktree failed with `spawn git ENOENT` | git-gateway asked with `show-ref`, which the agent refuses; then read only the RPC error, but the agent answers a git exit code of 1 or 128 as a successful call | 0439ecf69, 4addfd401 | one failed phase start |
| 6 | Ensuring the proposal worktree twice failed with `WORKTREE_PATH_EXISTS` | git-gateway looked a worktree up by idempotency key but never stored the key | 032c8ab3e | a failed plan run |
| 7 | Tasks timed out at five minutes, twice, and the Request went back to the backlog | contract runs sent no `timeoutMs`, so the agent used its default | 5077a3b5a | one Request returned to backlog, redo of Solution and Plan ($10) |
| 8 | A plan run was cut at 15 minutes after spending the whole budget | authoring the change of a size L request takes 13 to 25 minutes; the agent's ceiling, infra-fleet's wait and the attempt budget were 15, 16 and 15 minutes | a174e8fe8 | $0 recorded (cut off) |
| 9 | The tasks.md sync failed every pass with `REQUEST_REPORTER_REQUIRED` | the sweep has no user and project-service wants one | 5077a3b5a | none (found in the log in time) |
| 10 | A Request reopened and planned again opened the gate of the new plan's third phase first | the execution tree took the first Plan task and every phase found, mixing the old and the new plan | 6a13519d5 | none |
| 11 | The stored bundle of the second plan run held two of four specs | the bundle was built from the paths the run changed, not the whole change directory | 51e9c1626 | none |

## Product gaps found, not fixed in this change

- The CLI sign-in token lasts about ten minutes and cannot be renewed; a long wait dies with a signature error while the run goes on.
  The waiting commands now say so and how to read the run; renewing the token belongs to auth-service.
- A checker that cannot run (here, the Stat failure) costs an agent attempt by design (T2.5), and an agent attempt costs $6 or more;
  retrying the check once before spending the attempt would have saved about $13 in this run.
- After `request reopen` and `confirm-type` the Request sits in `analyzing` with no run until `orca solution generate` is called.
- A Solution with a blocking open question could not be answered from the CLI; `orca clarification ask --solution` was added.
- A size L Solution, Plan or task list needs an approver other than the reporter, so a one-user stack cannot pass those gates. The user
  allowed three project-scoped self-approval policies for the pilot project; they are removed with the project.
- The agents leave their code uncommitted and all tasks of one plan share one task worktree, so a branch diff shows nothing. The
  handover lists worktrees; it cannot say what is committed. (`pilot_diff_report.py` reads the working tree.)
- A task run reports no cost and no turns: only analysis and plan runs carry them.
- Local stack: after a service is redeployed, every service that dials it must be restarted too (see the memory note
  `orca-local-stack-gotchas`).

## Findings about the flow itself

- The Solution cited the graph and found real conflicts: ADR-09, ADR-12, BRL-64, NFR-CO-01 and NFR-CO-02 against a feature the CR asks
  for, and eight gaps (no element for quota or cost, none for the `AI_ENABLED` flag). A blocking question about ADR-09 was asked and
  answered by a person (recorded as a pilot assumption, not a product decision); the second Solution did not ask it again.
- The plan has 5 phases and 30 tasks, each with Serves, Code, Verify and Done when; the structure check passed with 5 to 6 knowledge
  conflict warnings, which the approver reads.
- The sync ticks tasks.md on the proposal branch one commit per sweep, touching only that file.
