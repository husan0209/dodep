'use client'

interface Transaction {
  id: string
  type: 'deposit' | 'withdraw' | 'bet' | 'win' | 'bonus'
  amount: number
  currency: string
  status: 'pending' | 'completed' | 'failed'
  method: string
  createdAt: string
}

interface TransactionHistoryProps {
  transactions: Transaction[]
}

export function TransactionHistory({ transactions }: TransactionHistoryProps) {
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/20'
      case 'pending':
        return 'text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/20'
      case 'failed':
        return 'text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900/20'
      default:
        return 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-900/20'
    }
  }

  const getTypeLabel = (type: string) => {
    switch (type) {
      case 'deposit':
        return 'Пополнение'
      case 'withdraw':
        return 'Вывод'
      case 'bet':
        return 'Ставка'
      case 'win':
        return 'Выигрыш'
      case 'bonus':
        return 'Бонус'
      default:
        return type
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-white">История транзакций</h3>
        <button className="text-xs text-blue-400 hover:text-blue-300">Скачать выписку</button>
      </div>

      {transactions.length === 0 ? (
        <div className="py-8 text-center">
          <p className="text-gray-500 dark:text-gray-400">Транзакций нет</p>
        </div>
      ) : (
        <div className="space-y-3">
          {transactions.map((tx) => (
            <div
              key={tx.id}
              className="flex items-center justify-between rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3"
            >
              <div className="flex items-center space-x-4">
                <div
                  className={`flex h-10 w-10 items-center justify-center rounded-full ${
                    tx.amount > 0
                      ? 'bg-green-100 text-green-600 dark:bg-green-900/20 dark:text-green-400'
                      : 'bg-red-100 text-red-600 dark:bg-red-900/20 dark:text-red-400'
                  }`}
                >
                  {tx.amount > 0 ? '↑' : '↓'}
                </div>
                <div>
                  <p className="text-xs font-medium text-gray-100">{getTypeLabel(tx.type)}</p>
                  <p className="text-[10px] text-gray-500">
                    {tx.method} • {new Date(tx.createdAt).toLocaleDateString('ru-RU')}
                  </p>
                </div>
              </div>
              <div className="text-right">
                <p
                  className={`text-sm font-semibold ${
                    tx.amount > 0
                      ? 'text-green-600 dark:text-green-400'
                      : 'text-red-600 dark:text-red-400'
                  }`}
                >
                  {tx.amount > 0 ? '+' : ''}
                  {tx.amount.toLocaleString('ru-RU')} {tx.currency}
                </p>
                <span
                  className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${getStatusColor(tx.status)}`}
                >
                  {tx.status === 'completed'
                    ? 'Завершено'
                    : tx.status === 'pending'
                      ? 'В обработке'
                      : 'Ошибка'}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
