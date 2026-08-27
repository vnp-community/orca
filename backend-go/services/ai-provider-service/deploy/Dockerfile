# Multi-stage build, static binary, distroless runtime — per
# specs/backend-go/architecture/04-tech-stack.md's build conventions.
FROM golang:1.25-bookworm AS build
WORKDIR /src

# go.work-aware build: the whole workspace is needed at build time because
# this service depends on the common and proto modules via go.work, not a
# versioned registry dependency (see architecture/03's shared-library policy).
COPY go.work go.work.sum* ./
COPY common ./common
COPY proto ./proto
COPY services ./services

WORKDIR /src/services/ai-provider-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /out/ai-provider-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ai-provider-service /ai-provider-service
COPY services/ai-provider-service/migrations /migrations
USER nonroot:nonroot
ENTRYPOINT ["/ai-provider-service"]
