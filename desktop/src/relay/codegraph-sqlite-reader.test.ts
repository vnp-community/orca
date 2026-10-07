import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import {
  isSqliteReadAvailable,
  openCodeGraphDb,
  closeCodeGraphDb,
  findNodes,
  callersById,
  calleesById
} from './codegraph-sqlite-reader'

let hasNodeSqlite = false
try {
  require('node:sqlite')
  hasNodeSqlite = true
} catch {
  hasNodeSqlite = false
}

describe.skipIf(!hasNodeSqlite)('codegraph-sqlite-reader', () => {
  let tmpDir: string
  let dbPath: string
  let projectPath: string

  beforeEach(() => {
    projectPath = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-codegraph-test-'))
    const cgDir = path.join(projectPath, '.codegraph')
    fs.mkdirSync(cgDir)
    dbPath = path.join(cgDir, 'codegraph.db')
    
    // Create dummy schema
    const { DatabaseSync } = require('node:sqlite')
    const db = new DatabaseSync(dbPath)
    db.exec(`
      CREATE TABLE project_metadata (key TEXT, value TEXT);
      INSERT INTO project_metadata VALUES ('extractionVersion', '24');
      CREATE TABLE schema_versions (version INTEGER);
      INSERT INTO schema_versions VALUES (1), (8);
      CREATE TABLE nodes (id INTEGER PRIMARY KEY, type TEXT, name TEXT, path TEXT, start_line INTEGER, end_line INTEGER, signature TEXT, docstring TEXT, is_exported INTEGER);
      INSERT INTO nodes VALUES (1, 'method', 'foo', 'a.ts', 10, 15, '() => void', 'doc', 1);
      INSERT INTO nodes VALUES (2, 'function', 'bar', 'b.ts', 20, 25, '() => void', '', 1);
      CREATE TABLE edges (source INTEGER, target INTEGER, type TEXT);
      INSERT INTO edges VALUES (1, 2, 'CALLS');
    `)
    db.close()
    
    process.env.ORCA_CODEINTEL_SQLITE = 'auto'
  })

  afterEach(() => {
    closeCodeGraphDb(projectPath)
    try { fs.rmSync(projectPath, { recursive: true, force: true }) } catch {}
  })

  it('detects availability', () => {
    expect(isSqliteReadAvailable(projectPath)).toBe(true)
  })

  it('detects unavailability if extraction version is wrong', () => {
    const { DatabaseSync } = require('node:sqlite')
    const db = new DatabaseSync(dbPath)
    db.exec(`UPDATE project_metadata SET value = '25' WHERE key = 'extractionVersion';`)
    db.close()
    
    expect(isSqliteReadAvailable(projectPath)).toBe(false)
  })

  it('opens db, queries and caches', () => {
    const db = openCodeGraphDb(projectPath)
    expect(db).not.toBeNull()
    
    const nodes = findNodes(db, 'foo')
    expect(nodes.length).toBe(1)
    expect(nodes[0].name).toBe('foo')
    
    const callers = callersById(db, '2')
    expect(callers.length).toBe(1)
    expect(callers[0].name).toBe('foo')

    const callees = calleesById(db, '1')
    expect(callees.length).toBe(1)
    expect(callees[0].name).toBe('bar')
    
    const db2 = openCodeGraphDb(projectPath)
    expect(db2).toBe(db) // cached
  })

  it('returns null if env is off', () => {
    process.env.ORCA_CODEINTEL_SQLITE = 'off'
    expect(openCodeGraphDb(projectPath)).toBeNull()
    expect(isSqliteReadAvailable(projectPath)).toBe(false)
  })
})
