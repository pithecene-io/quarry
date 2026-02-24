/**
 * Proxy configuration types per CONTRACT_PROXY.md
 */

/**
 * Allowed proxy protocols.
 * Note: socks5 is best-effort with Puppeteer.
 */
export type ProxyProtocol = 'http' | 'https' | 'socks5'

/**
 * Proxy selection strategies for pools.
 */
export type ProxyStrategy = 'round_robin' | 'random' | 'sticky'

/**
 * A resolved proxy endpoint the executor can dial.
 * Emitted by runtime in run requests.
 */
export type ProxyEndpoint = {
  /** Proxy protocol */
  readonly protocol: ProxyProtocol
  /** Proxy host */
  readonly host: string
  /** Proxy port (1-65535) */
  readonly port: number
  /** Optional username for authentication */
  readonly username?: string
  /** Optional password for authentication */
  readonly password?: string
}

/**
 * Redacted proxy endpoint for run results.
 * Password is always omitted.
 */
export type ProxyEndpointRedacted = Omit<ProxyEndpoint, 'password'>

/**
 * Sticky scope determines what key is used for sticky assignment.
 */
export type ProxyStickyScope = 'job' | 'domain' | 'origin'

/**
 * Sticky configuration for a proxy pool.
 */
export type ProxySticky = {
  /** Scope for sticky key derivation */
  readonly scope: ProxyStickyScope
  /** Optional TTL in milliseconds for sticky entries */
  readonly ttlMs?: number
}

/**
 * Proxy pool definition.
 * Parsed and validated by runtime.
 */
export type ProxyPool = {
  /** Pool name (unique identifier) */
  readonly name: string
  /** Selection strategy */
  readonly strategy: ProxyStrategy
  /** Available endpoints (must have at least one) */
  readonly endpoints: readonly ProxyEndpoint[]
  /** Optional sticky configuration (only valid when strategy is 'sticky') */
  readonly sticky?: ProxySticky
  /** Optional recency window size for random selection.
   * Maintains a ring buffer of recently-used endpoint indices
   * and excludes them from random selection. */
  readonly recencyWindow?: number
}

/**
 * Job-level proxy selection request.
 * Submitted with job/run to select a pool and optional overrides.
 */
export type JobProxyRequest = {
  /** Pool name to select from */
  readonly pool: string
  /** Optional strategy override */
  readonly strategy?: ProxyStrategy
  /** Optional sticky key override (used instead of derived key) */
  readonly stickyKey?: string
}
