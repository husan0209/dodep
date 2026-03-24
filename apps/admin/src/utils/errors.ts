import { type AxiosError } from "axios";

interface ApiErrorResponse {
  error?: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
}

const ERROR_MESSAGES: Record<string, string> = {
  WALLET_INSUFFICIENT_BALANCE: "Insufficient balance",
  BET_ODDS_CHANGED: "Odds have changed",
  BET_EVENT_SUSPENDED: "Event is suspended",
  BET_MARKET_CLOSED: "Market is closed",
  AUTH_INVALID_CREDENTIALS: "Invalid email or password",
  AUTH_ACCOUNT_LOCKED: "Account is locked",
  AUTH_TOKEN_EXPIRED: "Session expired",
  RATE_LIMITED: "Too many requests, please try again later",
  USER_BLOCKED: "User is blocked",
  WITHDRAWAL_LIMIT_EXCEEDED: "Withdrawal limit exceeded",
  KYC_REQUIRED: "KYC verification required",
};

export function getErrorMessage(error: unknown): string {
  if (isAxiosError(error)) {
    const data = error.response?.data as ApiErrorResponse | undefined;
    const code = data?.error?.code;
    if (code && ERROR_MESSAGES[code]) {
      return ERROR_MESSAGES[code];
    }
    return data?.error?.message || error.message || "Request failed";
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "An unexpected error occurred";
}

function isAxiosError(error: unknown): error is AxiosError<ApiErrorResponse> {
  return (
    typeof error === "object" &&
    error !== null &&
    "isAxiosError" in error &&
    (error as AxiosError).isAxiosError === true
  );
}
