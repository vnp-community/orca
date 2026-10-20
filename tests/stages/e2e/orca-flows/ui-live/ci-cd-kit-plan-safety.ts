/**
 * What a Plan may not ask the agent to do on the owner's machine, read before "Approve and run all steps". The agent runs with
 * every permission, and the owner's own stack (docker containers qa0210-*) runs on the same machine, so commands that start, stop
 * or deploy anything, push, or reach outside the repo are refused; words in a CI example (curl in a YAML file) are not commands.
 */

const NEGATION = /\b(do not|don't|never|must not|without|avoid|no need to)\b|không|đừng|tránh|cấm/i

const UNSAFE: { name: string; re: RegExp }[] = [
  { name: 'git push', re: /\bgit\s+push\b/i },
  {
    name: 'git remote/config write',
    re: /\bgit\s+(remote\s+(add|set-url)|config\s+--(global|system))\b/i
  },
  { name: 'sudo', re: /\bsudo\b/i },
  { name: 'rm -rf outside the repo', re: /\brm\s+-[a-z]*r[a-z]*f?\s+(\/|~|\$HOME|\.\.)/i },
  {
    name: 'docker / compose lifecycle',
    re: /\bdocker(-compose)?\s+(compose\s+)?(up|run|start|stop|restart|build|exec|down|rm|kill)\b/i
  },
  { name: 'cluster or infra tool', re: /\b(kubectl|helm|terraform|ansible)\b/i },
  {
    name: 'release / publish',
    re: /\b(npm\s+publish|gh\s+(pr|release|workflow|run)\b|make\s+(deploy|up)\b)/i
  },
  { name: 'path outside the repo', re: /(\/Users\/|~\/|\$HOME\b)/ }
]

/** The clause around a hit, to tell "docker compose up" from "do not run docker compose up". */
function negated(text: string, at: number): boolean {
  const before = text.slice(Math.max(0, at - 160), at)
  const sentenceStart = Math.max(
    before.lastIndexOf('.'),
    before.lastIndexOf('\n'),
    before.lastIndexOf(';')
  )
  return NEGATION.test(before.slice(sentenceStart + 1))
}

/** Names of the unsafe things the text asks for (empty = nothing to stop the run); `allowed` paths are taken out first. */
export function unsafePlanText(text: string, allowedPaths: string[]): string[] {
  const clean = allowedPaths.reduce((t, p) => t.replaceAll(p, ''), text)
  const hits: string[] = []
  for (const { name, re } of UNSAFE) {
    const flags = re.flags.includes('g') ? re.flags : `${re.flags}g`
    for (const m of clean.matchAll(new RegExp(re.source, flags))) {
      if (!negated(clean, m.index ?? 0)) {
        hits.push(`${name}: "${m[0]}"`)
        break
      }
    }
  }
  return hits
}
