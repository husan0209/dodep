import axios, { type AxiosError, type InternalAxiosRequestConfig } from "axios";
import { useAuthStore } from "@/stores/authStore";
import { API_BASE_URL } from "@/utils/constants";

function generateIdempotencyKey(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: { "Content-Type": "application/json" },
  // Phase 0.5: send cookies (admin_refresh_token is HttpOnly, scoped to
  // /admin/auth). Required for cross-origin dev (3001 → 8090) and prod.
  withCredentials: true,
});

apiClient.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = useAuthStore.getState().accessToken;
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }

  // Idempotency key for mutating requests
  const method = config.method?.toUpperCase();
  if (
    method &&
    ["POST", "PUT", "PATCH", "DELETE"].includes(method) &&
    config.headers
  ) {
    config.headers["X-Idempotency-Key"] = generateIdempotencyKey();
  }

  return config;
});

let isRefreshing = false;
let failedQueue: Array<{
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

function processQueue(error: unknown, token: string | null = null) {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token);
    }
  });
  failedQueue = [];
}

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & {
      _retry?: boolean;
    };

    if (error.response?.status === 401 && !originalRequest._retry) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        }).then((token) => {
          if (originalRequest.headers) {
            originalRequest.headers.Authorization = `Bearer ${token}`;
          }
          return apiClient(originalRequest);
        });
      }

      originalRequest._retry = true;
      isRefreshing = true;

      const { clearAuth, setTokens } = useAuthStore.getState();

      try {
        // Phase 0.5: refresh_token is read from the HttpOnly cookie by the
        // server. We must NOT send it from JS — that defeats the purpose.
        // withCredentials is required so the browser attaches the cookie.
        const response = await axios.post(
          `${API_BASE_URL}/admin/auth/refresh`,
          {},
          { withCredentials: true },
        );
        const { access_token, refresh_token: newRefreshToken } =
          response.data.data;
        // newRefreshToken is the deprecated body fallback; kept in state for
        // one release window only. Cookie is the authoritative store.
        setTokens(access_token, newRefreshToken ?? "");
        processQueue(null, access_token);
        if (originalRequest.headers) {
          originalRequest.headers.Authorization = `Bearer ${access_token}`;
        }
        return apiClient(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError, null);
        clearAuth();
        window.location.href = "/login";
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  },
);

export default apiClient;
