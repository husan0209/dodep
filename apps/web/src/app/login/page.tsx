'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@stores/auth-store'
import { trackEvent } from '@lib/telemetry'
import { authApi } from '@lib/api/auth'

export default function LoginPage() {
  const router = useRouter()
  const { login } = useAuthStore()
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      const cleanIdentifier = identifier.replace(/\s/g, '')
      trackEvent('auth_login_submitted', {
        identifierType: cleanIdentifier.includes('@') ? 'email' : 'username',
      })
      await login(cleanIdentifier, password)
      router.replace('/sportsbook')
    } catch (err: any) {
      // Show specific error message
      if (err?.error?.code === 'AUTH_INVALID_CREDENTIALS') {
        setError('Неверный email или пароль')
      } else if (err?.error?.message) {
        setError(err.error.message)
      } else {
        setError('Ошибка входа. Проверьте email и пароль.')
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleGoogleLogin = () => {
    window.location.href = authApi.getGoogleStartUrl()
  }

  return (
    <div className="relative flex min-h-[calc(100vh-4rem)] items-center justify-center overflow-hidden px-4 py-12 sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute left-1/4 top-1/4 h-96 w-96 rounded-full bg-blue-600/10 blur-3xl" />
      <div className="pointer-events-none absolute bottom-1/4 right-1/4 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />

      <div className="card relative z-10 w-full max-w-md !p-8">
        <div>
          <h2 className="mt-2 text-center font-display text-3xl font-bold text-white">
            Вход в аккаунт
          </h2>
          <p className="mt-4 text-center text-sm font-medium text-gray-400">
            Или{' '}
            <Link
              href="/register"
              className="font-bold text-blue-400 transition-colors hover:text-blue-300"
            >
              создайте новый аккаунт
            </Link>
          </p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {error && (
            <div className="rounded-xl border border-red-500/30 bg-red-900/40 p-4">
              <p className="text-sm font-medium text-red-200">{error}</p>
            </div>
          )}

          <div className="space-y-5">
            <div>
              <label
                htmlFor="email"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Email или Username
              </label>
              <input
                id="identifier"
                name="identifier"
                type="text"
                autoComplete="username"
                required
                suppressHydrationWarning
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                className="input-field"
                placeholder="you@example.com или your_username"
              />
            </div>

            <div>
              <label
                htmlFor="password"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Пароль
              </label>
              <input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                suppressHydrationWarning
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="input-field"
                placeholder="••••••••"
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-2">
            <div className="flex items-center">
              <input
                id="remember-me"
                name="remember-me"
                type="checkbox"
                suppressHydrationWarning
                className="h-4 w-4 rounded border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] text-blue-600 focus:ring-blue-500/50"
              />
              <label
                htmlFor="remember-me"
                className="ml-2 block cursor-pointer text-sm font-medium text-gray-400 transition-colors hover:text-white"
              >
                Запомнить меня
              </label>
            </div>

            <div className="text-sm font-medium">
              <Link
                href="/forgot-password"
                className="text-gray-400 transition-colors hover:text-blue-400"
              >
                Забыли пароль?
              </Link>
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="btn-primary mt-4 w-full py-3 text-lg disabled:opacity-50"
          >
            {isLoading ? 'Выполняем вход...' : 'Войти'}
          </button>

          <button
            type="button"
            onClick={handleGoogleLogin}
            className="w-full rounded-xl border border-white/20 py-3 text-lg text-white transition-colors hover:bg-white/10"
          >
            Continue with Google
          </button>
        </form>
      </div>
    </div>
  )
}
