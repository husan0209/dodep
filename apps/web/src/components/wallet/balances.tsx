'use client'

const mockBalances = [
  { currency: 'RUB', balance: 15000, locked: 500 },
  { currency: 'USD', balance: 100, locked: 20 },
  { currency: 'EUR', balance: 50, locked: 0 },
]

export function Balances() {
  return (
    <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
      {mockBalances.map((wallet) => (
        <div key={wallet.currency} className="card p-4">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-2xl">
              {wallet.currency === 'RUB' && '🇷🇺'}
              {wallet.currency === 'USD' && '🇺🇸'}
              {wallet.currency === 'EUR' && '🇪🇺'}
            </span>
            <span className="text-sm font-medium text-gray-500 dark:text-gray-400">
              {wallet.currency}
            </span>
          </div>
          <p className="text-2xl font-bold text-white">{wallet.balance.toLocaleString('ru-RU')}</p>
          {wallet.locked > 0 && (
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              В ставках: {wallet.locked.toLocaleString('ru-RU')}
            </p>
          )}
        </div>
      ))}
    </div>
  )
}
