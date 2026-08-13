/**
 * TeamService — CRUD service for teams and team membership
 *
 * Manages orca_teams (migration 0016) and orca_team_members (migration 0010,
 * `priority` column added by migration 0016). Team has no departmentId/
 * parentId by design — a Team cuts across the department tree rather than
 * belonging to one department (docs/guides/user-profile-team-department-rbac.md §5.2).
 *
 * `priority` on TeamMember is the cascade-merge tiebreaker used by
 * ProfileResolver when a user belongs to multiple teams — higher wins.
 *
 * Pattern follows ProjectService.ts: pool.withConnection((db) => db.query(...)).
 *
 * @module main/team/TeamService
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { Team, TeamMember } from '../../shared/team-types'

/** Raw DB row for orca_teams */
type TeamRow = {
  id: string
  name: string
  createdAt: number
  updatedAt: number
}

/** Raw DB row for orca_team_members */
type TeamMemberRow = {
  teamId: string
  userId: string
  role: string
  priority: number
  addedAt: number
}

function rowToTeam(r: TeamRow): Team {
  return {
    id: r.id,
    name: r.name,
    createdAt: new Date(r.createdAt),
    updatedAt: new Date(r.updatedAt),
  }
}

function rowToTeamMember(r: TeamMemberRow): TeamMember {
  return {
    teamId: r.teamId,
    userId: r.userId,
    role: r.role,
    priority: r.priority,
    addedAt: new Date(r.addedAt),
  }
}

export class TeamService {
  constructor(private readonly pool: IConnectionPool) {}

  /** Create a new team. */
  async createTeam({ name }: { name: string }): Promise<Team> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_teams (id, name, created_at, updated_at)
         VALUES (?, ?, ?, ?)`,
        [id, name, now, now]
      )
    )
    return { id, name, createdAt: new Date(now), updatedAt: new Date(now) }
  }

  /** List all teams. */
  async listTeams(): Promise<Team[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TeamRow>(
        `SELECT id, name, created_at as createdAt, updated_at as updatedAt
         FROM orca_teams
         ORDER BY name`
      )
    )
    return rows.map(rowToTeam)
  }

  /** Get a team by ID. Returns null if not found. */
  async getTeam(teamId: string): Promise<Team | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TeamRow>(
        `SELECT id, name, created_at as createdAt, updated_at as updatedAt
         FROM orca_teams WHERE id = ?`,
        [teamId]
      )
    )
    if (!rows[0]) {return null}
    return rowToTeam(rows[0])
  }

  /**
   * Add (or update) a team member.
   * PK is (team_id, user_id) — INSERT … ON CONFLICT DO UPDATE, safe for
   * repeated calls (same pattern as ProjectService.addMember).
   */
  async addMember({
    teamId,
    userId,
    role,
    priority
  }: {
    teamId: string
    userId: string
    role: string
    priority: number
  }): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_team_members (team_id, user_id, role, priority, added_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(team_id, user_id) DO UPDATE SET
           role     = excluded.role,
           priority = excluded.priority,
           added_at = excluded.added_at`,
        [teamId, userId, role, priority, now]
      )
    )
  }

  /** Remove a member from a team. */
  async removeMember({ teamId, userId }: { teamId: string; userId: string }): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        'DELETE FROM orca_team_members WHERE team_id = ? AND user_id = ?',
        [teamId, userId]
      )
    )
  }

  /** List all members of a team. */
  async listMembers(teamId: string): Promise<TeamMember[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TeamMemberRow>(
        `SELECT team_id as teamId, user_id as userId, role, priority, added_at as addedAt
         FROM orca_team_members WHERE team_id = ?`,
        [teamId]
      )
    )
    return rows.map(rowToTeamMember)
  }

  /**
   * List all team memberships for a user — used by ProfileResolver to
   * resolve which teams' profile layers apply, tiebroken by `priority`.
   */
  async listTeamsForUser(userId: string): Promise<TeamMember[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<TeamMemberRow>(
        `SELECT team_id as teamId, user_id as userId, role, priority, added_at as addedAt
         FROM orca_team_members WHERE user_id = ?`,
        [userId]
      )
    )
    return rows.map(rowToTeamMember)
  }
}
