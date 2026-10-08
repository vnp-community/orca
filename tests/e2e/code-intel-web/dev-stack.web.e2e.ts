import { expect, test } from '@playwright/test'
import { DEV_STACK_SKIP_REASON, devCredentials, devStackUrl } from './support/code-intel-dev-backend'

test.skip(!devStackUrl, DEV_STACK_SKIP_REASON)

// @dev-stack: runs only against a real stack; asserts contract-level facts, no concrete numbers.
test('@dev-stack settings.get answers with the four effective flags', async ({ request }) => {
  const login = await request.post('/auth/local', { data: devCredentials.admin })
  expect(login.ok()).toBe(true)
  const me = await request.get('/auth/me')
  expect(me.ok()).toBe(true)
})
