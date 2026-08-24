#!/bin/sh
# Creates one Postgres database per data-owning backend-go service, per
# specs/backend-go/architecture/05-data-architecture.md's database-per-service
# rule. Run automatically by the postgres container's
# docker-entrypoint-initdb.d hook on first startup (docker-compose.yml).
# Only the 15 services that own a database are listed here — see
# specs/backend-go/services/00-service-catalog.md for which 2 don't.
# issuetracking joined this list in Epic G (docs/execution-plan.md):
# issue-tracking-service's database hosts only its transactional-outbox
# table, never a queryable copy of issue data (Jira/Linear remain the
# systems of record) — see that service's internal/adapter/postgres
# package doc comment. scm joined in Phase 3 (docs/execution-plan.md §3):
# scm-integration-service's rate_limit_cache/webhook_delivery_log tables —
# see that service's cmd/server/main.go doc comment.
set -e

DATABASES="auth tenant project infra aiprovider workflow task orchestration automation annotation notification usage credential issuetracking scm"

for db in $DATABASES; do
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
		SELECT 'CREATE DATABASE $db OWNER orca' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
	EOSQL
done
