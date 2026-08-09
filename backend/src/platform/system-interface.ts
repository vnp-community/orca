/** ISystemInfo — platform/OS information queries */
export interface ISystemInfo {
  /** Host OS platform */
  getPlatform(): NodeJS.Platform

  /** Total system memory in bytes */
  getTotalMemory(): number

  /** Free system memory in bytes */
  getFreeMemory(): number

  /** Number of CPU cores */
  getCpuCount(): number

  /** OS hostname */
  getHostname(): string
}
