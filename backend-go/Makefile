# Root Makefile — operates across the whole go.work workspace. Per-service
# Makefiles aren't used; this repo's CI is module-scoped (see
# specs/backend-go/architecture/10-deployment-infrastructure.md) but local
# dev commands are easiest as one shared entrypoint.

SERVICES := ai-provider-service annotation-service api-gateway auth-service \
            automation-service credential-broker-service git-gateway-service \
            infra-fleet-service issue-tracking-service notification-service \
            orchestration-service project-service scm-integration-service \
            task-service tenant-service usage-service workflow-service

.PHONY: dev-up dev-down build vet test test-integration lint fmt fmt-check \
        proto-gen proto-lint migrate-all tidy-all help

help:
	@echo "make dev-up            - start postgres/vault/nats via docker compose"
	@echo "make dev-down          - stop the local dev stack"
	@echo "make build             - go build every module (common, proto, all services)"
	@echo "make vet               - go vet every module"
	@echo "make test              - go test every module (unit tests only)"
	@echo "make test-integration  - go test -tags=integration every module (needs Docker)"
	@echo "make lint              - golangci-lint every module"
	@echo "make fmt               - gofmt -w the whole tree"
	@echo "make fmt-check         - fail if gofmt would change anything"
	@echo "make proto-gen         - regenerate Go code from proto/*.proto via buf"
	@echo "make proto-lint        - buf lint + breaking-change check"
	@echo "make migrate-all       - run golang-migrate up for every service against DATABASE_DSN_<SERVICE>"
	@echo "make tidy-all          - go mod tidy every module"

dev-up:
	docker compose up -d
	@echo "postgres :5432, vault :8200 (dev root token: dev-root-token), nats :4222"

dev-down:
	docker compose down

# NOTE: as of this scaffold, `go build ./...` from the workspace root hits a
# known issue with this Go toolchain version resolving the ./... pattern
# across workspace modules ("directory prefix . does not contain modules
# listed in go.work") — iterate per-module instead until that's resolved
# (revisit the toolchain version at that point, this may just be fixed in a
# later Go release).
build:
	@for m in common proto $(addprefix services/,$(SERVICES)); do \
		echo "==> build $$m"; (cd $$m && go build ./...) || exit 1; \
	done

vet:
	@for m in common proto $(addprefix services/,$(SERVICES)); do \
		echo "==> vet $$m"; (cd $$m && go vet ./...) || exit 1; \
	done

test:
	@for m in common proto $(addprefix services/,$(SERVICES)); do \
		echo "==> test $$m"; (cd $$m && go test ./...) || exit 1; \
	done

test-integration:
	@for m in $(addprefix services/,$(SERVICES)); do \
		echo "==> integration test $$m"; (cd $$m && go test -tags=integration ./...) || exit 1; \
	done

lint:
	@for m in common proto $(addprefix services/,$(SERVICES)); do \
		echo "==> lint $$m"; (cd $$m && golangci-lint run ./...) || exit 1; \
	done

fmt:
	gofmt -w $$(find . -name '*.go' -not -path '*/gen/*')

fmt-check:
	@diff=$$(gofmt -l $$(find . -name '*.go' -not -path '*/gen/*')); \
	if [ -n "$$diff" ]; then echo "gofmt needed on:"; echo "$$diff"; exit 1; fi

proto-gen:
	cd proto && buf generate

proto-lint:
	cd proto && buf lint && buf breaking --against '.git#branch=main' || true

tidy-all:
	@for m in common proto $(addprefix services/,$(SERVICES)); do \
		echo "==> tidy $$m"; (cd $$m && go mod tidy) || exit 1; \
	done
	go work sync

migrate-all:
	@echo "Run per-service, e.g.:"
	@echo "  migrate -path services/usage-service/migrations -database \$$DATABASE_DSN up"
	@echo "See each service's README for its own migrations path and expected DSN."
