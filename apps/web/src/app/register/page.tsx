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
    const value = e.target.value;
    const name = e.target.name;
    
    if (name === 'email') {
      setFormData({
        ...formData,
        [name]: value.replace(/\s/g, ''),
      });
    } else if (name === 'username') {
      setFormData({
        ...formData,
        [name]: value.trim(),
      });
    } else {
      setFormData({
        ...formData,
        [name]: value,
      });
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
      await register(cleanEmail, formData.password, formData.username.trim(), formData.countryCode, formData.currencyCode)
      
      router.replace('/sportsbook')
    } catch (err: any) {
      if (err?.error?.code === 'USER_ALREADY_EXISTS' || err?.error?.code === 'AUTH_USER_ALREADY_EXISTS') {
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
    <div className="min-h-[calc(100vh-4rem)] flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8 relative overflow-hidden">
      <div className="absolute top-1/3 right-1/4 w-[500px] h-[500px] bg-blue-600/10 rounded-full blur-3xl pointer-events-none" />
      <div className="absolute bottom-1/4 left-1/4 w-[400px] h-[400px] bg-cyan-500/10 rounded-full blur-3xl pointer-events-none" />
      
      <div className="w-full max-w-xl relative z-10 card !p-8 md:!p-10">
        <div>
          <h2 className="mt-2 text-center text-4xl font-bold font-display text-white">
            Регистрация
          </h2>
          <p className="mt-4 text-center text-sm font-medium text-gray-400">
            Или{' '}
            <Link
              href="/login"
              className="font-bold text-blue-400 hover:text-blue-300 transition-colors"
            >
              войдите в существующий аккаунт
            </Link>
          </p>
        </div>

        <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
          {error && (
            <div className="rounded-xl bg-red-900/40 border border-red-500/30 p-4">
              <p className="text-sm font-medium text-red-200">{error}</p>
            </div>
          )}

          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <div className="md:col-span-2">
              <label htmlFor="username" className="block text-sm font-semibold text-gray-300 mb-1 pl-1">
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
                value={formData.email}
                onChange={handleChange}
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
              <label htmlFor="confirmPassword" className="block text-sm font-semibold text-gray-300 mb-1 pl-1">
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
              <label htmlFor="countryCode" className="block text-sm font-semibold text-gray-300 mb-1 pl-1">
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
              <label htmlFor="currencyCode" className="block text-sm font-semibold text-gray-300 mb-1 pl-1">
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
              className="mt-1 h-4 w-4 rounded border-white/10 bg-black/40 text-primary-600 focus:ring-primary-500/50"
            />
            <label htmlFor="terms" className="ml-3 block text-sm font-medium text-gray-400">
              Я согласен с{' '}
              <Link href="/terms" className="text-blue-400 hover:text-white transition-colors underline underline-offset-2">
                условиями использования
              </Link>{' '}
              и{' '}
              <Link href="/privacy" className="text-blue-400 hover:text-white transition-colors underline underline-offset-2">
                политикой конфиденциальности
              </Link>
            </label>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="btn-primary w-full py-3.5 text-lg mt-6"
          >
            {isLoading ? 'Регистрация...' : 'Создать аккаунт'}
          </button>

          <button
            type="button"
            onClick={handleGoogleRegister}
            className="w-full py-3.5 text-lg rounded-xl border border-white/20 text-white hover:bg-white/10 transition-colors"
          >
            Continue with Google
          </button>
        </form>
      </div>
    </div>
  )
}
