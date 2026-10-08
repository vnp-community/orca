import { expect, test } from '@playwright/test'
import { bootApp } from './support/code-intel-app-navigation'
import {
  DEV_STACK_SKIP_REASON,
  devCredentials,
  devStackUrl
} from './support/code-intel-dev-backend'

test.skip(!devStackUrl, DEV_STACK_SKIP_REASON)

// @dev-stack: runs only against a real stack; asserts contract-level facts, no concrete numbers.
test('@dev-stack local login opens a session and the web SPA boots on it', async ({ page }) => {
  const login = await page.request.post('/auth/local', { data: devCredentials.admin })
  expect(login.ok()).toBe(true)
  const me = await page.request.get('/auth/me')
  expect(me.ok()).toBe(true)
  // Why: page.request shares the page's cookie jar, so the SPA boots straight into the session.
  await bootApp(page)
  await expect(page.getByText('Orca web hit a renderer error.')).toHaveCount(0)
})
