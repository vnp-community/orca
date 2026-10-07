# Multi-stage build, static binary, distroless runtime
FROM golang:1.25-bookworm AS build
WORKDIR /src

COPY go.work go.work.sum* ./
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
USER nonroot:nonroot
ENTRYPOINT ["/request-service"]
