# BACKLOG-020: Automations "PR merged" trigger — needs a detection-mechanism decision + trigger_depth design

**Origin:** `specs/backend-go/crs/v4/automations/tasks/TASK-BE-AUTO-009-pr-merged-source-and-circular-guard.md` (full survey detail, including the confirmed absence of any existing mechanism)
**Priority:** Low-Medium — automation triggers on other events already work; this is the one still-missing trigger source
**Blocked on:** **A product/architecture decision** (webhook vs. polling, re-estimated at real scope) plus **an open design question** (how `trigger_depth` propagates through an automation chain that has zero real callers today)
**Owner:** whoever owns automation-service/scm-integration-service's roadmap — this is bigger than the 1-task scope it was originally estimated at

---

## What this is

`automation-service`'s `HandleExternalTrigger` usecase is built and can
dispatch a run when called, but nothing calls it for the "a pull request
just got merged" trigger case yet. The task that was meant to wire this up
(`TASK-BE-AUTO-009`) assumed `scm-integration-service` already had *some*
existing PR-status-detection mechanism (webhook or polling, built for an
unrelated purpose like UI status display) it could just add one more call
onto.

## What the investigation found (confirmed independently twice — 2026-09-09)

`scm-integration-service` has **no such mechanism at all**:
- No webhook receiver — its proto only exposes client-initiated RPCs
  (`ListPullRequests`, `MergePullRequest`, `GetPullRequestForBranch`, etc.);
  `MergePullRequest` is the app *initiating* a merge, not detecting one
  happening on GitHub/GitLab's side.
- No polling/scheduler code anywhere in the service.
- No NATS/eventbus publishing tied to PR state changes.
- A `scm.webhook_delivery_log` table already exists in the schema
  (`migrations/0001_init.up.sql`), pre-built per spec anticipating a future
  webhook receiver — but nothing writes to it; the receiver itself was never
  built.

So this is not "wire 1 more call into something that exists" — it's "build
a webhook receiver (or a polling loop) from scratch," a materially bigger
scope than the task doc assumed.

## The two things that need deciding before this can be a real task

1. **Detection mechanism — webhook or polling?**
   - Webhook (the better long-term fit, and the delivery-log table is
     already scaffolded for it): needs a new HTTP ingress route, signature
     verification, and per-provider payload parsing (GitHub and GitLab
     shapes differ).
   - Polling: needs a scheduler, a "have I seen this merge already" dedup
     store, and burns API rate limit per watched repo.
   - Either way, re-estimate effort at this real scope, not the original
     task's "add 1 call" estimate.

2. **How does `trigger_depth` propagate through a chain?** The circular-
   trigger guard this task also wants (stop automation A's `create_pr` →
   PR merged → automation B's `HandleExternalTrigger` → ... from looping
   forever) needs some field carrying "which automation triggered this, at
   what depth" through the chain. Nothing carries that today, and
   `TASK-BE-AUTO-008`'s own survey found **zero real service-to-service
   callers of `HandleExternalTrigger` today** — so there's no live case to
   design the field against yet. Building a guard for a scenario that can't
   happen yet is speculative design, not implementation.

## What unblocks this

1. Pick webhook vs. polling (re-scoped), and get that scoped as its own
   real initiative (webhook ingress + verification, or scheduler + dedup
   store — either is a multi-task effort on its own).
2. Design `trigger_depth`'s propagation shape once there's a concrete first
   real chain to design against (likely once step 1 produces the first real
   external trigger caller).
3. Only then should `TASK-BE-AUTO-009` (or a replacement task) move off
   BLOCKED.
