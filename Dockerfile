# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1: build — compile Go binary with embedded executor bundle
# =============================================================================
FROM node:24.13.0-bookworm-slim AS build

# Pin Go version to match go.mod
ARG GO_VERSION=1.25.6
# TARGETARCH is injected by BuildKit for multi-platform builds (e.g. amd64, arm64).
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git \
  && rm -rf /var/lib/apt/lists/*

# Install Go from official tarball
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
  | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Enable corepack for pnpm
RUN corepack enable

WORKDIR /src

# Layer 1: dependency manifests (cacheable)
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY sdk/package.json sdk/
COPY executor-node/package.json executor-node/

# Build stage only compiles TS/Go — skip Puppeteer's browser download.
ENV PUPPETEER_SKIP_DOWNLOAD=true

RUN pnpm install --frozen-lockfile

# Layer 2: source code
COPY sdk/ sdk/
COPY executor-node/ executor-node/
COPY quarry/ quarry/

# Build TS packages → bundle executor → compile Go binary
RUN pnpm -C sdk run build \
  && pnpm -C executor-node run build \
  && pnpm -C executor-node run bundle

WORKDIR /src/quarry
RUN CGO_ENABLED=0 go build -o /usr/local/bin/quarry ./cmd/quarry

# =============================================================================
# Stage 2: deps — install runtime Node.js dependencies
# =============================================================================
FROM node:24.13.0-bookworm-slim AS deps

WORKDIR /opt/quarry

# Install puppeteer ecosystem without downloading bundled Chromium.
# The full image uses system Chromium; the slim image has no browser.
ENV PUPPETEER_SKIP_DOWNLOAD=true

# Pin exact versions matching pnpm-lock.yaml for reproducible builds.
RUN npm install --omit=dev \
  puppeteer@24.37.2 \
  puppeteer-extra@3.3.6 \
  puppeteer-extra-plugin-stealth@2.11.2 \
  puppeteer-extra-plugin-adblocker@2.13.6

# =============================================================================
# Stage 3: slim — runtime image without Chromium
# =============================================================================
FROM node:24.13.0-bookworm-slim AS slim

RUN groupadd -r quarry && useradd -r -g quarry -m quarry

COPY --from=build /usr/local/bin/quarry /usr/local/bin/quarry
COPY --from=deps /opt/quarry/node_modules /opt/quarry/node_modules

# Let Node resolve puppeteer and friends from the shared location.
# User scripts that ship their own node_modules resolve those first.
ENV NODE_PATH=/opt/quarry/node_modules

# Chromium cannot use its sandbox inside containers without extra capabilities.
ENV QUARRY_NO_SANDBOX=1

USER quarry
WORKDIR /work

ENTRYPOINT ["quarry"]

# =============================================================================
# Stage 4: full — runtime image with system Chromium + fonts
# =============================================================================
FROM slim AS full

USER root

RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    fonts-liberation \
    fonts-noto-color-emoji \
    fonts-noto-cjk \
  && rm -rf /var/lib/apt/lists/*

# Point Puppeteer at system Chromium instead of expecting a downloaded copy.
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium

USER quarry
