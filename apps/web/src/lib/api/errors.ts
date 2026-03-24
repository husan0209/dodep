import { ApiClientError } from "./client";

const errorMessages: Record<string, string> = {
  // Auth errors
  AUTH_INVALID_CREDENTIALS: "Неверный email или пароль",
  AUTH_TOKEN_EXPIRED: "Сессия истекла",
  AUTH_ACCOUNT_LOCKED: "Аккаунт заблокирован",
  AUTH_ACCOUNT_SUSPENDED: "Аккаунт приостановлен",

  // Wallet errors
  WALLET_INSUFFICIENT_BALANCE: "Недостаточно средств",
  WALLET_CURRENCY_MISMATCH: "Неверная валюта",
  WALLET_LIMIT_EXCEEDED: "Превышен лимит",

  // Bet errors
  BET_EVENT_SUSPENDED: "Событие приостановлено",
  BET_MARKET_CLOSED: "Рынок закрыт",
  BET_ODDS_CHANGED: "Коэффициенты изменились",
  BET_STAKE_TOO_LOW: "Ставка слишком маленькая",
  BET_STAKE_TOO_HIGH: "Ставка слишком большая",
  BET_MAX_PAYOUT_EXCEEDED: "Превышен максимальный выигрыш",

  // Payment errors
  PAYMENT_METHOD_UNAVAILABLE: "Метод оплаты недоступен",
  PAYMENT_AMOUNT_TOO_LOW: "Сумма слишком маленькая",
  PAYMENT_AMOUNT_TOO_HIGH: "Сумма слишком большая",
  PAYMENT_KYC_REQUIRED: "Требуется верификация",
  PAYMENT_PROVIDER_ERROR: "Ошибка платежного провайдера",

  // System errors
  RATE_LIMITED: "Слишком много запросов",
  UNKNOWN_ERROR: "Произошла неизвестная ошибка",
};

export function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    return errorMessages[error.error.code] ?? error.error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Произошла неизвестная ошибка";
}

export function getErrorTitle(error: unknown): string {
  if (error instanceof ApiClientError) {
    switch (error.error.code) {
      case "AUTH_INVALID_CREDENTIALS":
      case "AUTH_TOKEN_EXPIRED":
        return "Ошибка аутентификации";
      case "WALLET_INSUFFICIENT_BALANCE":
        return "Недостаточно средств";
      case "BET_EVENT_SUSPENDED":
      case "BET_MARKET_CLOSED":
        return "Ставка не принята";
      case "RATE_LIMITED":
        return "Слишком много запросов";
      default:
        return "Ошибка";
    }
  }
  return "Ошибка";
}
