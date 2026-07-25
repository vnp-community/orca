import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PostgreSQLAdapter } from '../pg-adapter'

const mockClient = {
  query: vi.fn(),
  connect: vi.fn().mockResolvedValue(undefined),
  end: vi.fn().mockResolvedValue(undefined)
}

// pg.Client must be a real class constructor (not a plain function)
class MockPgClient {
  query = mockClient.query
  connect = mockClient.connect
  end = mockClient.end
}

vi.mock('pg', () => ({ Client: MockPgClient }))

describe('PostgreSQLAdapter (mocked pg)', () => {
  let adapter: PostgreSQLAdapter

  beforeEach(async () => {
    vi.clearAllMocks()
    mockClient.query.mockResolvedValue({ rows: [], rowCount: 0 })
    adapter = await PostgreSQLAdapter.connect({
      host: 'localhost',
      port: 5432,
      database: 'test',
      username: 'postgres',
      password: 'pass'
    })
  })

  it('capabilities.dialect is postgresql', () => {
    expect(adapter.capabilities.dialect).toBe('postgresql')
  })

  it('capabilities.returning is true', () => {
    expect(adapter.capabilities.returning).toBe(true)
  })

  it('query() returns rows', async () => {
    mockClient.query.mockResolvedValueOnce({ rows: [{ id: 1 }], rowCount: 1 })
    const rows = await adapter.query('SELECT * FROM t')
    expect(rows).toEqual([{ id: 1 }])
  })

  it('query() passes params', async () => {
    mockClient.query.mockResolvedValueOnce({ rows: [], rowCount: 0 })
    await adapter.query('SELECT * FROM t WHERE id = $1', [1])
    expect(mockClient.query).toHaveBeenCalledWith('SELECT * FROM t WHERE id = $1', [1])
  })

  it('exec() calls client.query', async () => {
    await adapter.exec('CREATE TABLE t (id INT)')
    expect(mockClient.query).toHaveBeenCalledWith('CREATE TABLE t (id INT)')
  })

  it('transaction() commits on success', async () => {
    await adapter.transaction(async () => {})
    expect(mockClient.query).toHaveBeenCalledWith('BEGIN')
    expect(mockClient.query).toHaveBeenCalledWith('COMMIT')
  })

  it('transaction() rolls back on error', async () => {
    await expect(
      adapter.transaction(async () => {
        throw new Error('pg fail')
      })
    ).rejects.toThrow('pg fail')
    expect(mockClient.query).toHaveBeenCalledWith('ROLLBACK')
  })

  it('close() calls client.end()', async () => {
    await adapter.close()
    expect(mockClient.end).toHaveBeenCalledOnce()
  })
})
