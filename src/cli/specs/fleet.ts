// src/cli/specs/fleet.ts
import type { CommandSpec } from '../args'
import { GLOBAL_FLAGS } from '../args'

export const FLEET_COMMAND_SPECS: CommandSpec[] = [
  {
    path: ['fleet', 'import'],
    summary: 'Import servers from a fleet config YAML file',
    usage: 'orca fleet import <config-file> [--dry-run] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'dry-run'],
    examples: [
      'orca fleet import deploy/dev/orca-fleet.yaml',
      'orca fleet import deploy/dev/orca-fleet.yaml --dry-run',
    ],
  },
  {
    path: ['fleet', 'provision'],
    summary: 'Deploy Orca relay to fleet servers (connect + relay deploy)',
    usage: 'orca fleet provision [--all] [--project <name>] [--server <id>] [--concurrency <n>] [--dry-run] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'all', 'project', 'server', 'concurrency', 'dry-run'],
    examples: [
      'orca fleet provision --all',
      'orca fleet provision --project vnp-blc',
      'orca fleet provision --server dev-alpha',
    ],
  },
  {
    path: ['fleet', 'status'],
    summary: 'Show health status of all fleet servers',
    usage: 'orca fleet status [--project <name>] [--team <name>] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'project', 'team'],
  },
  {
    path: ['fleet', 'list'],
    summary: 'List all servers in fleet',
    usage: 'orca fleet list [--project <name>] [--team <name>] [--environment <env>] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'project', 'team', 'environment'],
  },
  {
    path: ['fleet', 'sync'],
    summary: 'Sync fleet config — add/update servers to match config file',
    usage: 'orca fleet sync <config-file> [--dry-run] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'dry-run'],
    examples: [
      'orca fleet sync deploy/dev/orca-fleet.yaml',
      'orca fleet sync deploy/dev/orca-fleet.yaml --dry-run',
    ],
  },
  {
    path: ['fleet', 'bootstrap'],
    summary: 'Install dependencies and clone repos on a fleet server',
    usage: 'orca fleet bootstrap [--server <id>] [--all] [--config <fleet.yaml>] [--skip-node] [--skip-git] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'server', 'all', 'config', 'skip-node', 'skip-git'],
    examples: [
      'orca fleet bootstrap --server dev-alpha --config deploy/dev/orca-fleet.yaml',
      'orca fleet bootstrap --all --config deploy/dev/orca-fleet.yaml',
    ],
  },
]
