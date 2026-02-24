/**
 * Proxy validation helpers per CONTRACT_PROXY.md
 */

import type { ProxyEndpoint, ProxyPool, ProxyProtocol, ProxyStrategy } from './types/proxy'

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

const VALID_PROTOCOLS: readonly ProxyProtocol[] = ['http', 'https', 'socks5']
const VALID_STRATEGIES: readonly ProxyStrategy[] = ['round_robin', 'random', 'sticky']
const MIN_PORT = 1
const MAX_PORT = 65535
const LARGE_POOL_THRESHOLD = 100

function validationError(field: string, message: string): ProxyValidationError {
  return { field, message }
}

function validationWarning(field: string, message: string): ProxyValidationWarning {
  return { field, message }
}

function isPresent(value: string | undefined): boolean {
  return value !== undefined && value !== ''
}

function buildValidationResult(
  errors: ProxyValidationError[],
  warnings: ProxyValidationWarning[]
): ProxyValidationResult {
  return { valid: errors.length === 0, errors, warnings }
}

/**
 * Validate a proxy endpoint per CONTRACT_PROXY hard validation rules.
 *
 * Hard validation (must reject):
 * - host must be a non-empty string
 * - port is within 1-65535
 * - protocol is one of http|https|socks5
 * - username and password must be provided together if either is set
 */
export function validateProxyEndpoint(endpoint: ProxyEndpoint, prefix = ''): ProxyValidationResult {
  const errors: ProxyValidationError[] = []
  const warnings: ProxyValidationWarning[] = []
  const fieldPrefix = prefix ? `${prefix}.` : ''

  // Host validation (required per CONTRACT_PROXY.md)
  if (!endpoint.host || typeof endpoint.host !== 'string') {
    errors.push(validationError(`${fieldPrefix}host`, 'Host is required and must be a non-empty string'))
  }

  // Protocol validation
  if (!VALID_PROTOCOLS.includes(endpoint.protocol)) {
    errors.push(
      validationError(
        `${fieldPrefix}protocol`,
        `Invalid protocol "${endpoint.protocol}". Must be one of: ${VALID_PROTOCOLS.join(', ')}`
      )
    )
  }

  // Port validation
  if (
    typeof endpoint.port !== 'number' ||
    !Number.isInteger(endpoint.port) ||
    endpoint.port < MIN_PORT ||
    endpoint.port > MAX_PORT
  ) {
    errors.push(
      validationError(
        `${fieldPrefix}port`,
        `Invalid port ${endpoint.port}. Must be integer between ${MIN_PORT} and ${MAX_PORT}`
      )
    )
  }

  // Auth pair validation
  if (isPresent(endpoint.username) !== isPresent(endpoint.password)) {
    errors.push(
      validationError(
        `${fieldPrefix}username/${fieldPrefix}password`,
        'Username and password must be provided together'
      )
    )
  }

  // Soft warning: socks5 with Puppeteer
  if (endpoint.protocol === 'socks5') {
    warnings.push(
      validationWarning(`${fieldPrefix}protocol`, 'socks5 usage with Puppeteer is best-effort')
    )
  }

  return buildValidationResult(errors, warnings)
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
    errors.push(validationError('name', 'Pool name is required'))
  }

  // Strategy validation
  if (!VALID_STRATEGIES.includes(pool.strategy)) {
    errors.push(
      validationError(
        'strategy',
        `Invalid strategy "${pool.strategy}". Must be one of: ${VALID_STRATEGIES.join(', ')}`
      )
    )
  }

  // Endpoints validation
  if (!pool.endpoints || pool.endpoints.length === 0) {
    errors.push(validationError('endpoints', 'Pool must have at least one endpoint'))
  } else {
    // Validate each endpoint
    const endpointResults = pool.endpoints.map((endpoint, i) =>
      validateProxyEndpoint(endpoint, `endpoints[${i}]`)
    )
    errors.push(...endpointResults.flatMap((r) => r.errors))
    warnings.push(...endpointResults.flatMap((r) => r.warnings))

    // Soft warning: large pool with round_robin
    if (pool.strategy === 'round_robin' && pool.endpoints.length > LARGE_POOL_THRESHOLD) {
      warnings.push(
        validationWarning(
          'strategy',
          `Large endpoint list (${pool.endpoints.length}) with round_robin strategy; consider using random`
        )
      )
    }
  }

  // Recency window validation
  if (pool.recencyWindow !== undefined) {
    if (
      typeof pool.recencyWindow !== 'number' ||
      !Number.isInteger(pool.recencyWindow) ||
      pool.recencyWindow <= 0
    ) {
      errors.push(validationError('recencyWindow', 'Recency window must be a positive integer'))
    }
    if (pool.strategy !== 'random') {
      warnings.push(
        validationWarning(
          'recencyWindow',
          `Recency window is set but strategy is "${pool.strategy}"; recency window only applies to random selection`
        )
      )
    }
  }

  // Sticky validation
  if (pool.sticky) {
    if (pool.strategy !== 'sticky') {
      warnings.push(
        validationWarning('sticky', 'Sticky configuration provided but strategy is not "sticky"')
      )
    }

    const validScopes = ['job', 'domain', 'origin']
    if (!validScopes.includes(pool.sticky.scope)) {
      errors.push(
        validationError(
          'sticky.scope',
          `Invalid sticky scope "${pool.sticky.scope}". Must be one of: ${validScopes.join(', ')}`
        )
      )
    }

    if (
      pool.sticky.ttlMs !== undefined &&
      (typeof pool.sticky.ttlMs !== 'number' || pool.sticky.ttlMs <= 0)
    ) {
      errors.push(validationError('sticky.ttlMs', 'Sticky TTL must be a positive number'))
    }
  }

  return buildValidationResult(errors, warnings)
}

/**
 * Redact password from a proxy endpoint.
 * Returns a new object without the password field.
 */
export function redactProxyEndpoint<T extends ProxyEndpoint>(endpoint: T): Omit<T, 'password'> {
  const { password: _, ...redacted } = endpoint
  return redacted
}
