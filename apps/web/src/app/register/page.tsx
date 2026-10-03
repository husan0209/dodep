'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@stores/auth-store'
import { trackEvent } from '@lib/telemetry'
import { authApi } from '@lib/api/auth'

export default function RegisterPage() {
  const router = useRouter()
  const { register } = useAuthStore()
  const [formData, setFormData] = useState({
    username: '',
    email: '',
    password: '',
    confirmPassword: '',
    countryCode: 'RU',
    currencyCode: 'RUB',
  })
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const handleGoogleRegister = () => {
    window.location.href = authApi.getGoogleStartUrl()
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const value = e.target.value
    const name = e.target.name

    if (name === 'email') {
      setFormData({
        ...formData,
        [name]: value.replace(/\s/g, ''),
      })
    } else if (name === 'username') {
      setFormData({
        ...formData,
        [name]: value.trim(),
      })
    } else {
      setFormData({
        ...formData,
        [name]: value,
      })
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isLoading) {
      return
    }
    setError('')

    if (formData.password !== formData.confirmPassword) {
      setError('Пароли не совпадают')
      return
    }

    if (formData.password.length < 8) {
      setError('Пароль должен содержать минимум 8 символов')
      return
    }

    setIsLoading(true)

    try {
      const cleanEmail = formData.email.replace(/\s/g, '')
      trackEvent('auth_register_submitted', {
        countryCode: formData.countryCode,
        currencyCode: formData.currencyCode,
      })
      await register(
        cleanEmail,
        formData.password,
        formData.username.trim(),
        formData.countryCode,
        formData.currencyCode,
      )

      router.replace('/sportsbook')
    } catch (err: any) {
      if (
        err?.error?.code === 'USER_ALREADY_EXISTS' ||
        err?.error?.code === 'AUTH_USER_ALREADY_EXISTS'
      ) {
        setError('Пользователь с таким email уже существует. Войдите или используйте другой email.')
      } else if (err?.error?.message) {
        setError(err.error.message)
      } else {
        setError('Ошибка при регистрации. Попробуйте другой email.')
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="relative flex min-h-[calc(100vh-4rem)] items-center justify-center overflow-hidden px-4 py-12 sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute right-1/4 top-1/3 h-[500px] w-[500px] rounded-full bg-blue-600/10 blur-3xl" />
      <div className="pointer-events-none absolute bottom-1/4 left-1/4 h-[400px] w-[400px] rounded-full bg-cyan-500/10 blur-3xl" />

      <div className="card relative z-10 w-full max-w-xl !p-8 md:!p-10">
        <div>
          <h2 className="mt-2 text-center font-display text-4xl font-bold text-white">
            Регистрация
          </h2>
          <p className="mt-4 text-center text-sm font-medium text-gray-400">
            Или{' '}
            <Link
              href="/login"
              className="font-bold text-blue-400 transition-colors hover:text-blue-300"
            >
              войдите в существующий аккаунт
            </Link>
          </p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {error && (
            <div className="rounded-xl border border-red-500/30 bg-red-900/40 p-4">
              <p className="text-sm font-medium text-red-200">{error}</p>
            </div>
          )}

          <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
            <div className="md:col-span-2">
              <label
                htmlFor="username"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Имя пользователя
              </label>
              <input
                id="username"
                name="username"
                type="text"
                autoComplete="username"
                required
                suppressHydrationWarning
                value={formData.username}
                onChange={handleChange}
                className="input-field"
                placeholder="Ваш логин"
              />
            </div>

            <div className="md:col-span-2">
              <label
                htmlFor="email"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Email
              </label>
              <input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                required
                suppressHydrationWarning
                value={formData.email}
                onChange={handleChange}
                className="input-field"
                placeholder="you@example.com"
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
                autoComplete="new-password"
                required
                suppressHydrationWarning
                value={formData.password}
                onChange={handleChange}
                className="input-field"
                placeholder="••••••••"
              />
            </div>

            <div>
              <label
                htmlFor="confirmPassword"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Подтвердите пароль
              </label>
              <input
                id="confirmPassword"
                name="confirmPassword"
                type="password"
                autoComplete="new-password"
                required
                suppressHydrationWarning
                value={formData.confirmPassword}
                onChange={handleChange}
                className="input-field"
                placeholder="••••••••"
              />
            </div>

            <div>
              <label
                htmlFor="countryCode"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Страна
              </label>
              <select
                id="countryCode"
                name="countryCode"
                required
                value={formData.countryCode}
                onChange={handleChange}
                className="input-field [&>option]:bg-gray-800"
              >
                <option value="RU">Россия</option>
                <option value="UA">Украина</option>
                <option value="KZ">Казахстан</option>
                <option value="BY">Беларусь</option>
                <option value="UZ">Узбекистан</option>
                <option value="TR">Турция</option>
              </select>
            </div>

            <div>
              <label
                htmlFor="currencyCode"
                className="mb-1 block pl-1 text-sm font-semibold text-gray-300"
              >
                Валюта
              </label>
              <select
                id="currencyCode"
                name="currencyCode"
                required
                value={formData.currencyCode}
                onChange={handleChange}
                className="input-field [&>option]:bg-gray-800"
              >
                <option value="RUB">RUB — Рубль</option>
                <option value="USD">USD — Доллар США</option>
                <option value="EUR">EUR — Евро</option>
                <option value="KZT">KZT — Тенге</option>
                <option value="UAH">UAH — Гривна</option>
              </select>
            </div>
          </div>

          <div className="flex items-start pt-2">
            <input
              id="terms"
              name="terms"
              type="checkbox"
              required
              suppressHydrationWarning
              className="text-primary-600 focus:ring-primary-500/50 mt-1 h-4 w-4 rounded border-white/10 bg-black/40"
            />
            <label htmlFor="terms" className="ml-3 block text-sm font-medium text-gray-400">
              Я согласен с{' '}
              <Link
                href="/terms"
                className="text-blue-400 underline underline-offset-2 transition-colors hover:text-white"
              >
                условиями использования
              </Link>{' '}
              и{' '}
              <Link
                href="/privacy"
                className="text-blue-400 underline underline-offset-2 transition-colors hover:text-white"
              >
                политикой конфиденциальности
              </Link>
            </label>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="btn-primary mt-6 w-full py-3.5 text-lg"
          >
            {isLoading ? 'Регистрация...' : 'Создать аккаунт'}
          </button>

          <button
            type="button"
            onClick={handleGoogleRegister}
            className="w-full rounded-xl border border-white/20 py-3.5 text-lg text-white transition-colors hover:bg-white/10"
          >
            Continue with Google
          </button>
        </form>
      </div>
    </div>
  )
}
