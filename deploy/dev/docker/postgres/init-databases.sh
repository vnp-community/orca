#!/bin/sh
# Creates one Postgres database per data-owning backend-go service, per
# specs/backend-go/architecture/05-data-architecture.md's database-per-service
# rule. Run automatically by the postgres container's
# docker-entrypoint-initdb.d hook on first startup (docker-compose.yml).
# Only the 13 services that own a database are listed here — see
# specs/backend-go/services/00-service-catalog.md for which 4 don't.
set -e

DATABASES="auth tenant project infra aiprovider workflow task orchestration automation annotation notification usage credential"

for db in $DATABASES; do
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
		SELECT 'CREATE DATABASE $db OWNER orca' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
	EOSQL
done
