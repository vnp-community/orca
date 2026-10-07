import path from 'node:path'
import fs from 'node:fs'
import { Rule } from './quality-rule-pack-schema'

export function locateScript(root: string, scriptRule: Rule['script']): string | null {
  if (!scriptRule) return null
  
  const name = scriptRule.name
  const paths = [
    path.join(root, 'config/scripts', `${name}.mjs`),
    path.join(root, 'desktop/config/scripts', `${name}.mjs`)
  ]
  
  for (const p of paths) {
    if (fs.existsSync(p)) {
      try {
        return fs.realpathSync(p)
      } catch {
        return null
      }
    }
  }
  
  return null
}
