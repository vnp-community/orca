# syntax=docker/dockerfile:1
# Context = the tdd-agentic-ai repo root: docker build -f deploy/tsagent/Dockerfile --target <target> . Targets:
#   dev      Python + libraries, no code; code mounted from src/tsagent at /app/tsagent (a code change only needs a restart)
#   builder  PyInstaller builds a one-file binary on bookworm (matching the runtime glibc)
#   export   only the binary: docker build --target export --output type=local,dest=bin
#   runtime  debian bookworm-slim, no Python; binary mounted from ../bin
#   prod     image with the code baked in (default)
ARG PYTHON_IMAGE=python:3.12-slim-bookworm

FROM ${PYTHON_IMAGE} AS deps
WORKDIR /app
COPY pyproject.toml ./
RUN python -c "import tomllib; print('\n'.join(tomllib.load(open('pyproject.toml','rb'))['project']['dependencies']))" > /tmp/req.txt \
 && pip install --no-cache-dir -r /tmp/req.txt

FROM deps AS dev
ENV PYTHONPATH=/app PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
RUN useradd --create-home --uid 10001 tsagent && mkdir -p /data && chown tsagent /data
USER tsagent
EXPOSE 8766
HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD python -c "import urllib.request as u; u.urlopen('http://127.0.0.1:8766/health', timeout=3)"
ENTRYPOINT ["python", "-m", "tsagent"]
CMD ["serve", "--host", "0.0.0.0", "--port", "8766"]

FROM deps AS builder
RUN apt-get update && apt-get install -y --no-install-recommends binutils && rm -rf /var/lib/apt/lists/* \
 && pip install --no-cache-dir pyinstaller==6.10.0
COPY src/tsagent ./tsagent
COPY deploy/tsagent/entry.py ./entry.py
# data files (ontology, generated SQL, JSON Schemas, prompts) added explicitly: the bundle runs from source, not a pip install
RUN pyinstaller --onefile --name tsagent --collect-submodules tsagent \
      --add-data tsagent/ontology/tso.yaml:tsagent/ontology --add-data tsagent/ontology/profiles.yaml:tsagent/ontology --add-data tsagent/store/schema.sql:tsagent/store \
      --add-data tsagent/gateway/views.sql:tsagent/gateway --add-data tsagent/schemas:tsagent/schemas \
      --add-data tsagent/validate/shape_rules.json:tsagent/validate --add-data tsagent/analyze/prompts:tsagent/analyze/prompts \
      --add-data tsagent/design/prompts:tsagent/design/prompts --add-data tsagent/specify/prompts:tsagent/specify/prompts \
      --hidden-import jsonschema --hidden-import markdown_it --hidden-import yaml --hidden-import httpx entry.py \
 && ./dist/tsagent --version

FROM scratch AS export
COPY --from=builder /app/dist/tsagent /tsagent

FROM debian:bookworm-slim AS runtime
RUN useradd --create-home --uid 10001 tsagent && mkdir -p /data /opt/tsagent/bin && chown tsagent /data
USER tsagent
EXPOSE 8766
ENTRYPOINT ["/opt/tsagent/bin/tsagent"]
CMD ["serve", "--host", "0.0.0.0", "--port", "8766"]

FROM deps AS prod
ENV PYTHONPATH=/app PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
COPY src/tsagent ./tsagent
RUN useradd --create-home --uid 10001 tsagent && mkdir -p /data && chown tsagent /data
USER tsagent
EXPOSE 8766
HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD python -c "import urllib.request as u; u.urlopen('http://127.0.0.1:8766/health', timeout=3)"
ENTRYPOINT ["python", "-m", "tsagent"]
CMD ["serve", "--host", "0.0.0.0", "--port", "8766"]
