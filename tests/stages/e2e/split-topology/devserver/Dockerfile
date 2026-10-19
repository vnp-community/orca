# A remote dev server as Orca sees one: a Linux box with git, Node, the Dev Server Agent and (optionally) tdd-agent.
# agent.js, the fake `claude` and the fixtures are bind-mounted (docker-compose.devserver.yml), so a rebuilt agent
# needs a container restart, not an image rebuild.
FROM node:22.11.0-bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends git python3 python3-venv ca-certificates bash procps curl ripgrep iproute2 \
 && rm -rf /var/lib/apt/lists/*

# Real Claude Code (AI_MODE=real). Off by default: CI and most runs use the fake CLI.
ARG INSTALL_CLAUDE_CLI=0
RUN if [ "$INSTALL_CLAUDE_CLI" = "1" ]; then npm install -g @anthropic-ai/claude-code && npm cache clean --force; fi

# tdd-agent + tsagent from wheels built on the host by split-up.sh (TDD_AGENT_SRC). wheels/ always exists (it may hold
# only .gitkeep), so the image builds without the integration too.
COPY wheels/ /tmp/wheels/
RUN if ls /tmp/wheels/*.whl >/dev/null 2>&1; then \
      python3 -m venv /opt/tdd-agent \
      && /opt/tdd-agent/bin/pip install --no-cache-dir /tmp/wheels/*.whl \
      && ln -s /opt/tdd-agent/bin/tdd-agent /usr/local/bin/tdd-agent \
      && ln -s /opt/tdd-agent/bin/tsagent /usr/local/bin/tsagent; \
    fi \
 && rm -rf /tmp/wheels

COPY entrypoint.sh /usr/local/bin/devserver-entrypoint
RUN chmod 0755 /usr/local/bin/devserver-entrypoint \
 && mkdir -p /srv/projects /opt/orca-agent \
 && chown node:node /srv/projects

# The image's `node` user (uid 1000): the agent never runs as root, like on a real dev server.
USER node
WORKDIR /home/node
ENTRYPOINT ["/usr/local/bin/devserver-entrypoint"]
