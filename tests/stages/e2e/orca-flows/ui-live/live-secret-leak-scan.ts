/**
 * "No secret in what the screen or the activity stream shows." Same notion of a secret as the acceptance suite's AST-02
 * (suite_ai_stub_login.py): a well-known credential shape, or the value of a stack/process variable whose name says it is
 * one. Findings name the variable, never its value.
 */
import { readStackEnv } from './live-stack-config'

const SECRET_SHAPES: { name: string; re: RegExp }[] = [
  { name: 'anthropic-api-key-shape', re: /sk-ant-[A-Za-z0-9_-]{10,}/ },
  { name: 'private-key-block', re: /-----BEGIN [A-Z ]*PRIVATE KEY-----/ },
  { name: 'bearer-jwt', re: /Bearer\s+eyJ[A-Za-z0-9_-]{10,}/ }
]
const SECRET_NAME_HINTS = ['PASSWORD', 'TOKEN', 'SECRET', 'KEY']
const MIN_SECRET_LENGTH = 8

export type SecretCatalog = { values: { name: string; value: string }[] }

/** Secret-looking variables of deploy/dev/.env and of this process's environment (values of 8+ characters). */
export function loadSecretCatalog(): SecretCatalog {
  const merged = { ...readStackEnv(), ...process.env } as Record<string, string | undefined>
  const values = Object.entries(merged)
    .filter(
      ([name, value]) =>
        typeof value === 'string' &&
        value.length >= MIN_SECRET_LENGTH &&
        SECRET_NAME_HINTS.some((h) => name.toUpperCase().includes(h))
    )
    .map(([name, value]) => ({ name, value: value as string }))
  return { values }
}

/** What leaked into `text`, by variable name or shape name only. */
export function findSecrets(catalog: SecretCatalog, text: string): string[] {
  const found = new Set<string>()
  for (const { name, value } of catalog.values) {
    if (text.includes(value)) {
      found.add(`value of ${name}`)
    }
  }
  for (const shape of SECRET_SHAPES) {
    if (shape.re.test(text)) {
      found.add(shape.name)
    }
  }
  return [...found]
}

/** The scan the agent-session monitor runs over everything it read from the screen and the activity frames, with its log line. */
export function scanForSecrets(
  texts: string[],
  frameText: string
): { note: string; leaks: string[] } {
  const catalog = loadSecretCatalog()
  const leaks = findSecrets(catalog, `${texts.join('\n')}\n${frameText}`)
  return {
    leaks,
    note: `secret scan: ${catalog.values.length} secret-looking variable(s) checked against ${texts.length} screen read(s) and the activity frames: ${leaks.length === 0 ? 'clean' : `LEAK ${leaks.join(', ')}`}`
  }
}
