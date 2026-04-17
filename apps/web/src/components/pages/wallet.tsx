'use client'

import { useState } from 'react'
import { Balances } from '@components/wallet/balances'
import { DepositForm } from '@components/wallet/deposit-form'
import { WithdrawForm } from '@components/wallet/withdraw-form'
import { TransactionHistory } from '@components/wallet/transaction-history'

const mockTransactions = [
  { id: '1', type: 'deposit' as const, amount: 1000, currency: 'RUB', status: 'completed' as const, method: 'Card', createdAt: '2024-03-24T10:00:00Z' },
  { id: '2', type: 'withdraw' as const, amount: 500, currency: 'RUB', status: 'pending' as const, method: 'Bank Transfer', createdAt: '2024-03-23T15:30:00Z' },
  { id: '3', type: 'bet' as const, amount: -100, currency: 'RUB', status: 'completed' as const, method: 'Bet', createdAt: '2024-03-23T12:00:00Z' },
]

export function WalletPage() {
  const [activeTab, setActiveTab] = useState<'deposit' | 'withdraw' | 'history'>('deposit')

  return (
    <div className="section max-w-2xl">
      <h1 className="text-sm font-bold text-white mb-4">Кошелёк</h1>

      <div className="mb-4">
        <Balances />
      </div>

      {/* Tabs */}
      <div className="flex gap-0.5 mb-3 bg-[rgb(var(--bg-primary))] p-0.5 rounded">
        {([
          { id: 'deposit', label: 'Пополнить' },
          { id: 'withdraw', label: 'Вывести' },
          { id: 'history', label: 'История' },
        ] as const).map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex-1 py-1.5 text-xs font-medium rounded transition-colors ${
              activeTab === tab.id ? 'bg-blue-600 text-white' : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <div className="card p-3 fade-in">
        {activeTab === 'deposit' && <DepositForm />}
        {activeTab === 'withdraw' && <WithdrawForm />}
        {activeTab === 'history' && <TransactionHistory transactions={mockTransactions} />}
      </div>
    </div>
  )
}
