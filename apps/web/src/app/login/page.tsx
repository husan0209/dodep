'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@stores/auth-store'
import { trackEvent } from '@lib/telemetry'

export default function LoginPage() {
  const router = useRouter()
  const { login } = useAuthStore()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      // Ensure email has no whitespace
      const cleanEmail = email.replace(/\s/g, '')
      trackEvent('auth_login_submitted', { emailDomain: cleanEmail.split('@')[1] || 'unknown' })
      await login(cleanEmail, password)
      router.push('/sportsbook')
    } catch (err: any) {
      // Show specific error message
      if (err?.error?.code === 'INVALID_CREDENTIALS') {
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

  return (
    <div className="min-h-[calc(100vh-4rem)] flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8 relative overflow-hidden">
      <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-blue-600/10 rounded-full blur-3xl pointer-events-none" />
      <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-cyan-500/10 rounded-full blur-3xl pointer-events-none" />
      
      <div className="w-full max-w-md relative z-10 card !p-8">
        <div>
          <h2 className="mt-2 text-center text-3xl font-bold font-display text-white">
            Вход в аккаунт
          </h2>
          <p className="mt-4 text-center text-sm font-medium text-gray-400">
            Или{' '}
            <Link
              href="/register"
              className="font-bold text-blue-400 hover:text-blue-300 transition-colors"
            >
              создайте новый аккаунт
            </Link>
          </p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {error && (
            <div className="rounded-xl bg-red-900/40 border border-red-500/30 p-4">
              <p className="text-sm font-medium text-red-200">{error}</p>
            </div>
          )}

          <div className="space-y-5">
            <div>
              <label htmlFor="email" className="block text-sm font-semibold text-gray-300 mb-1 pl-1">
                Email
              </label>
              <input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                required
                suppressHydrationWarning
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="input-field"
                placeholder="you@example.com"
              />
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-semibold text-gray-300 mb-1 pl-1">
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
              <label htmlFor="remember-me" className="ml-2 block text-sm font-medium text-gray-400 cursor-pointer hover:text-white transition-colors">
                Запомнить меня
              </label>
            </div>

            <div className="text-sm font-medium">
              <Link
                href="/forgot-password"
                className="text-gray-400 hover:text-blue-400 transition-colors"
              >
                Забыли пароль?
              </Link>
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="btn-primary w-full py-3 text-lg disabled:opacity-50 mt-4"
          >
            {isLoading ? 'Выполняем вход...' : 'Войти'}
          </button>
        </form>
      </div>
    </div>
  )
}
