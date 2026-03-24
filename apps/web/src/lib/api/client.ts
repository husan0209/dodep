import { useAuthStore } from "@/stores/auth-store";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
  ): Promise<ApiResponse<T>> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (options.params) {
      Object.entries(options.params).forEach(([k, v]) =>
        url.searchParams.set(k, v)
      );
    }

    const { accessToken, refreshTokens, logout } =
      useAuthStore.getState();

    let token = accessToken;

    const makeRequest = async (t: string | null) => {
      const response = await fetch(url.toString(), {
        method,
        headers: {
          "Content-Type": "application/json",
          ...(t ? { Authorization: `Bearer ${t}` } : {}),
          "X-Request-ID": crypto.randomUUID(),
          ...options.headers,
        },
        body: options.body ? JSON.stringify(options.body) : undefined,
      });
      return response;
    };

    let response = await makeRequest(token);

    // Auto-refresh token on 401
    if (response.status === 401 && token) {
      try {
        await refreshTokens();
        token = useAuthStore.getState().accessToken;
        response = await makeRequest(token);
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

    if (!response.ok) {
      const error: ApiError = await response.json().catch(() => ({
        code: "UNKNOWN_ERROR",
        message: response.statusText,
        request_id: "unknown",
      }));
      throw new ApiClientError(response.status, error);
    }

    return response.json();
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

export const api = new ApiClient(API_BASE);
