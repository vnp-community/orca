# BUG-FLEET-01: No YAML fleet-inventory import — servers can only be registered one at a time via API

**Business Logic:** [BL-FLEET-01](../../../../docs/logic/fleet/BL-FLEET-01-fleet-inventory.md) — Fleet Inventory Config (YAML)
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** An Admin/DevOps user who wants to declare their dev-server fleet as infrastructure-as-code (`orca-fleet.yaml` with `defaults`/`projects`/`servers` sections) has no way to do so against backend-go. There is no `orca fleet import/list/status` CLI, no YAML parser, no upsert-by-`hostname+user` semantics, and no `project`/`tags` grouping — every server must be registered individually through `devServer.add`/`ssh.*`, and there is nowhere to attach a project or tag to it at all.

---

## Spec summary

BL-FLEET-01 describes declaring a fleet of dev servers as a YAML file (`deploy/dev/orca-fleet.yaml`) with `defaults` (e.g. `relayGracePeriodSec`, `nodeVersion`), a `projects[]` list, and a `servers[]` list (hostname, user, identityFile, project, tags, port). `orca fleet import --file <path> [--dry-run]` parses and Zod-validates the file, then upserts each server into the SSH targets store (insert-or-update by `hostname+user`), tags it by project/tags, and reports `{ imported, updated, skipped, errors }`. `orca fleet list [--project <name>]` and `orca fleet status` round out the CLI.

## What backend-go has

- `CreateSshTarget` and `RegisterDevServer` RPCs (`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:16-17,102-109`) let a caller register **one** SSH target / dev server per call, backed by real usecases (`backend-go/services/infra-fleet-service/internal/usecase/create_ssh_target.go`, `register_dev_server.go:26-50`) and Postgres persistence (`backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go`).
- `ListSshTargets`/`ListDevServers` RPCs back `ssh.listTargets` and `devServer.list` in the wscompat layer (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:331`, `channels.go:436`).
- The domain models are deliberately narrow: `domain.SshTarget` is `{ID, TenantID, Host, UserName, VaultSSHRole}` (`backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:22-29`) and `domain.DevServer` is `{ID, TenantID, Host, Mode, SSHTargetID}` (`backend-go/services/infra-fleet-service/internal/domain/dev_server.go:48-54`, whose own doc comment says "display name, bootstrap status, agent version are not modeled here"). Neither has a `project`, `tags`, `identityFile`, or `port` field.

## What's missing

- No YAML parser or schema validation equivalent to `FleetConfig`/`FleetServer` (the spec's Zod schemas) anywhere in `backend-go/` — confirmed by grep for `FleetConfig`, `orca-fleet`, `parseFleetYaml`, `fleet.import` returning zero hits in `backend-go/`.
- No `orca fleet import|list|status` CLI — `infra-fleet-service/cmd/server/main.go` is a gRPC server composition root only, no subcommands (`grep` for `flag\.|cobra|Command(` returns nothing).
- No batch/upsert-by-`hostname+user` import path, no `--dry-run` mode, no `{imported, updated, skipped, errors}` summary shape.
- No `project`/`tags` concept anywhere in the domain or proto — a server registered via `devServer.add`/`CreateSshTarget` cannot be grouped or filtered by project (`ssh.listTargets`/`devServer.list` return the full unfiltered set, no `--project` equivalent).
- The only `orca-fleet.yaml` in the repo is a legacy file under `/opt/repos/orca/deploy/old/orca-fleet.yaml`, outside `backend-go/` and outside any code path that reads it.

## See also

None of the existing `specs/backend-go/bugs/missing-v1|api-v1` reports cover fleet-inventory import specifically — closest overlap is BUG-003 (`devServer.list` timeout, already fixed) and BUG-024 (`ssh.*` channels, now wired — see `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:331-383`), but neither concerns bulk YAML import.

## References

- `docs/logic/fleet/BL-FLEET-01-fleet-inventory.md`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:16-17,88-99,102-109` — `CreateSshTarget`/`RegisterDevServer` RPC and message shapes
- `backend-go/services/infra-fleet-service/internal/domain/ssh_target.go:22-29` — `SshTarget` struct, no project/tags
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go:48-54` — `DevServer` struct, no project/tags
- `backend-go/services/infra-fleet-service/internal/usecase/register_dev_server.go:26-50`, `create_ssh_target.go` — one-at-a-time registration usecases
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:436-451` — `devServer.list`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go:331` — `ssh.listTargets`
- `backend-go/services/infra-fleet-service/cmd/server/main.go` — no CLI subcommands
- `/opt/repos/orca/deploy/old/orca-fleet.yaml` — legacy, unreferenced by backend-go
