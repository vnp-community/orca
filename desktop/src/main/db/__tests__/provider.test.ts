import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  registerDatabaseProvider,
  getDatabaseProvider,
  createDatabase,
  clearProviderRegistry,
  getRegisteredDialects
} from '../provider'
import type { DatabaseProvider, IDatabase } from '../types'

function makeMockDb(): IDatabase {
  return {
    capabilities: {
      dialect: 'sqlite',
      walMode: true,
      returning: false,
      nativeJson: false,
      placeholderStyle: 'positional'
    },
    exec: () => {},
    prepare: () => ({
      run: () => ({ changes: 0, lastInsertRowid: 0 }),
      get: () => undefined,
      all: () => []
    }),
    close: () => {},
    transaction: async (fn) => fn(),
    query: async () => []
  }
}

describe('DatabaseProvider Registry', () => {
  beforeEach(() => {
    clearProviderRegistry()
  })

  it('getRegisteredDialects() returns empty array initially', () => {
    expect(getRegisteredDialects()).toEqual([])
  })

  it('registerDatabaseProvider() registers a provider', () => {
    const provider: DatabaseProvider = { dialect: 'sqlite', connect: async () => makeMockDb() }
    registerDatabaseProvider(provider)
    expect(getDatabaseProvider('sqlite')).toBe(provider)
  })

  it('getDatabaseProvider() throws for unregistered dialect', () => {
    expect(() => getDatabaseProvider('mysql')).toThrow(
      'No database provider registered for dialect: "mysql"'
    )
  })

  it('getDatabaseProvider() error message lists available dialects', () => {
    registerDatabaseProvider({ dialect: 'postgresql', connect: async () => makeMockDb() })
    try {
      getDatabaseProvider('mysql')
    } catch (err) {
      expect((err as Error).message).toContain('postgresql')
    }
  })

  it('createDatabase() delegates to provider.connect()', async () => {
    const mockDb = makeMockDb()
    const connectSpy = vi.fn().mockResolvedValue(mockDb)
    registerDatabaseProvider({ dialect: 'sqlite', connect: connectSpy })

    const db = await createDatabase({ dialect: 'sqlite', path: ':memory:', readonly: false })
    expect(db).toBe(mockDb)
    expect(connectSpy).toHaveBeenCalledOnce()
  })

  it('registerDatabaseProvider() overwrites existing provider for same dialect', () => {
    const p1: DatabaseProvider = { dialect: 'sqlite', connect: async () => makeMockDb() }
    const p2: DatabaseProvider = { dialect: 'sqlite', connect: async () => makeMockDb() }
    registerDatabaseProvider(p1)
    registerDatabaseProvider(p2)
    expect(getDatabaseProvider('sqlite')).toBe(p2)
  })

  it('getRegisteredDialects() lists all registered dialects', () => {
    registerDatabaseProvider({ dialect: 'sqlite', connect: async () => makeMockDb() })
    registerDatabaseProvider({ dialect: 'mysql', connect: async () => makeMockDb() })
    const dialects = getRegisteredDialects()
    expect(dialects).toContain('sqlite')
    expect(dialects).toContain('mysql')
  })

  it('clearProviderRegistry() removes all providers', () => {
    registerDatabaseProvider({ dialect: 'sqlite', connect: async () => makeMockDb() })
    clearProviderRegistry()
    expect(getRegisteredDialects()).toHaveLength(0)
  })
})
