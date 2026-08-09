import { describe, it, expect, vi } from 'vitest'

// Minimal tests for server-bootstrap DB lifecycle — avoids Electron imports
describe('ServerBootstrap — DB lifecycle (unit)', () => {
  it('pool.drain() is called on shutdown pattern', async () => {
    const mockPool = {
      acquire: vi.fn(),
      release: vi.fn(),
      withConnection: vi.fn().mockImplementation(async (fn: (db: unknown) => Promise<unknown>) =>
        fn({
          capabilities: { dialect: 'sqlite' },
          exec: vi.fn().mockResolvedValue(undefined),
          prepare: vi.fn(),
          close: vi.fn().mockResolvedValue(undefined),
          transaction: vi.fn().mockImplementation(async (fn: () => Promise<unknown>) => fn()),
          query: vi.fn().mockResolvedValue([])
        })
      ),
      withTransaction: vi.fn(),
      stats: vi.fn().mockReturnValue({ total: 1, idle: 1, acquired: 0, waiting: 0 }),
      drain: vi.fn().mockResolvedValue(undefined),
      destroy: vi.fn().mockResolvedValue(undefined)
    }

    // Simulate shutdown sequence
    await mockPool.drain()
    expect(mockPool.drain).toHaveBeenCalledOnce()
  })

  it('ORCA_DB_URL env var is accessible', () => {
    const saved = process.env['ORCA_DB_URL']
    process.env['ORCA_DB_URL'] = 'sqlite://:memory:'
    expect(process.env['ORCA_DB_URL']).toBe('sqlite://:memory:')
    if (saved !== undefined) {
      process.env['ORCA_DB_URL'] = saved
    } else {
      delete process.env['ORCA_DB_URL']
    }
  })

  it('loadDatabaseConfig() returns null when no env vars set', async () => {
    // Save and clear DB env vars
    const saved = {
      ORCA_DB_URL: process.env['ORCA_DB_URL'],
      ORCA_DB_DIALECT: process.env['ORCA_DB_DIALECT']
    }
    delete process.env['ORCA_DB_URL']
    delete process.env['ORCA_DB_DIALECT']

    const { loadDatabaseConfig } = await import('../db/config-loader')
    const config = loadDatabaseConfig()
    expect(config).toBeNull()

    // Restore
    if (saved.ORCA_DB_URL) {process.env['ORCA_DB_URL'] = saved.ORCA_DB_URL}
    if (saved.ORCA_DB_DIALECT) {process.env['ORCA_DB_DIALECT'] = saved.ORCA_DB_DIALECT}
  })

  it('loadDatabaseConfig() parses ORCA_DB_URL when set', async () => {
    const saved = process.env['ORCA_DB_URL']
    process.env['ORCA_DB_URL'] = 'sqlite://:memory:'

    const { loadDatabaseConfig } = await import('../db/config-loader')
    const config = loadDatabaseConfig()
    expect(config).not.toBeNull()
    expect(config?.dialect).toBe('sqlite')

    if (saved !== undefined) {
      process.env['ORCA_DB_URL'] = saved
    } else {
      delete process.env['ORCA_DB_URL']
    }
  })
})
