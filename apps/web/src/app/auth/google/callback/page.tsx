"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Reads the OAuth tokens from the query string, so it must live inside a
 * Suspense boundary: `useSearchParams()` opts the component out of static
 * prerendering and Next.js fails the build with
 * "useSearchParams() should be wrapped in a suspense boundary" otherwise.
 */
function GoogleCallbackInner() {
  const router = useRouter();
  const params = useSearchParams();
  const { setTokens, fetchUser } = useAuthStore();

  useEffect(() => {
    const accessToken = params.get("access_token");
    const refreshToken = params.get("refresh_token");
    const errorCode = params.get("error_code");
    const errorMessage = params.get("error_message");

    if (errorCode) {
      router.replace(
        `/login?error=${encodeURIComponent(errorCode)}&message=${encodeURIComponent(errorMessage || "OAuth failed")}`,
      );
      return;
    }

    if (!accessToken || !refreshToken) {
      router.replace("/login?error=AUTH_OAUTH_INVALID_CALLBACK");
      return;
    }

    setTokens(accessToken, refreshToken);
    fetchUser()
      .then(() => router.replace("/sportsbook"))
      .catch(() => router.replace("/login?error=AUTH_OAUTH_USER_FETCH_FAILED"));
  }, [fetchUser, params, router, setTokens]);

  return (
    <div className="min-h-[calc(100vh-4rem)] flex items-center justify-center px-4">
      <div className="card !p-8 text-center">
        <h1 className="text-2xl font-bold text-white">Signing you in...</h1>
        <p className="mt-2 text-gray-400">
          Google authorization complete, redirecting to sportsbook.
        </p>
      </div>
    </div>
  );
}

export default function GoogleCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-[calc(100vh-4rem)] flex items-center justify-center px-4">
          <p className="text-gray-400">Completing sign-in...</p>
        </div>
      }
    >
      <GoogleCallbackInner />
    </Suspense>
  );
}
