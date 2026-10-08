import type {
  ErdColumn,
  ErdModel,
  ErdTable
} from '../../../../../shared/code-intel-architecture-types'

export function erdColumn(name: string, over: Partial<ErdColumn> = {}): ErdColumn {
  return {
    name,
    type: 'text',
    canonicalType: 'text',
    nullable: false,
    isPk: false,
    isFk: false,
    generated: false,
    ...over
  }
}

export function erdTable(
  name: string,
  columns: ErdColumn[],
  over: Partial<ErdTable> = {}
): ErdTable {
  return {
    name,
    schema: 'infra',
    columns,
    pk: columns.filter((c) => c.isPk).map((c) => c.name),
    indexes: [],
    checks: [],
    rls: [],
    rlsState: 'none',
    tenantScoped: false,
    degraded: false,
    firstMigration: '0001',
    lastMigration: '0002',
    accessedBy: [],
    ...over
  }
}

/** Three tables, one FK, one cross-service logical link, three column changes. */
export function sampleErdModel(over: Partial<ErdModel> = {}): ErdModel {
  return {
    service: 'infra-fleet',
    dialect: 'postgres',
    schema: 'infra',
    asOfMigration: '0042',
    tables: [
      erdTable('dev_servers', [
        erdColumn('id', { isPk: true, type: 'uuid', canonicalType: 'uuid' }),
        erdColumn('vault_ssh_role'),
        erdColumn('status', { type: 'text', defaultExpr: "'password=hunter2'" })
      ]),
      erdTable(
        'agents',
        [erdColumn('id', { isPk: true }), erdColumn('dev_server_id', { isFk: true })],
        {
          accessedBy: [
            {
              symbol: { key: 'sym:a', kind: 'function', name: 'ListAgents', filePath: 'a.go' },
              op: 'read',
              confidence: 0.9
            },
            {
              symbol: { key: 'sym:b', kind: 'function', name: 'SaveAgent', filePath: 'b.go' },
              op: 'readwrite',
              confidence: 0.8
            }
          ]
        }
      ),
      erdTable('audit', [erdColumn('id', { isPk: true })])
    ],
    relations: [
      {
        kind: 'fk',
        from: { table: 'agents', columns: ['dev_server_id'] },
        to: { table: 'dev_servers', columns: ['id'] },
        crossService: false,
        source: 'ddl',
        confidence: 1
      },
      {
        kind: 'logical',
        from: { table: 'dev_servers', columns: ['tenant_id'] },
        to: { service: 'auth', table: 'tenants', columns: ['id'] },
        crossService: true,
        source: 'declared',
        confidence: 1
      }
    ],
    externalRefs: [{ service: 'auth', table: 'tenants' }],
    changes: [
      {
        table: 'dev_servers',
        column: 'vault_ssh_role',
        kind: 'added',
        migrationFile: 'm/0042.sql',
        line: 3
      },
      {
        table: 'dev_servers',
        column: 'status',
        kind: 'modified',
        migrationFile: 'm/0042.sql',
        before: { type: 'varchar(32)', nullable: false },
        after: { type: 'text', nullable: false }
      },
      {
        table: 'dev_servers',
        column: 'old_flag',
        kind: 'removed',
        migrationFile: 'm/0042.sql',
        before: { type: 'bool', nullable: true }
      }
    ],
    warnings: [],
    ...over
  }
}
