import { useAuthStore } from "@/stores/auth-store";

const DEFAULT_API_BASE = "http://localhost:8080";
const DEFAULT_AUTH_API_BASE = "http://localhost:8083";

type PublicEnv = Record<string, string | undefined>;

export function resolveApiBaseUrl(env: PublicEnv = process.env): string {
  return env.NEXT_PUBLIC_API_URL || DEFAULT_API_BASE;
}

export function resolveAuthApiBaseUrl(env: PublicEnv = process.env): string {
  return env.NEXT_PUBLIC_AUTH_API_URL || env.NEXT_PUBLIC_API_URL || DEFAULT_AUTH_API_BASE;
}

export interface ApiResponse<T> {
  data: T;
  meta?: {
    page?: number;
    total?: number;
    cursor?: string;
  };
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  request_id: string;
}

function normalizeApiError(raw: unknown, fallbackMessage: string): ApiError {
  if (raw && typeof raw === "object") {
    const payload = raw as Record<string, unknown>;
    const nestedError =
      payload.error && typeof payload.error === "object"
        ? (payload.error as Record<string, unknown>)
        : null;
    const legacyError = typeof payload.error === "string" ? payload.error : null;
    const message =
      typeof nestedError?.message === "string"
        ? nestedError.message
        : typeof payload.message === "string"
        ? payload.message
        : legacyError ?? fallbackMessage;

    return {
      code:
        typeof nestedError?.code === "string"
          ? nestedError.code
          : typeof payload.code === "string"
          ? payload.code
          : "UNKNOWN_ERROR",
      message,
      details:
        nestedError?.details && typeof nestedError.details === "object"
          ? (nestedError.details as Record<string, unknown>)
          : payload.details && typeof payload.details === "object"
          ? (payload.details as Record<string, unknown>)
          : undefined,
      request_id:
        typeof payload.request_id === "string"
          ? payload.request_id
          : "unknown",
    };
  }

  return {
    code: "UNKNOWN_ERROR",
    message: fallbackMessage,
    request_id: "unknown",
  };
}

export class ApiClientError extends Error {
  constructor(
    public status: number,
    public error: ApiError
  ) {
    super(error.message);
    this.name = "ApiClientError";
  }

  get isNotFound() { return this.status === 404; }
  get isUnauthorized() { return this.status === 401; }
  get isForbidden() { return this.status === 403; }
  get isValidation() { return this.status === 422; }
  get isRateLimited() { return this.status === 429; }
}

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    method: string,
    path: string,
    options: {
      body?: unknown;
      params?: Record<string, string>;
      headers?: Record<string, string>;
    } = {}
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (options.params) {
      Object.entries(options.params).forEach(([k, v]) =>
        url.searchParams.set(k, v)
      );
    }

    const { accessToken, refreshTokens, logout } =
      useAuthStore.getState();

    let token = accessToken;

    const makeRequest = async (t: string | null): Promise<T> => {
      const requestId = crypto.randomUUID();
      const startedAt = Date.now();
      let response: Response;
      try {
        response = await fetch(url.toString(), {
          method,
          headers: {
            "Content-Type": "application/json",
            ...(t ? { Authorization: `Bearer ${t}` } : {}),
            "X-Request-ID": requestId,
            ...options.headers,
          },
          body: options.body ? JSON.stringify(options.body) : undefined,
        });
      } catch (error) {
        const message =
          `Network error while calling ${method} ${url.toString()}. ` +
          "Backend is unreachable or refused the connection.";
        console.error("[api] network_error", {
          requestId,
          method,
          url: url.toString(),
          elapsedMs: Date.now() - startedAt,
          error,
        });
        throw new ApiClientError(0, {
          code: "NETWORK_CONNECTION_REFUSED",
          message,
          details: {
            method,
            url: url.toString(),
            elapsed_ms: Date.now() - startedAt,
          },
          request_id: requestId,
        });
      }
      
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        const error = normalizeApiError(payload, response.statusText);
        throw new ApiClientError(response.status, error);
      }
      
      return response.json();
    };

    try {
      return await makeRequest(token);
    } catch (error) {
      // Auto-refresh token on 401
      if (error instanceof ApiClientError && error.isUnauthorized && token) {
        try {
          await refreshTokens();
          token = useAuthStore.getState().accessToken;
          return await makeRequest(token);
        } catch {
          logout();
          window.location.href = "/login";
          throw new ApiClientError(401, {
            code: "AUTH_TOKEN_EXPIRED",
            message: "Session expired",
            request_id: "unknown",
          });
        }
      }
      throw error;
    }
  }

  get<T>(path: string, params?: Record<string, string>) {
    return this.request<T>("GET", path, { params });
  }

  post<T>(path: string, body?: unknown) {
    return this.request<T>("POST", path, { body });
  }

  patch<T>(path: string, body?: unknown) {
    return this.request<T>("PATCH", path, { body });
  }

  delete<T>(path: string) {
    return this.request<T>("DELETE", path);
  }
}

export function createApiClient(baseUrl: string) {
  return new ApiClient(baseUrl);
}

export const api = createApiClient(resolveApiBaseUrl());
export const authApiClient = createApiClient(resolveAuthApiBaseUrl());
