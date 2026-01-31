# Proxy Support

This document describes proxy configuration and usage in Quarry.
It is informational; normative behavior is defined in `docs/contracts/CONTRACT_PROXY.md`.

---

## Overview

Quarry provides provider-agnostic proxy support with runtime selection and
executor application. Pools are declared once and referenced by jobs.

---

## Configuration

### Repository-level pools

Example (`quarry.yaml`):

```yaml
proxies:
  iproyal_nyc:
    strategy: round_robin
    endpoints:
      - protocol: https
        host: geo.iproyal.com
        port: 12321
        username: ${IPROYAL_USER}
        password: ${IPROYAL_PASS}
      - protocol: https
        host: geo.iproyal.com
        port: 12322
        username: ${IPROYAL_USER}
        password: ${IPROYAL_PASS}
```

### Job-level selection

```yaml
job:
  proxy:
    pool: iproyal_nyc
    strategy: round_robin
```

---

## Strategies

- `round_robin`: deterministic rotation per pool
- `random`: random endpoint selection
- `sticky`: stable mapping by job/domain/origin with optional TTL

---

## Security Notes

- Use environment variables for credentials.
- Runtime and executor must never log passwords.
- Results may include `proxy_used` metadata without passwords.

---

## Troubleshooting

- 403/407 responses: verify credentials and proxy auth application.
- Timeouts: verify endpoint availability and protocol compatibility.
- Socks5 issues: Puppeteer support is best-effort; prefer http/https if possible.
