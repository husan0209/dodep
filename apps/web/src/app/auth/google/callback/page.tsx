import { Suspense } from "react";

import GoogleCallbackInner from "./google-callback-inner";

export default function GoogleCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-[calc(100vh-4rem)] flex items-center justify-center px-4">
          <div className="card !p-8 text-center">
            <h1 className="text-2xl font-bold text-white">Signing you in...</h1>
            <p className="mt-2 text-gray-400">
              Google authorization complete, redirecting to sportsbook.
            </p>
          </div>
        </div>
      }
    >
      <GoogleCallbackInner />
    </Suspense>
  );
}
