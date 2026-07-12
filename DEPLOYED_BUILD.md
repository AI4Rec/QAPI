# Deployed customization baseline

This repository tracks the customized source tree used for the running QAPI
deployment.

## Baseline

- Application version: `v1.0.0-rc.20-curated.16`
- Upstream project: `QuantumNous/new-api`
- Upstream base: `v1.0.0-rc.20`
- Source captured from: `/tmp/codex-runs/qapi-activation` on `tencent-sg`
- Running binary path: `/opt/new-api/new-api`
- Running binary SHA-256:
  `2835d606ad184891aa77a3303cc72bf1f56d4a829869faa4cb05a7dfede6a8e2`

The remote source tree has no `.git` metadata. This repository is the reliable
history point for the curated deployment.

## Current deployment timeline

- Default frontend build completed: 2026-07-12 20:35 +08:00
- Curated.16 application build completed: 2026-07-12 20:36 +08:00
- Curated.16 deployment completed: 2026-07-12 20:37 +08:00
- Service health check: HTTP 200 on `/api/status`

## Curated capabilities

- CPA Codex account import, monitoring, archive and deletion
- Separate CPA-import and official-login account pools with quota countdowns
- Server-side Codex OAuth through CLIProxyAPI
- Full Chrome controlled through an isolated Xvfb display
- Operational-cost asset tracking and archive management
- Activation queue scheduling and administration
- Responses API compatibility and billing normalization extensions

The remote-browser runtime is documented in
`docs/deployment/remote-codex-oauth-browser.md`. Browser binaries and extracted
system libraries remain deployment artifacts and are not committed.

## Excluded runtime state

The repository intentionally excludes SQLite databases, service secrets,
logs, dependency directories, generated frontend bundles, build caches, and
the deployed binary. Runtime data and secrets require a separate encrypted
backup process.
