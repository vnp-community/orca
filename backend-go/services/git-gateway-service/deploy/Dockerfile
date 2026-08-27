# Multi-stage build, static binary, distroless runtime — per
# specs/backend-go/architecture/04-tech-stack.md's build conventions.
#
# No migrations to copy into the runtime stage — per git-gateway-service.md
# §5, this service owns no database.
FROM golang:1.23-bookworm AS build
WORKDIR /src

# go.work-aware build: the whole workspace is needed at build time because
# this service depends on the common and proto modules via go.work, not a
# versioned registry dependency (see architecture/03's shared-library policy).
COPY go.work go.work.sum* ./
COPY common ./common
COPY proto ./proto
COPY services/git-gateway-service ./services/git-gateway-service

WORKDIR /src/services/git-gateway-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /out/git-gateway-service ./cmd/server

# The runtime image needs a real `git` binary — this service's host-local
# dispatch path (internal/adapter/localgit) shells out to it directly
# (git-gateway-service.md §2 step 3). distroless/static has no package
# manager, so this stage instead installs git from Debian and copies just
# the binary plus dynamic deps into the static base below.
FROM debian:bookworm-slim AS git
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=git /usr/bin/git /usr/bin/git
COPY --from=git /usr/lib/git-core /usr/lib/git-core
COPY --from=git /usr/share/git-core /usr/share/git-core
COPY --from=build /out/git-gateway-service /git-gateway-service
USER nonroot:nonroot
ENTRYPOINT ["/git-gateway-service"]
