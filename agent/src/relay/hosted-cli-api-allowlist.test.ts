import { describe, expect, it } from 'vitest'
import { assertAllowedGhArgs, assertAllowedGlabArgs } from './hosted-cli-api-allowlist'

describe('assertAllowedGhArgs', () => {
  it('rejects an unknown subcommand', () => {
    expect(() => assertAllowedGhArgs(['admin', 'foo'])).toThrow('subcommand "admin" is not allowed')
  })

  it('rejects an empty argv', () => {
    expect(() => assertAllowedGhArgs([])).toThrow('subcommand "" is not allowed')
  })

  it('allows non-api subcommands through without endpoint validation', () => {
    expect(() => assertAllowedGhArgs(['pr', 'create', '--title', 'x'])).not.toThrow()
    expect(() => assertAllowedGhArgs(['issue', 'list'])).not.toThrow()
    expect(() => assertAllowedGhArgs(['auth', 'status'])).not.toThrow()
  })

  it('rejects any argument with a shell metacharacter', () => {
    expect(() => assertAllowedGhArgs(['pr', 'create', '--title', 'x; rm -rf /'])).toThrow(
      'disallowed character'
    )
  })

  it('rejects a NUL byte in any argument', () => {
    expect(() => assertAllowedGhArgs(['pr', 'view', '\0'])).toThrow('disallowed character')
  })

  describe('api subcommand — endpoint allowlist', () => {
    it('allows a repo-scoped path', () => {
      expect(() => assertAllowedGhArgs(['api', 'repos/acme/widgets/issues/42'])).not.toThrow()
    })

    it('allows a repo-scoped path with query params, ignoring the query string', () => {
      expect(() =>
        assertAllowedGhArgs(['api', 'repos/acme/widgets/pulls?head=main&state=all&per_page=1'])
      ).not.toThrow()
    })

    it('allows the literal rate_limit path', () => {
      expect(() => assertAllowedGhArgs(['api', 'rate_limit'])).not.toThrow()
    })

    it('allows the literal graphql path with -f query=...', () => {
      expect(() => assertAllowedGhArgs(['api', 'graphql', '-f', 'query=query { viewer { id } }'])).not.toThrow()
    })

    it('allows the literal user path', () => {
      expect(() => assertAllowedGhArgs(['api', 'user', '--jq', '.login'])).not.toThrow()
    })

    it('allows user/starred/{repo} (the star-toggle caller)', () => {
      expect(() => assertAllowedGhArgs(['api', '-X', 'PUT', 'user/starred/acme/widgets'])).not.toThrow()
    })

    it('allows -X DELETE on a repo-scoped path', () => {
      expect(() =>
        assertAllowedGhArgs(['api', '-X', 'DELETE', 'repos/acme/widgets/issues/comments/9'])
      ).not.toThrow()
    })

    it('allows --cache/--paginate/--include as boolean/value flags around the path', () => {
      expect(() =>
        assertAllowedGhArgs(['api', '--cache', '60s', 'repos/acme/widgets/pulls/1'])
      ).not.toThrow()
      expect(() =>
        assertAllowedGhArgs(['api', '--paginate', 'repos/acme/widgets/labels', '--jq', '.[].name'])
      ).not.toThrow()
      expect(() => assertAllowedGhArgs(['api', '--include', 'user/starred/acme/widgets'])).not.toThrow()
    })

    it('rejects an org-admin-level path', () => {
      expect(() => assertAllowedGhArgs(['api', 'orgs/acme/members'])).toThrow('not in the allowlist')
    })

    it('rejects an arbitrary/unscoped path', () => {
      expect(() => assertAllowedGhArgs(['api', 'admin/users'])).toThrow('not in the allowlist')
      expect(() => assertAllowedGhArgs(['api', 'installation/repositories'])).toThrow('not in the allowlist')
    })

    it('rejects a repo path for a DIFFERENT repo shape trying to sneak past the regex (path traversal)', () => {
      expect(() => assertAllowedGhArgs(['api', 'repos/'])).toThrow('not in the allowlist')
      expect(() => assertAllowedGhArgs(['api', 'repositories/acme/widgets'])).toThrow('not in the allowlist')
    })

    it('rejects an unsupported HTTP method', () => {
      expect(() =>
        assertAllowedGhArgs(['api', '-X', 'TRACE', 'repos/acme/widgets/issues/1'])
      ).toThrow('HTTP method "TRACE" is not allowed')
    })

    it('rejects a call with no endpoint path at all', () => {
      expect(() => assertAllowedGhArgs(['api', '--paginate'])).toThrow('has no endpoint path')
    })
  })
})

describe('assertAllowedGlabArgs', () => {
  it('rejects an unknown subcommand', () => {
    expect(() => assertAllowedGlabArgs(['admin', 'foo'])).toThrow('subcommand "admin" is not allowed')
  })

  it('allows non-api subcommands through without endpoint validation', () => {
    expect(() => assertAllowedGlabArgs(['mr', 'create', '--title', 'x'])).not.toThrow()
    expect(() => assertAllowedGlabArgs(['auth', 'status'])).not.toThrow()
  })

  it('rejects any argument with a shell metacharacter', () => {
    expect(() => assertAllowedGlabArgs(['mr', 'create', '--title', 'x`whoami`'])).toThrow(
      'disallowed character'
    )
  })

  describe('api subcommand — endpoint allowlist', () => {
    it('allows a project-scoped path', () => {
      expect(() => assertAllowedGlabArgs(['api', 'projects/123/merge_requests'])).not.toThrow()
    })

    it('allows the literal user path', () => {
      expect(() => assertAllowedGlabArgs(['api', 'user'])).not.toThrow()
    })

    it('allows --hostname as a value flag before the path (self-hosted routing)', () => {
      expect(() =>
        assertAllowedGlabArgs(['api', '--hostname', 'gitlab.example.com', 'projects/123/issues'])
      ).not.toThrow()
    })

    it('allows -X on a project-scoped path', () => {
      expect(() =>
        assertAllowedGlabArgs(['api', '-X', 'PATCH', 'projects/123/merge_requests/4'])
      ).not.toThrow()
    })

    it('rejects a non-project/non-user path', () => {
      expect(() => assertAllowedGlabArgs(['api', 'admin/users'])).toThrow('not in the allowlist')
      expect(() => assertAllowedGlabArgs(['api', 'groups/acme/members'])).toThrow('not in the allowlist')
    })

    it('rejects an unsupported HTTP method', () => {
      expect(() =>
        assertAllowedGlabArgs(['api', '-X', 'CONNECT', 'projects/123/issues'])
      ).toThrow('HTTP method "CONNECT" is not allowed')
    })
  })
})
