# Deployed customization baseline

This repository was initialized from the customized source tree used for the
running QAPI deployment.

## Baseline

- Application version: `v1.0.0-rc.20-curated.6`
- Upstream project: `QuantumNous/new-api`
- Upstream base: `v1.0.0-rc.20`
- Source captured from: `/tmp/new-api-custom-src` on `tencent-sg`
- Running binary path: `/opt/new-api/new-api`
- Running binary SHA-256:
  `1de6e691e87c251ea82968016120377ac4c72b9a71890c939d34c325dd47b36a`

The source tree had no original `.git` metadata when it was recovered. The
commit that adds this file is therefore the first reliable history point for
future customizations.

## Final build timeline

- Last detected source edits: 2026-07-12 02:35:33 +08:00
- Default frontend build output completed: 2026-07-12 02:36:33 +08:00
- Deployed binary written: 2026-07-12 02:37:17 +08:00
- Service process started: 2026-07-12 02:37:16 +08:00

## Customized files detected after the source import

The following files had modification times later than the initial source
import and capture the known customization sequence:

- `controller/model.go`
- `model/channel.go`
- `controller/cpa_import.go`
- `router/api-router.go`
- `setting/chat.go`
- `web/default/src/features/channels/api.ts`
- `web/default/src/features/channels/index.tsx`
- `web/default/src/features/channels/components/channels-primary-buttons.tsx`
- `web/default/src/features/channels/components/cpa-accounts-panel.tsx`
- `web/default/src/features/channels/components/dialogs/cpa-import-dialog.tsx`
- `web/default/src/features/keys/index.tsx`
- `web/default/src/features/keys/components/data-table-row-actions.tsx`
- `web/default/src/features/keys/components/dialogs/cc-switch-dialog.tsx`
- `web/default/src/features/chat/lib/chat-links.ts`
- `web/default/src/features/playground/constants.ts`
- `web/default/src/features/playground/index.tsx`
- `web/default/src/features/playground/components/input/playground-input-tools.tsx`
- `web/default/src/hooks/use-sidebar-data.ts`

## Excluded runtime state

The repository intentionally excludes SQLite databases, service secrets,
logs, dependency directories, generated frontend bundles, build caches, and
the deployed binary. Runtime data and secrets require a separate encrypted
backup process.
