'use client'

const mockBalances = [
  { currency: 'RUB', balance: 15000, locked: 500 },
  { currency: 'USD', balance: 100, locked: 20 },
  { currency: 'EUR', balance: 50, locked: 0 },
]

export function Balances() {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
      {mockBalances.map((wallet) => (
        <div key={wallet.currency} className="card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-2xl">
              {wallet.currency === 'RUB' && '🇷🇺'}
              {wallet.currency === 'USD' && '🇺🇸'}
              {wallet.currency === 'EUR' && '🇪🇺'}
            </span>
            <span className="text-sm font-medium text-gray-500 dark:text-gray-400">
              {wallet.currency}
            </span>
          </div>
          <p className="text-2xl font-bold text-gray-900 dark:text-white">
            {wallet.balance.toLocaleString('ru-RU')}
          </p>
          {wallet.locked > 0 && (
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              В ставках: {wallet.locked.toLocaleString('ru-RU')}
            </p>
          )}
        </div>
      ))}
    </div>
  )
}
