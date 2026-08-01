# BL-AIP-03 — Provider Health Check & Quota Management

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-AIP-03 |
| **Tên** | Provider Health Check & Quota Management |
| **Domain** | AI Provider Management |
| **Actor** | System (background), Admin |
| **Priority** | P1 |

---

> **Topology:** Dev Server chủ động **mở WebSocket vào Orca Server** (WS client).  
> Health check gửi qua JSON-RPC trên WS đó — không dùng SSH relay.  
> `AgentConnectionManager.getConnection(devServerId)` trả về WS connection đã mở sẵn.

---

## Background Health Check (mỗi 15 phút)

```typescript
async function checkAllProviderAccounts() {
  const accounts = await db.aiProviderAccounts.findAll({ status: ['healthy', 'degraded'] })

  for (const account of accounts) {
    try {
      // Ping provider qua WS connection Dev Server đã mở vào Orca
      const conn = AgentConnectionManager.getConnection(account.devServerId)
      if (!conn) {
        await db.aiProviderAccounts.update(account.id, { status: 'unreachable', lastCheckedAt: now() })
        continue
      }
      const result = await conn.call('ai.ping', {
        accountId: account.id,
        provider: account.provider
      })
      // Dev Server: đọc .enc file → decrypt apiKey → test API call → trả { latencyMs, ok }

      await db.aiProviderAccounts.update(account.id, {
        status: 'healthy',
        latencyMs: result.latencyMs,
        lastCheckedAt: now()
      })
    } catch (err) {
      const status = classifyError(err)
      // 'quota_exceeded' | 'invalid_key' | 'unreachable'
      await db.aiProviderAccounts.update(account.id, { status, lastCheckedAt: now() })

      if (status !== 'unreachable') {
        await sendAlert(account, status)  // Webhook + WebSocket push
      }
    }
  }
}
```

## Quota Tracking

```typescript
// Mỗi khi agent/workflow hoàn thành:
async function recordTokenUsage(accountId: string, tokensUsed: number) {
  const today = toDateString(now())
  await db.run(
    `INSERT INTO orca_provider_usage (account_id, date, tokens_used)
     VALUES (?, ?, ?)
     ON CONFLICT (account_id, date) DO UPDATE
     SET tokens_used = tokens_used + ?`,
    [accountId, today, tokensUsed, tokensUsed]
  )
  
  // Check quota
  const account = await db.aiProviderAccounts.findById(accountId)
  if (account.quotaLimitPerDay) {
    const usage = await getUsageToday(accountId)
    if (usage > account.quotaLimitPerDay * 0.8) {
      await sendAlert(account, 'quota_warning_80pct')
    }
    if (usage >= account.quotaLimitPerDay) {
      await db.aiProviderAccounts.update(accountId, { status: 'quota_exceeded' })
      await sendAlert(account, 'quota_exceeded')
    }
  }
}
```

## DB Schema (migration 0008)

```sql
CREATE TABLE orca_ai_provider_accounts (
  id                  TEXT PRIMARY KEY,
  dev_server_id       TEXT REFERENCES ssh_hosts(id),
  provider            TEXT NOT NULL,  -- anthropic|openai|google|azure|aws|ollama|vllm
  name                TEXT NOT NULL,
  scope               TEXT DEFAULT 'server',  -- server|project|user
  project_id          TEXT REFERENCES orca_projects(id),
  user_id             TEXT REFERENCES orca_users(id),
  is_default          INTEGER DEFAULT 0,
  models              TEXT DEFAULT '[]',  -- JSON array
  quota_limit_per_day INTEGER,
  status              TEXT DEFAULT 'healthy',  -- healthy|degraded|quota_exceeded|invalid_key|unreachable
  latency_ms          INTEGER,
  last_checked_at     INTEGER,
  created_by          TEXT REFERENCES orca_users(id),
  created_at          INTEGER,
  updated_at          INTEGER
);

CREATE TABLE orca_provider_usage (
  account_id  TEXT REFERENCES orca_ai_provider_accounts(id),
  date        TEXT,   -- YYYY-MM-DD
  tokens_used INTEGER DEFAULT 0,
  cost_usd    REAL,   -- optional
  PRIMARY KEY (account_id, date)
);

CREATE INDEX idx_provider_accounts_server ON orca_ai_provider_accounts(dev_server_id);
CREATE INDEX idx_provider_accounts_project ON orca_ai_provider_accounts(project_id);
```
