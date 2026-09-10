# BACKLOG-019: Cloud provider credential storage design — needs security sign-off before any code

**Origin:** `specs/backend-go/crs/v4/fleet-provisioning/tasks/TASK-BE-FLEET-009-cloud-credential-storage-design-security-review.md`, design doc at `specs/backend-go/crs/v4/fleet-provisioning/solutions/BE-FLEET-SOL-004-cloud-credential-storage-design.md` (full detail)
**Priority:** Medium — the one item keeping F31 Fleet Provisioning's backend-go work below fully done (13/14 tasks)
**Blocked on:** **A human security review sign-off**, not engineering time — CR-FLEET-002 self-rates this "High" risk (first time backend-go would hold a raw, long-lived third-party credential, not just a short-lived Vault SSH cert)
**Owner:** whoever does security review at this org — role/name intentionally left open in the design doc (question 6 of 6)

---

## What this is

Fleet provisioning's Terraform-apply flow needs cloud provider credentials
(IAM keys / service-account JSON) to actually run `terraform apply` against
AWS/GCP/etc. Nothing in backend-go today holds a credential like this — the
existing model only ever hands out short-lived Vault SSH certs through the
SSH secrets engine. Storing a long-lived third-party credential is a new
security surface, so no code gets written until a design is reviewed and
signed off.

## Where things stand (2026-09-09)

- **Question 1 of 6 (where to store it) is decided**: Vault KV engine, reusing the
  existing shared Vault instance, but a separate KV mount/path from the SSH
  secrets engine already in use (not the same mount, not a new Postgres
  table with hand-rolled encryption-at-rest).
- **Questions 2–5 have concrete proposals** in `BE-FLEET-SOL-004`, built by
  reusing 100% of `credential-broker-service`'s existing infrastructure
  (rotation/audit/scoping/agent-delivery-by-value pattern) rather than
  inventing anything new.
- **Question 6 (who reviews) is deliberately left open** — no security
  review process has claimed this yet.
- **No sign-off has happened.** The design doc is a proposal, not an
  approved design.

## What unblocks this

1. Identify who actually does security review for backend-go changes at
   this org (answers question 6).
2. That person/team reviews `BE-FLEET-SOL-004`'s proposals for questions
   2–5 (credential lifecycle/rotation/audit, tenant vs. fleet-definition
   scoping, agent delivery channel, and the 4-point threat model — log
   leakage, cross-tenant read, credential surviving past `terraform apply`,
   accidental commit into a git-tracked working dir).
3. Once approved, a new code task (not yet created — deliberately not
   pre-numbered in the fleet-provisioning task set) implements the
   signed-off design. No one should pick a storage schema and start coding
   without this sign-off, per the task's own explicit instruction.
