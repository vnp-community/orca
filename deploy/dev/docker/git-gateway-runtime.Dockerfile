# git-gateway-service's own runtime image — every OTHER backend-go service in
# this mount-based dev deploy uses the shared, unmodified
# gcr.io/distroless/static-debian12:nonroot image (see docker-compose.yml's
# x-go-image comment: "smallest possible image, no Dockerfile built"). This
# service is the one deliberate exception: its host-local dispatch path
# (internal/adapter/localgit) shells out to a real `git` binary — per
# specs/backend-go/tdd/services/git-gateway-service.md §2 ("no connectionId
# → execute locally... retained for local/dev deployments"), which is
# EXACTLY this dev deploy's own topology, not a hypothetical.
#
# Mirrors backend-go/services/git-gateway-service/deploy/Dockerfile's own
# runtime stages exactly (git installed from Debian into a distroless base)
# — this file only skips that Dockerfile's Go BUILD stage, since
# build-local.sh already compiles the binary separately and bind-mounts it
# in, the same way every other service's container gets its binary.
#
# No ENTRYPOINT/CMD here: docker-compose.yml's x-go-defaults anchor already
# sets `entrypoint: ["/app/bin/orca"]` for every service, this one included.
FROM debian:bookworm-slim AS git
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*
# /data/repos is where docker-compose.yml mounts the git-gateway-repos
# volume — created+chowned here (65532 = distroless's fixed "nonroot" UID/
# GID, confirmed via `docker inspect .Config.User`) since a Docker named
# volume otherwise mounts root-owned by default, and this final image's
# USER nonroot can't mkdir/chown itself (no shell, no coreutils).
RUN mkdir -p /data/repos && chown 65532:65532 /data/repos

# Live-verified gap in the ALSO-checked-in production Dockerfile
# (backend-go/services/git-gateway-service/deploy/Dockerfile) this mirrors:
# copying only /usr/bin/git + /usr/lib/git-core + /usr/share/git-core (no
# shared libs) fails at runtime — `git: error while loading shared
# libraries: libpcre2-8.so.0: cannot open shared object file` — distroless/
# base-debian12 ships libc/ld-linux/libz but not git's other dynamic deps
# (libpcre2, and git-core sub-binaries like git-remote-https pull in
# libcurl/libssl/libcrypto for HTTPS transport on top of that). Copying the
# whole shared-library directory rather than hand-picking .so files avoids
# re-discovering this one .so at a time for every git subcommand that
# happens to need one.
FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=git /usr/bin/git /usr/bin/git
COPY --from=git /usr/lib/git-core /usr/lib/git-core
COPY --from=git /usr/share/git-core /usr/share/git-core
COPY --from=git /usr/lib/x86_64-linux-gnu /usr/lib/x86_64-linux-gnu
COPY --from=git /lib/x86_64-linux-gnu /lib/x86_64-linux-gnu
COPY --from=git /etc/ssl/certs /etc/ssl/certs
COPY --from=git --chown=65532:65532 /data/repos /data/repos
