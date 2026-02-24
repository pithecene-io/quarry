/**
 * Unit tests for proxy validation and redaction helpers.
 *
 * Validates the hard validation rules from CONTRACT_PROXY.md:
 * - Host must be a non-empty string
 * - Protocol must be http|https|socks5
 * - Port must be integer between 1 and 65535
 * - Username and password must be provided together
 * - Pool must have at least one endpoint
 */
import { describe, expect, it } from 'vitest'
import { redactProxyEndpoint, validateProxyEndpoint, validateProxyPool } from '../src/proxy'
import type { ProxyEndpoint, ProxyPool } from '../src/types/proxy'

function validEndpoint(overrides: Partial<ProxyEndpoint> = {}): ProxyEndpoint {
  return {
    protocol: 'http',
    host: 'proxy.example.com',
    port: 8080,
    ...overrides
  }
}

function validPool(overrides: Partial<ProxyPool> = {}): ProxyPool {
  return {
    name: 'test-pool',
    strategy: 'round_robin',
    endpoints: [validEndpoint()],
    ...overrides
  }
}

describe('validateProxyEndpoint', () => {
  it('valid endpoint (http, host, port) returns valid', () => {
    const result = validateProxyEndpoint(validEndpoint())

    expect(result.valid).toBe(true)
    expect(result.errors).toHaveLength(0)
  })

  it('valid endpoint with auth (username + password) returns valid', () => {
    const result = validateProxyEndpoint(validEndpoint({ username: 'user', password: 'pass' }))

    expect(result.valid).toBe(true)
    expect(result.errors).toHaveLength(0)
  })

  it('missing protocol returns invalid', () => {
    const result = validateProxyEndpoint(
      validEndpoint({ protocol: '' as ProxyEndpoint['protocol'] })
    )

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('protocol')
  })

  it('invalid protocol (ftp) returns invalid', () => {
    const result = validateProxyEndpoint(
      validEndpoint({ protocol: 'ftp' as ProxyEndpoint['protocol'] })
    )

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('protocol')
    expect(result.errors[0].message).toContain('ftp')
  })

  it('empty host returns invalid', () => {
    const result = validateProxyEndpoint(validEndpoint({ host: '' }))

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('host')
  })

  it('port 0 returns invalid', () => {
    const result = validateProxyEndpoint(validEndpoint({ port: 0 }))

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('port')
  })

  it('port 65536 returns invalid', () => {
    const result = validateProxyEndpoint(validEndpoint({ port: 65536 }))

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('port')
  })

  it('non-integer port (80.5) returns invalid', () => {
    const result = validateProxyEndpoint(validEndpoint({ port: 80.5 }))

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toBe('port')
  })

  it('username without password returns invalid', () => {
    const result = validateProxyEndpoint(validEndpoint({ username: 'user' }))

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toContain('username')
    expect(result.errors[0].field).toContain('password')
  })

  it('password without username returns invalid', () => {
    const result = validateProxyEndpoint(validEndpoint({ password: 'pass' }))

    expect(result.valid).toBe(false)
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0].field).toContain('username')
    expect(result.errors[0].field).toContain('password')
  })
})

describe('validateProxyPool', () => {
  it('valid pool with endpoints returns valid', () => {
    const result = validateProxyPool(validPool())

    expect(result.valid).toBe(true)
    expect(result.errors).toHaveLength(0)
  })

  it('empty endpoints returns invalid', () => {
    const result = validateProxyPool(validPool({ endpoints: [] }))

    expect(result.valid).toBe(false)
    expect(result.errors.some((e) => e.field === 'endpoints')).toBe(true)
  })

  it('pool with one invalid endpoint surfaces that endpoint error', () => {
    const badEndpoint = validEndpoint({ port: 0 })
    const result = validateProxyPool(validPool({ endpoints: [badEndpoint] }))

    expect(result.valid).toBe(false)
    expect(result.errors.some((e) => e.field === 'endpoints[0].port')).toBe(true)
  })
})

describe('redactProxyEndpoint', () => {
  it('endpoint without auth returns unchanged', () => {
    const endpoint = validEndpoint()
    const redacted = redactProxyEndpoint(endpoint)

    expect(redacted).toEqual({
      protocol: 'http',
      host: 'proxy.example.com',
      port: 8080
    })
  })

  it('endpoint with auth has password field removed', () => {
    const endpoint = validEndpoint({ username: 'user', password: 's3cret' })
    const redacted = redactProxyEndpoint(endpoint)

    expect(redacted).toEqual({
      protocol: 'http',
      host: 'proxy.example.com',
      port: 8080,
      username: 'user'
    })
    expect('password' in redacted).toBe(false)
  })
})
