// Package testutil provides the testcontainers-go helpers every service's
// adapter/postgres integration tests use, per
// specs/backend-go/standards/testing-strategy.md ("adapter/postgres/: real
// Postgres via testcontainers-go, migrations run against it before tests").
package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartPostgres launches a disposable Postgres container for the duration
// of a test and returns its connection DSN. The container is terminated
// automatically via t.Cleanup — callers don't need their own defer.
func StartPostgres(t *testing.T, dbName string) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "orca",
			"POSTGRES_PASSWORD": "orca",
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("testutil: starting postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("testutil: getting container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("testutil: getting mapped port: %v", err)
	}

	return fmt.Sprintf("postgres://orca:orca@%s:%s/%s?sslmode=disable", host, port.Port(), dbName)
}
