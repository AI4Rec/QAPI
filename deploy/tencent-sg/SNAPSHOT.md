# Tencent SG server snapshot — 2026-07-18

This directory records the non-secret source and deployment configuration for the QAPI version running on the `tencent-sg` host after the 2026-07-18 12:09 CST restart.

## Application runtime

- Executable: `/opt/new-api/new-api`
- Binary timestamp: `2026-07-18 11:32:14 +08:00`
- Binary size: `138870968` bytes
- Binary SHA-256: `db4ac37ff42eee7bef69e5ce808a398a6cdf904528ee53c47d1f5e7ee04eb1d6`
- Embedded VCS revision: `b853921de88c4127521bdab59810dfcaf74cb6a6`
- Embedded VCS state: `modified=true`
- Matching source snapshot before this manifest: `55312db76df28f3c5ae2be99bdf07b5e3e0d0296`

The running binary was built from the `b853921` checkout with local modifications. Those application changes were subsequently committed as `55312db`. Tests and deployment helpers in that commit are not part of the binary.

## Active icon

- Public URL: `https://qapi.click/qapi-logo.png?v=20260718`
- Server path: `/opt/new-api/static/qapi-logo.png`
- Source copies:
  - `web/default/public/qapi-logo.png`
  - `web/classic/public/qapi-logo.png`
- SHA-256: `7bb6753aea061d45618dfb6b6cfc0d27f9dc8516b471a658a9e5f4be9b8f1488`

Nginx serves this file directly. The application status setting selects the public URL, so the legacy `logo.png` remains unchanged.

## Capacity monitor

- Runtime source: `/usr/local/lib/qapi-monitor/qapi_capacity_collector.py`
- SHA-256: `a7a68641ace31bed387f2517ba8063665ce49899d57d20f3a016c11324a0857e`
- Service: `qapi-monitor.service`

The monitor reads the management secret at runtime from the external CLIProxyAPI configuration. No secret value is stored in this repository snapshot.

## Configuration checksums

- `nginx/qapi.click.conf`: `7b135fedd70a680eb6396f31c7adad6ec5f5c0b883cd384ab8162bd98f7cc1e6`
- `systemd/new-api.service`: `8fd768b93828c2f7d598b64534b056b7483f5738d95e5a0de4859349db7ef6fd`
- `systemd/new-api-memory-guard.conf`: `15a04b387b62d80046d394947863fce9418c96ea834aed1ff17207e172caa7f0`
- `systemd/qapi-monitor.service`: `07465fb7543ea924aac91349e2ec2ac3272e883f6b6fd1fe32d06afc1c017d06`

## Intentionally excluded

Secrets, `/etc/new-api.env`, TLS private keys, databases, logs, browser sessions, generated monitor state, account credentials, and other runtime data are not source code and are not included.
