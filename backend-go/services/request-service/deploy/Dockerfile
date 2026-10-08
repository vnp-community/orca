# Multi-stage build, static binary, distroless runtime
# go.work requires Go >= 1.26.0, so the 1.25 image cannot build any module in the workspace.
FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.work go.work.sum* ./
COPY cmd ./cmd
COPY common ./common
COPY proto ./proto
COPY policy ./policy
COPY services ./services

WORKDIR /src/services/request-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /out/request-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/request-service /request-service
COPY --from=build /src/policy/orca-authz /policy/orca-authz
COPY services/request-service/migrations /migrations
ENV OPA_BUNDLE_PATH=/policy/orca-authz
USER nonroot:nonroot
ENTRYPOINT ["/request-service"]
