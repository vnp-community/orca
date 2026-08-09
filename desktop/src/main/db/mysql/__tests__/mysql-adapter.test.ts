import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MySQLAdapter } from '../mysql-adapter'

// Mock mysql2 for unit tests (no real DB needed)
const mockConn = {
  execute: vi.fn(),
  beginTransaction: vi.fn().mockResolvedValue(undefined),
  commit: vi.fn().mockResolvedValue(undefined),
  rollback: vi.fn().mockResolvedValue(undefined),
  end: vi.fn().mockResolvedValue(undefined)
}

vi.mock('mysql2/promise', () => ({
  createConnection: vi.fn().mockResolvedValue(mockConn)
}))

describe('MySQLAdapter (mocked mysql2)', () => {
  let adapter: MySQLAdapter

  beforeEach(async () => {
    vi.clearAllMocks()
    adapter = await MySQLAdapter.connect({
      host: 'localhost',
      port: 3306,
      database: 'test',
      username: 'root',
      password: 'pass'
    })
  })

  it('capabilities.dialect is mysql', () => {
    expect(adapter.capabilities.dialect).toBe('mysql')
  })

  it('capabilities.nativeJson is true', () => {
    expect(adapter.capabilities.nativeJson).toBe(true)
  })

  it('query() calls execute and returns rows', async () => {
    mockConn.execute.mockResolvedValueOnce([[{ id: 1, name: 'test' }], []])
    const rows = await adapter.query('SELECT * FROM users')
    expect(rows).toEqual([{ id: 1, name: 'test' }])
    expect(mockConn.execute).toHaveBeenCalledWith('SELECT * FROM users', [])
  })

  it('query() passes params to execute', async () => {
    mockConn.execute.mockResolvedValueOnce([[{ id: 1 }], []])
    await adapter.query('SELECT * FROM users WHERE id = ?', [1])
    expect(mockConn.execute).toHaveBeenCalledWith(
      'SELECT * FROM users WHERE id = ?',
      [1]
    )
  })

  it('exec() calls execute', async () => {
    mockConn.execute.mockResolvedValueOnce([{ affectedRows: 0 }, []])
    await adapter.exec('CREATE TABLE t (id INT)')
    expect(mockConn.execute).toHaveBeenCalledWith('CREATE TABLE t (id INT)')
  })

  it('transaction() commits on success', async () => {
    mockConn.execute.mockResolvedValue([{ affectedRows: 1 }, []])
    await adapter.transaction(async () => {
      await adapter.exec('INSERT INTO t VALUES (1)')
    })
    expect(mockConn.beginTransaction).toHaveBeenCalledOnce()
    expect(mockConn.commit).toHaveBeenCalledOnce()
    expect(mockConn.rollback).not.toHaveBeenCalled()
  })

  it('transaction() rolls back on error', async () => {
    await expect(
      adapter.transaction(async () => {
        throw new Error('tx fail')
      })
    ).rejects.toThrow('tx fail')
    expect(mockConn.rollback).toHaveBeenCalledOnce()
    expect(mockConn.commit).not.toHaveBeenCalled()
  })

  it('close() calls connection.end()', async () => {
    await adapter.close()
    expect(mockConn.end).toHaveBeenCalledOnce()
  })

  it('prepare() returns statement with all()', async () => {
    mockConn.execute.mockResolvedValueOnce([[{ val: 42 }], []])
    const stmt = await adapter.prepare('SELECT val FROM t')
    const rows = await stmt.all()
    expect(rows).toEqual([{ val: 42 }])
  })
})
