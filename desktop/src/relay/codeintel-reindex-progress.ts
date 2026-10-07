export function parseProgressLine(line: string): { message: string, percent: number | null } {
  // Strip ANSI
  let msg = line.replace(/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g, '').trim()
  
  const home = process.env.HOME
  if (home && msg.includes(home)) {
    msg = msg.replace(new RegExp(home.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g'), '~')
  }

  let percent: number | null = null
  const m = msg.match(/(?:^|[^\d-])(\d{1,3})%/)
  if (m) {
    const p = parseInt(m[1], 10)
    if (p >= 0 && p <= 100) {
      percent = p
    }
  }

  if (msg.length > 200) {
    msg = msg.substring(0, 197) + '...'
  }

  return { message: msg, percent }
}

export function createProgressLimiter(onEmit: (msg: string, pct: number | null) => void) {
  let lastEmit = 0
  let lastMsg: string | null = null

  return (msg: string, pct: number | null, stateChanged: boolean = false) => {
    const now = Date.now()
    if (stateChanged || now - lastEmit >= 1000) {
      lastEmit = now
      lastMsg = msg
      onEmit(msg, pct)
    }
  }
}
