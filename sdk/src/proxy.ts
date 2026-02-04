/**
 * Proxy validation helpers per CONTRACT_PROXY.md
 */

import type { ProxyEndpoint, ProxyPool, ProxyProtocol, ProxyStrategy } from './types/proxy'

// ============================================
// Validation Result Types
// ============================================

/**
 * Hard validation error that must cause rejection.
 */
export type ProxyValidationError = {
  readonly field: string
  readonly message: string
}

/**
 * Soft warning that should be surfaced but not rejected.
 */
export type ProxyValidationWarning = {
  readonly field: string
  readonly message: string
}

/**
 * Result of proxy validation.
 */
export type ProxyValidationResult = {
  readonly valid: boolean
  readonly errors: readonly ProxyValidationError[]
  readonly warnings: readonly ProxyValidationWarning[]
}

// ============================================
// Constants
// ============================================

const VALID_PROTOCOLS: readonly ProxyProtocol[] = ['http', 'https', 'socks5']
const VALID_STRATEGIES: readonly ProxyStrategy[] = ['round_robin', 'random', 'sticky']
const MIN_PORT = 1
const MAX_PORT = 65535
const LARGE_POOL_THRESHOLD = 100

// ============================================
// Validation Functions
// ============================================

/**
 * Validate a proxy endpoint per CONTRACT_PROXY hard validation rules.
 *
 * Hard validation (must reject):
 * - port is within 1-65535
 * - protocol is one of http|https|socks5
 * - username and password must be provided together if either is set
 */
export function validateProxyEndpoint(endpoint: ProxyEndpoint, prefix = ''): ProxyValidationResult {
  const errors: ProxyValidationError[] = []
  const warnings: ProxyValidationWarning[] = []
  const fieldPrefix = prefix ? `${prefix}.` : ''

  // Protocol validation
  if (!VALID_PROTOCOLS.includes(endpoint.protocol)) {
    errors.push({
      field: `${fieldPrefix}protocol`,
      message: `Invalid protocol "${endpoint.protocol}". Must be one of: ${VALID_PROTOCOLS.join(', ')}`
    })
  }

  // Port validation
  if (
    typeof endpoint.port !== 'number' ||
    !Number.isInteger(endpoint.port) ||
    endpoint.port < MIN_PORT ||
    endpoint.port > MAX_PORT
  ) {
    errors.push({
      field: `${fieldPrefix}port`,
      message: `Invalid port ${endpoint.port}. Must be integer between ${MIN_PORT} and ${MAX_PORT}`
    })
  }

  // Auth pair validation
  const hasUsername = endpoint.username !== undefined && endpoint.username !== ''
  const hasPassword = endpoint.password !== undefined && endpoint.password !== ''
  if (hasUsername !== hasPassword) {
    errors.push({
      field: `${fieldPrefix}username/${fieldPrefix}password`,
      message: 'Username and password must be provided together'
    })
  }

  // Soft warning: socks5 with Puppeteer
  if (endpoint.protocol === 'socks5') {
    warnings.push({
      field: `${fieldPrefix}protocol`,
      message: 'socks5 usage with Puppeteer is best-effort'
    })
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings
  }
}

/**
 * Validate a proxy pool per CONTRACT_PROXY hard validation rules.
 *
 * Hard validation (must reject):
 * - endpoints length > 0
 * - all endpoints pass validation
 * - strategy is valid
 *
 * Soft warnings:
 * - very large endpoint lists with round_robin
 */
export function validateProxyPool(pool: ProxyPool): ProxyValidationResult {
  const errors: ProxyValidationError[] = []
  const warnings: ProxyValidationWarning[] = []

  // Name validation
  if (!pool.name || typeof pool.name !== 'string') {
    errors.push({
      field: 'name',
      message: 'Pool name is required'
    })
  }

  // Strategy validation
  if (!VALID_STRATEGIES.includes(pool.strategy)) {
    errors.push({
      field: 'strategy',
      message: `Invalid strategy "${pool.strategy}". Must be one of: ${VALID_STRATEGIES.join(', ')}`
    })
  }

  // Endpoints validation
  if (!pool.endpoints || pool.endpoints.length === 0) {
    errors.push({
      field: 'endpoints',
      message: 'Pool must have at least one endpoint'
    })
  } else {
    // Validate each endpoint
    for (let i = 0; i < pool.endpoints.length; i++) {
      const endpointResult = validateProxyEndpoint(pool.endpoints[i], `endpoints[${i}]`)
      errors.push(...endpointResult.errors)
      warnings.push(...endpointResult.warnings)
    }

    // Soft warning: large pool with round_robin
    if (pool.strategy === 'round_robin' && pool.endpoints.length > LARGE_POOL_THRESHOLD) {
      warnings.push({
        field: 'strategy',
        message: `Large endpoint list (${pool.endpoints.length}) with round_robin strategy; consider using random`
      })
    }
  }

  // Sticky validation
  if (pool.sticky) {
    if (pool.strategy !== 'sticky') {
      warnings.push({
        field: 'sticky',
        message: 'Sticky configuration provided but strategy is not "sticky"'
      })
    }

    const validScopes = ['job', 'domain', 'origin']
    if (!validScopes.includes(pool.sticky.scope)) {
      errors.push({
        field: 'sticky.scope',
        message: `Invalid sticky scope "${pool.sticky.scope}". Must be one of: ${validScopes.join(', ')}`
      })
    }

    if (
      pool.sticky.ttlMs !== undefined &&
      (typeof pool.sticky.ttlMs !== 'number' || pool.sticky.ttlMs <= 0)
    ) {
      errors.push({
        field: 'sticky.ttlMs',
        message: 'Sticky TTL must be a positive number'
      })
    }
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings
  }
}

/**
 * Redact password from a proxy endpoint.
 * Returns a new object without the password field.
 */
export function redactProxyEndpoint<T extends ProxyEndpoint>(endpoint: T): Omit<T, 'password'> {
  const { password: _, ...redacted } = endpoint
  return redacted
}
