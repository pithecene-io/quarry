# Proxy Support

This document describes proxy configuration and usage in Quarry.
It is informational; normative behavior is defined in `docs/contracts/CONTRACT_PROXY.md`.

---

## Overview

Quarry provides provider-agnostic proxy support with:
- **Runtime selection**: pools, strategies (round-robin, random, sticky)
- **Executor application**: Puppeteer launch args and page authentication
- **Validation**: hard and soft validation rules

Pools are declared once and referenced by jobs.

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

  residential_sticky:
    strategy: sticky
    sticky:
      scope: domain    # job | domain | origin
      ttl_ms: 3600000  # 1 hour
    endpoints:
      - protocol: http
        host: proxy.example.com
        port: 8080
```

### Job-level selection

```yaml
job:
  proxy:
    pool: iproyal_nyc
    # Optional: override pool strategy for this job
    strategy: round_robin
    # Optional: explicit sticky key (overrides scope-derived key)
    sticky_key: my-custom-key
```

---

## Strategies

### Round-robin

- Deterministic rotation per pool
- Counter maintained by runtime
- Suitable for load distribution

### Random

- Uniform random selection per request
- Secure RNG used
- Suitable for large pools

### Sticky

- Stable mapping by sticky key
- Key derivation precedence:
  1. Explicit `sticky_key` if provided
  2. If scope = `job`: job ID
  3. If scope = `domain`: request domain
  4. If scope = `origin`: scheme+host+port
- Optional TTL for entry expiration

---

## Validation

### Hard validation (rejects invalid config)

- Endpoints list must have at least one entry
- Port must be between 1 and 65535
- Protocol must be `http`, `https`, or `socks5`
- Username and password must be provided together

### Soft warnings (surfaced but not rejected)

- `socks5` usage with Puppeteer is best-effort
- Very large endpoint lists with `round_robin` (consider `random`)

---

## Runtime Behavior

### Selection

1. Runtime receives job with proxy request
2. Selector looks up pool by name
3. Strategy selects endpoint (updates counters/maps as needed)
4. Resolved endpoint is included in run request to executor

### Application (Executor)

1. Executor receives resolved endpoint in run input
2. Puppeteer launched with `--proxy-server=<protocol>://<host>:<port>`
3. If credentials present, `page.authenticate()` is called
4. Proxy password is never logged

### Result

- Run result includes `proxy_used` with redacted endpoint (no password)

---

## SDK Types

```typescript
import type {
  ProxyEndpoint,
  ProxyPool,
  ProxyStrategy,
  ProxySticky
} from '@justapithecus/quarry-sdk'

import {
  validateProxyEndpoint,
  validateProxyPool,
  redactProxyEndpoint
} from '@justapithecus/quarry-sdk'
```

---

## Security Notes

- Use environment variables for credentials
- Runtime and executor must never log passwords
- Results include `proxy_used` metadata without passwords
- Credentials are applied via page authentication, not URL

---

## Troubleshooting

| Symptom | Possible Cause | Solution |
|---------|---------------|----------|
| 403/407 responses | Invalid credentials | Verify username/password |
| Connection timeouts | Endpoint unavailable | Check proxy server status |
| Protocol errors | Wrong protocol | Verify http vs https vs socks5 |
| Socks5 failures | Limited Puppeteer support | Prefer http/https if possible |
| Sticky not working | Missing scope config | Add `sticky.scope` to pool |
