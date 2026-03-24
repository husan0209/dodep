/**
 * Shared TypeScript types for Opus Casino platform
 */

/**
 * User identifier (UUID v4)
 */
export type UserId = string;

/**
 * Bet identifier (UUID v4)
 */
export type BetId = string;

/**
 * Transaction identifier (UUID v4)
 */
export type TransactionId = string;

/**
 * Game identifier (UUID v4)
 */
export type GameId = string;

/**
 * Session identifier (UUID v4)
 */
export type SessionId = string;

/**
 * Monetary amount with currency
 * Amount is stored as string to avoid floating point precision issues
 * Examples: "100.00", "0.50", "1234.56"
 */
export interface Money {
  amount: string;
  currency: string; // ISO 4217 currency code
}

/**
 * Pagination parameters
 */
export interface PaginationParams {
  page_size?: number;
  cursor?: string;
  sort_by?: string;
  descending?: boolean;
}

/**
 * Pagination response
 */
export interface PaginationResult<T> {
  items: T[];
  next_cursor?: string;
  prev_cursor?: string;
  has_more: boolean;
  total_count?: number;
}

/**
 * Date range filter
 */
export interface DateRange {
  from?: Date;
  to?: Date;
}

/**
 * Error details
 */
export interface ErrorDetails {
  error_code: string;
  error_message: string;
  metadata?: Record<string, string>;
  field_errors?: FieldError[];
  trace_id?: string;
}

/**
 * Field validation error
 */
export interface FieldError {
  field: string;
  error_code: string;
  error_message: string;
}

/**
 * API response wrapper
 */
export interface ApiResponse<T> {
  data?: T;
  error?: ErrorDetails;
}

/**
 * Health check status
 */
export type HealthStatus = 'healthy' | 'degraded' | 'unhealthy';

/**
 * Health check response
 */
export interface HealthCheckResponse {
  service_name: string;
  status: HealthStatus;
  timestamp: Date;
  version: string;
  components?: Record<string, ComponentHealth>;
}

/**
 * Component health
 */
export interface ComponentHealth {
  name: string;
  status: HealthStatus;
  message?: string;
  latency_ms?: number;
}

/**
 * Device type
 */
export type DeviceType = 'web' | 'mobile_web' | 'ios' | 'android' | 'desktop';

/**
 * Country code (ISO 3166-1 alpha-2)
 */
export type CountryCode = string;

/**
 * Currency code (ISO 4217)
 */
export type CurrencyCode = string;
