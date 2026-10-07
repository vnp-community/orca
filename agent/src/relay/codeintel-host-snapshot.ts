import * as os from 'os'

export type HostSnapshot = {
  platform: NodeJS.Platform
  cores: number
  loadavg1: number
  freeMemBytes: number
}

// In container environments, freeMemBytes is the host's free memory, not cgroup limits.
export function readHostSnapshot(deps = { os }): HostSnapshot {
  return {
    platform: deps.os.platform(),
    cores: deps.os.availableParallelism(),
    loadavg1: Math.round(deps.os.loadavg()[0] * 100) / 100,
    freeMemBytes: deps.os.freemem()
  }
}
