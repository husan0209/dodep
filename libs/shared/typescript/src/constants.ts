/**
 * Constants for Opus Casino platform
 */

/**
 * API versions
 */
export const API_VERSIONS = {
  V1: 'v1',
} as const;

/**
 * Default pagination settings
 */
export const PAGINATION = {
  DEFAULT_PAGE_SIZE: 20,
  MAX_PAGE_SIZE: 100,
  MIN_PAGE_SIZE: 1,
} as const;

/**
 * Currency constants
 */
export const CURRENCIES = {
  USD: 'USD',
  EUR: 'EUR',
  GBP: 'GBP',
  RUB: 'RUB',
  BRL: 'BRL',
  INR: 'INR',
  JPY: 'JPY',
  CNY: 'CNY',
  KRW: 'KRW',
  CAD: 'CAD',
  AUD: 'AUD',
  CHF: 'CHF',
  BTC: 'BTC',
  ETH: 'ETH',
  USDT: 'USDT',
} as const;

/**
 * Country restrictions (gambling restricted jurisdictions)
 */
export const RESTRICTED_COUNTRIES = [
  'US', // United States (restricted states)
  'FR', // France (requires license)
  'IT', // Italy (requires license)
  'ES', // Spain (requires license)
  'NL', // Netherlands (requires license)
  'BE', // Belgium (requires license)
  'CH', // Switzerland (requires license)
  'AU', // Australia (interactive gambling act)
  'SG', // Singapore
  'HK', // Hong Kong
  'JP', // Japan (restrictions apply)
  'KR', // South Korea
  'CN', // China
  'RU', // Russia (restrictions apply)
] as const;

/**
 * Wallet types
 */
export const WALLET_TYPES = {
  MAIN: 'main',
  BONUS: 'bonus',
  FREE_SPINS: 'free_spins',
  CASHBACK: 'cashback',
} as const;

/**
 * Transaction types
 */
export const TRANSACTION_TYPES = {
  DEPOSIT: 'deposit',
  WITHDRAWAL: 'withdrawal',
  BET_PLACE: 'bet_place',
  BET_WIN: 'bet_win',
  BET_REFUND: 'bet_refund',
  BONUS_CREDIT: 'bonus_credit',
  BONUS_DEBIT: 'bonus_debit',
  TRANSFER: 'transfer',
  ADJUSTMENT: 'adjustment',
} as const;

/**
 * Bet types
 */
export const BET_TYPES = {
  SPORTS: 'sports',
  LIVE: 'live',
  CASINO: 'casino',
  LOTTERY: 'lottery',
  VIRTUAL: 'virtual',
} as const;

/**
 * Bet statuses
 */
export const BET_STATUSES = {
  PENDING: 'pending',
  ACCEPTED: 'accepted',
  SETTLED: 'settled',
  CANCELLED: 'cancelled',
  REJECTED: 'rejected',
} as const;

/**
 * Bonus types
 */
export const BONUS_TYPES = {
  WELCOME: 'welcome',
  DEPOSIT: 'deposit',
  NO_DEPOSIT: 'no_deposit',
  FREE_SPINS: 'free_spins',
  CASHBACK: 'cashback',
  RELOAD: 'reload',
  VIP: 'vip',
  LOYALTY: 'loyalty',
  TOURNAMENT: 'tournament',
} as const;

/**
 * KYC levels
 */
export const KYC_LEVELS = {
  NONE: 'none',
  BASIC: 'basic',
  IDENTITY: 'identity',
  ENHANCED: 'enhanced',
  VIP: 'vip',
} as const;

/**
 * Document types for KYC
 */
export const DOCUMENT_TYPES = {
  PASSPORT: 'passport',
  DRIVERS_LICENSE: 'drivers_license',
  NATIONAL_ID: 'national_id',
  PROOF_OF_ADDRESS: 'proof_of_address',
  BANK_STATEMENT: 'bank_statement',
  SELFIE: 'selfie',
} as const;

/**
 * Notification channels
 */
export const NOTIFICATION_CHANNELS = {
  EMAIL: 'email',
  SMS: 'sms',
  PUSH: 'push',
  IN_APP: 'in_app',
  TELEGRAM: 'telegram',
  WHATSAPP: 'whatsapp',
} as const;

/**
 * Notification types
 */
