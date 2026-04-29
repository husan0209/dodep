'use client'

import { useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useAuthStore } from '@/stores/auth-store'

export default function GoogleCallbackPage() {
  const router = useRouter()
  const params = useSearchParams()
  const { setTokens, fetchUser } = useAuthStore()

  useEffect(() => {
    const accessToken = params.get('access_token')
    const refreshToken = params.get('refresh_token')
    const errorCode = params.get('error_code')
    const errorMessage = params.get('error_message')

    if (errorCode) {
      router.replace(`/login?error=${encodeURIComponent(errorCode)}&message=${encodeURIComponent(errorMessage || 'OAuth failed')}`)
      return
    }

    if (!accessToken || !refreshToken) {
      router.replace('/login?error=AUTH_OAUTH_INVALID_CALLBACK')
      return
    }

    setTokens(accessToken, refreshToken)
    fetchUser()
      .then(() => router.replace('/sportsbook'))
      .catch(() => router.replace('/login?error=AUTH_OAUTH_USER_FETCH_FAILED'))
  }, [fetchUser, params, router, setTokens])

  return (
    <div className="min-h-[calc(100vh-4rem)] flex items-center justify-center px-4">
      <div className="card !p-8 text-center">
        <h1 className="text-2xl font-bold text-white">Signing you in...</h1>
        <p className="mt-2 text-gray-400">Google authorization complete, redirecting to sportsbook.</p>
      </div>
    </div>
  )
}
