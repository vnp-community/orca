import fs from 'fs'
import path from 'path'

let DatabaseSync: any
try {
  DatabaseSync = require('node:sqlite').DatabaseSync
} catch {
  DatabaseSync = null
}

const dbCache = new Map<string, { db: any, timer: NodeJS.Timeout }>()

export function isSqliteReadAvailable(projectPath: string): boolean {
  if (!DatabaseSync) return false
  if (process.env.ORCA_CODEINTEL_SQLITE === 'off') return false
  
  const dbPath = path.join(projectPath, '.codegraph', 'codegraph.db')
  if (!fs.existsSync(dbPath)) return false

  let db
  try {
    db = new DatabaseSync(dbPath, { readOnly: true })
    
    const extractionStmt = db.prepare("SELECT value FROM project_metadata WHERE key = 'extractionVersion'")
    const extRes = extractionStmt.get()
    if (!extRes || String(extRes.value) !== '24') {
      return false
    }

    const schemaStmt = db.prepare("SELECT MAX(version) as max_version FROM schema_versions")
    const schemaRes = schemaStmt.get()
    if (!schemaRes || schemaRes.max_version > 8) {
      return false
    }

    return true
  } catch (err) {
    if (process.env.ORCA_CODEINTEL_SQLITE === 'auto' || process.env.ORCA_CODEINTEL_SQLITE === 'on') {
      console.warn(`[CodeGraph] ExperimentalWarning: Failed to check SQLite db ${dbPath}:`, err)
    }
    return false
  } finally {
    if (db) {
      try { db.close() } catch {}
    }
  }
}

export function openCodeGraphDb(projectPath: string) {
  if (!DatabaseSync) return null
  if (process.env.ORCA_CODEINTEL_SQLITE === 'off') return null
  
  const dbPath = path.join(projectPath, '.codegraph', 'codegraph.db')
  if (!fs.existsSync(dbPath)) return null

  const cached = dbCache.get(projectPath)
  if (cached) {
    clearTimeout(cached.timer)
    cached.timer = setTimeout(() => closeCodeGraphDb(projectPath), 60000)
    return cached.db
  }

  let db
  try {
    db = new DatabaseSync(dbPath, { readOnly: true })
    db.exec('PRAGMA busy_timeout = 2000') // timeout: 2000
    
    // Check extractionVersion and schema version
    const extractionStmt = db.prepare("SELECT value FROM project_metadata WHERE key = 'extractionVersion'")
    const extRes = extractionStmt.get()
    if (!extRes || String(extRes.value) !== '24') {
      db.close()
      return null
    }

    const schemaStmt = db.prepare("SELECT MAX(version) as max_version FROM schema_versions")
    const schemaRes = schemaStmt.get()
    if (!schemaRes || schemaRes.max_version > 8) {
      db.close()
      return null
    }
  } catch (err) {
    return null
  }

  const timer = setTimeout(() => closeCodeGraphDb(projectPath), 60000)
  dbCache.set(projectPath, { db, timer })
  return db
}

export function closeCodeGraphDb(projectPath: string) {
  const cached = dbCache.get(projectPath)
  if (cached) {
    clearTimeout(cached.timer)
    try {
      cached.db.close()
    } catch {}
    dbCache.delete(projectPath)
  }
}

export function findNodes(db: any, query: string, limit: number = 1500) {
  const stmt = db.prepare("SELECT id, type, name, path, start_line, end_line, signature, docstring, is_exported FROM nodes WHERE name LIKE ? LIMIT ?")
  return stmt.all(`%${query}%`, limit)
}

export function callersById(db: any, id: string, limit: number = 1500) {
  const stmt = db.prepare(`
    SELECT n.id, n.type, n.name, n.path, n.start_line, n.end_line, n.signature, n.docstring, n.is_exported 
    FROM edges e 
    JOIN nodes n ON e.source = n.id 
    WHERE e.target = ? AND e.type = 'CALLS'
    LIMIT ?
  `)
  return stmt.all(id, limit)
}

export function calleesById(db: any, id: string, limit: number = 1500) {
  const stmt = db.prepare(`
    SELECT n.id, n.type, n.name, n.path, n.start_line, n.end_line, n.signature, n.docstring, n.is_exported 
    FROM edges e 
    JOIN nodes n ON e.target = n.id 
    WHERE e.source = ? AND e.type = 'CALLS'
    LIMIT ?
  `)
  return stmt.all(id, limit)
}