export const NOTIFICATION_TYPES = {
  WELCOME: 'welcome',
  DEPOSIT_CONFIRMED: 'deposit_confirmed',
  WITHDRAWAL_PROCESSED: 'withdrawal_processed',
  BET_SETTLED: 'bet_settled',
  BONUS_ACTIVATED: 'bonus_activated',
  BONUS_EXPIRING: 'bonus_expiring',
  KYC_STATUS: 'kyc_status',
  SECURITY_ALERT: 'security_alert',
  PROMOTION: 'promotion',
  TOURNAMENT: 'tournament',
  VIP_UPDATE: 'vip_update',
  REALITY_CHECK: 'reality_check',
  SYSTEM: 'system',
} as const;

/**
 * Device types
 */
export const DEVICE_TYPES = {
  WEB: 'web',
  MOBILE_WEB: 'mobile_web',
  IOS: 'ios',
  ANDROID: 'android',
  DESKTOP: 'desktop',
} as const;

/**
 * Rate limits
 */
export const RATE_LIMITS = {
  // Authentication
  LOGIN_ATTEMPTS: 5,
  LOGIN_WINDOW_MS: 15 * 60 * 1000, // 15 minutes
  
  // API
  API_REQUESTS_PER_MINUTE: 100,
  API_REQUESTS_PER_HOUR: 1000,
  
  // Betting
  BETS_PER_SECOND: 10,
  
  // Withdrawal
  WITHDRAWAL_REQUESTS_PER_DAY: 5,
  
  // Password reset
  PASSWORD_RESET_PER_HOUR: 3,
  
  // 2FA
  TOTP_WINDOW_SECONDS: 30,
  TOTP_MAX_ATTEMPTS: 5,
} as const;

/**
 * Betting limits
 */
export const BET_LIMITS = {
  MIN_STAKE: '0.10',
  MAX_STAKE: '10000.00',
  MAX_WIN_MULTIPLIER: 10000, // Max win = stake * multiplier
  MAX_ODDS: '1000.00',
  MIN_ODDS: '1.01',
} as const;

/**
 * Payment limits
 */
export const PAYMENT_LIMITS = {
  MIN_DEPOSIT: '1.00',
  MAX_DEPOSIT_DAILY: '10000.00',
  MIN_WITHDRAWAL: '10.00',
  MAX_WITHDRAWAL_DAILY: '50000.00',
  MAX_WITHDRAWAL_MONTHLY: '500000.00',
} as const;

/**
 * Session settings
 */
export const SESSION = {
  ACCESS_TOKEN_TTL_SECONDS: 900, // 15 minutes
  REFRESH_TOKEN_TTL_SECONDS: 604800, // 7 days
  SESSION_TTL_SECONDS: 2592000, // 30 days
  MAX_SESSIONS_PER_USER: 5,
} as const;

/**
 * Responsible gambling limits
 */
export const RESPONSIBLE_GAMBLING = {
  REALITY_CHECK_DEFAULT_MINUTES: 60,
  SESSION_TIME_LIMIT_DEFAULT_MINUTES: 120,
  SELF_EXCLUSION_MIN_DAYS: 6,
  SELF_EXCLUSION_MAX_YEARS: 5,
  COOLDOWN_PERIOD_DAYS: 24, // After limit decrease
} as const;

/**
 * Error codes
 */
export const ERROR_CODES = {
  // Authentication (1000-1999)
  AUTH_INVALID_CREDENTIALS: 'AUTH_1001',
  AUTH_TOKEN_EXPIRED: 'AUTH_1002',
  AUTH_TOKEN_INVALID: 'AUTH_1003',
  AUTH_2FA_REQUIRED: 'AUTH_1006',
  AUTH_2FA_INVALID: 'AUTH_1007',
  AUTH_ACCOUNT_LOCKED: 'AUTH_1008',
  
  // Wallet (5000-5999)
  WALLET_NOT_FOUND: 'WALLET_5001',
  INSUFFICIENT_BALANCE: 'WALLET_5002',
  INSUFFICIENT_AVAILABLE_BALANCE: 'WALLET_5003',
  
  // Bet (7000-7999)
  BET_NOT_FOUND: 'BET_7001',
  BET_INVALID: 'BET_7002',
  BET_ALREADY_SETTLED: 'BET_7003',
  BET_LIMIT_EXCEEDED: 'BET_7005',
  BET_ODDS_CHANGED: 'BET_7007',
  
  // System (11000-11999)
  INTERNAL_ERROR: 'SYS_11001',
  SERVICE_UNAVAILABLE: 'SYS_11002',
  RATE_LIMIT_EXCEEDED: 'SYS_11005',
} as const;
