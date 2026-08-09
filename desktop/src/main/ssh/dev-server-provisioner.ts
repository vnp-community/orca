/**
 * Dev Server Provisioner — Create per-user Linux accounts on dev server via SSH
 *
 * Idempotently ensures a unix account exists and Orca Server's public key
 * is authorized. Requires a privileged SSH connection (ubuntu/sudo NOPASSWD).
 *
 * @module main/ssh/dev-server-provisioner
 */

import { readFileSync } from 'node:fs'
import { toLinuxUsername } from './ssh-user-resolver'
import type { SshConnection } from './ssh-connection'

export class DevServerProvisioner {
  private readonly orcaPublicKey: string

  constructor(orcaPublicKeyPath: string) {
    // Read public key at construct time — fail fast if file is missing
    this.orcaPublicKey = readFileSync(orcaPublicKeyPath, 'utf-8').trim()
  }

  /**
   * Idempotent: ensure unix account exists and authorize Orca Server SSH public key.
   * Requires conn to have an account with sudo NOPASSWD (typically 'ubuntu').
   * Returns the provisioned Linux username.
   */
  async ensureUserAccount(
    conn:      SshConnection,
    userId:    string,
    userEmail: string
  ): Promise<string> {
    const linuxUser = toLinuxUsername(userEmail, userId)

    const exists = await this.checkUserExists(conn, linuxUser)
    if (!exists) {
      await this.createUser(conn, linuxUser)
    }
    await this.authorizeKey(conn, linuxUser, this.orcaPublicKey)

    return linuxUser
  }

  /** Return true if linux user already exists on the remote server */
  async checkUserExists(conn: SshConnection, linuxUser: string): Promise<boolean> {
    const result = await conn.exec(`id ${linuxUser} 2>&1`)
    return result.exitCode === 0
  }

  private async createUser(conn: SshConnection, linuxUser: string): Promise<void> {
    const result = await conn.exec(
      `sudo useradd -m -s /bin/bash ${linuxUser} 2>&1 && id ${linuxUser}`
    )
    if (result.exitCode !== 0) {
      throw new Error(
        `Failed to create unix account '${linuxUser}'. stderr: ${result.stderr || result.stdout}`
      )
    }
    // Optionally add to 'developers' group (ignore error if group doesn't exist)
    await conn.exec(
      `getent group developers &>/dev/null && sudo usermod -aG developers ${linuxUser} || true`
    )
  }

  private async authorizeKey(
    conn:      SshConnection,
    linuxUser: string,
    publicKey: string
  ): Promise<void> {
    const sshDir      = `/home/${linuxUser}/.ssh`
    const authKeyPath = `${sshDir}/authorized_keys`

    // Idempotent: mkdir, set perms, append key only if not already present
    const script = [
      `sudo mkdir -p ${sshDir}`,
      `sudo chmod 700 ${sshDir}`,
      `sudo chown ${linuxUser}:${linuxUser} ${sshDir}`,
      `sudo grep -qF '${publicKey}' ${authKeyPath} 2>/dev/null || echo '${publicKey}' | sudo tee -a ${authKeyPath} > /dev/null`,
      `sudo chmod 600 ${authKeyPath}`,
      `sudo chown ${linuxUser}:${linuxUser} ${authKeyPath}`
    ].join(' && ')

    const result = await conn.exec(script)
    if (result.exitCode !== 0) {
      throw new Error(
        `Failed to authorize SSH key for ${linuxUser}: ${result.stderr || result.stdout}`
      )
    }
  }
}
