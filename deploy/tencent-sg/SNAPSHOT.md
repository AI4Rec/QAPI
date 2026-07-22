# Tencent SG server snapshot — 2026-07-23

This directory records the non-secret source and deployment configuration for the QAPI version running on the `tencent-sg` host as observed on 2026-07-23.

## Application runtime

- Executable: `/opt/new-api/new-api`
- Application version: `v1.0.0-rc.20-curated.17`
- Binary timestamp: `2026-07-21 22:44:57 +08:00`
- Binary size: `143892772` bytes
- Binary SHA-256: `4fcfbc8f0e7dddabbdb522b3ecbdb7ec3c51457d62dc50dfafa584558e5e71c2`
- Service start timestamp: `2026-07-22 05:24:38 CST`
- Embedded VCS metadata: unavailable in the running binary

The remote source tree used to build the binary did not retain `.git` metadata. This repository snapshot is the traceable source backup for the deployed curated build.

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
- SHA-256: `da20441a8d317e2c4cdfb2d0018e368dcf0d3d2a88ce1a122c65ab953b180b31`
- Service: `qapi-monitor.service`

The monitor reads the management secret at runtime from the external CLIProxyAPI configuration. No secret value is stored in this repository snapshot.

## Configuration checksums

- `nginx/qapi.click.conf`: `5ed3665870e211ce2da601f6cbc63243ff6a66b2443c486d14902c766307c0fa`
- `qapi-monitor/qapi_capacity_collector.py`: `da20441a8d317e2c4cdfb2d0018e368dcf0d3d2a88ce1a122c65ab953b180b31`
- `scripts/qapi-storage-retention`: `86ad9a2dd192b04d494a258ab990bcda79351c66ccd15a54aace432a177f3460`
- `systemd/cliproxyapi-versioned.conf`: `351aac3708199bc3370d0db757574e951091a5110c400b4cf68c11cbe4e6c947`
- `systemd/logrotate-storage.conf`: `511aeef376a1b115002857008cf40361f5eabbb7af33674fd40453f2f65e808a`
- `systemd/new-api.service`: `8fd768b93828c2f7d598b64534b056b7483f5738d95e5a0de4859349db7ef6fd`
- `systemd/new-api-memory-guard.conf`: `15a04b387b62d80046d394947863fce9418c96ea834aed1ff17207e172caa7f0`
- `systemd/new-api-redis.conf`: `56ee3e3417a7c9e870bab9ee3a793814608f5764d6195b1e3b7063ad19146df9`
- `systemd/qapi-monitor.service`: `07465fb7543ea924aac91349e2ec2ac3272e883f6b6fd1fe32d06afc1c017d06`
- `systemd/qapi-storage-retention.service`: `3a603094a10e26e79d3cb02f0bbbcc54e8fd849d8aa21a5002dc6f6a5731ceeb`
- `systemd/qapi-storage-retention.timer`: `4569d9c2b9619521a18433b23e8c7a3306f583bbfa9541e3989ef463da270abf`
- `systemd/redis-server-qapi-memory.conf`: `93ed9d9b34018926181fb2d2b42fc0db45495edc542e6c3a7df20ec40b85c104`
- `systemd/sub2api.service`: `2957d30538fadb2e231bdb389c3c3eeae1fe3db60d8784ae53fd93e08d91fcd3`

## Intentionally excluded

Secrets, `/etc/new-api.env`, TLS private keys, databases, logs, browser sessions, generated monitor state, account credentials, and other runtime data are not source code and are not included.
