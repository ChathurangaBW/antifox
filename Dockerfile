# syntax=docker/dockerfile:1
#
# Runtime image (do not compile in the image): install common tools and copy in the **precompiled Linux single binary**.
# CI's binaries job cross-compiles the binary (pure Go; no QEMU) into
# dist/<TARGETARCH>/antifox in the build context. During multi-architecture builds, arm64 only emulates the apt layer.
# therefore only emulates the apt layer instead of the Next.js/Go builds.
#
# Before building the image locally, prepare the binary:
#   cd web && npm run build:static && cd ..
#   cp -r web/out server/webui/dist
#   CGO_ENABLED=0 GOARCH=amd64 go build -tags embedui -o dist/amd64/antifox ./cmd/antifox
#   docker build -t antifox:local .
FROM python:3.12-slim-bookworm
ARG TARGETARCH
# Common tools: ripgrep / curl / vim plus standard reconnaissance utilities.
# Install Node 20.x from NodeSource because Bookworm ships Node 18 and Playwright requires >=20.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates ripgrep curl wget vim git jq unzip \
      dnsutils iputils-ping netcat-openbsd whois nmap \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*
# Preinstall Playwright MCP/CLI and Chromium so browser MCP works out of the box.
# The previous layer removes apt lists, while `--with-deps` needs apt to install
# Chromium dependencies, so refresh the lists first or Playwright exits with code 100.
RUN apt-get update \
    && npm install -g @playwright/mcp@latest @playwright/cli@latest playwright@latest \
    && playwright-cli --help \
    && playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*
# Report generation uses python-docx (Go invokes report_builder.py as a subprocess).
RUN pip install --no-cache-dir python-docx
WORKDIR /app
# Precompiled binary for the target architecture (dist/amd64/antifox or dist/arm64/antifox).
COPY dist/${TARGETARCH}/antifox /app/antifox
RUN chmod +x /app/antifox
COPY skills/ /app/skills/
# Persistence point for data/ (SQLite + jwt.key).
VOLUME ["/app/data"]
EXPOSE 8787 8788
ENTRYPOINT ["/app/antifox"]
CMD ["-addr", ":8787", "-proxy", ":8788"]
