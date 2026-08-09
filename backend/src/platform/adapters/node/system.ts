import { cpus, totalmem, freemem, hostname } from 'node:os'
import type { ISystemInfo } from '../../system-interface'

/**
 * NodeSystemInfo — ISystemInfo using Node.js os module.
 */
export class NodeSystemInfo implements ISystemInfo {
  getPlatform(): NodeJS.Platform {
    return process.platform
  }

  getTotalMemory(): number {
    return totalmem()
  }

  getFreeMemory(): number {
    return freemem()
  }

  getCpuCount(): number {
    return cpus().length
  }

  getHostname(): string {
    return hostname()
  }
}
