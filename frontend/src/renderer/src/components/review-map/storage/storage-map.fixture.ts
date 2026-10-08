import type { StorageMap } from '../../../../../shared/code-intel-architecture-types'

const ev = (path: string) => [{ path, line: 1, kind: 'config' as const }]

/** infra-fleet -> postgres/mysql, one topic with two publishers, one Vault key. */
export function sampleStorageMap(over: Partial<StorageMap> = {}): StorageMap {
  const store = (id: string, kind: string, name: string, extra = {}) => ({
    id,
    kind: kind as never,
    name,
    env: 'dev' as const,
    deployed: true,
    supportedByCode: true,
    external: false,
    evidence: ev(`deploy/${id}.yml`),
    confidence: 'declared' as const,
    ...extra
  })
  return {
    stores: [
      store('pg', 'postgres', 'postgres', { owner: { name: 'infra-fleet' }, schemas: ['infra'] }),
      store('my', 'mysql', 'mysql', { owner: { name: 'infra-fleet' } }),
      store('vault', 'vault', 'secret/data/infra/ssh', { evidence: ev('deploy/vault.yml') })
    ],
    bindings: [
      {
        service: 'infra-fleet',
        store: 'pg',
        access: 'rw',
        via: 'postgres://u:p4ss@db/app',
        evidence: ev('svc/infra/config.yml'),
        confidence: 'declared'
      },
      {
        service: 'agent-gw',
        store: 'my',
        access: 'ro',
        via: 'env DB_URL',
        evidence: ev('svc/gw/config.yml'),
        confidence: 'inferred'
      },
      {
        service: 'infra-fleet',
        store: 'vault',
        access: 'ro',
        via: 'vault kv',
        configKey: 'vault_ssh_role',
        evidence: ev('svc/infra/vault.yml'),
        confidence: 'derived'
      }
    ],
    topics: [
      {
        name: 'orca.infra.agent',
        publishers: ['infra-fleet', 'agent-gw'],
        subscribers: ['notifier'],
        delivery: 'durable',
        payload: 'token=abc123',
        evidence: ev('svc/infra/events.go'),
        confidence: 'declared'
      }
    ],
    sources: [],
    redactedCount: 2,
    asOfCommit: 'f00dfeed',
    warnings: ['prod topology unknown'],
    ...over
  }
}
