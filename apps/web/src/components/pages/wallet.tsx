'use client'

import { useState } from 'react'
import { Balances } from '@components/wallet/balances'
import { DepositForm } from '@components/wallet/deposit-form'
import { WithdrawForm } from '@components/wallet/withdraw-form'
import { TransactionHistory } from '@components/wallet/transaction-history'

const mockTransactions = [
  {
    id: '1',
    type: 'deposit',
    amount: 1000,
    currency: 'RUB',
    status: 'completed',
    method: 'Card',
    createdAt: '2024-03-24T10:00:00Z',
  },
  {
    id: '2',
    type: 'withdraw',
    amount: 500,
    currency: 'RUB',
    status: 'pending',
    method: 'Bank Transfer',
    createdAt: '2024-03-23T15:30:00Z',
  },
  {
    id: '3',
    type: 'bet',
    amount: -100,
    currency: 'RUB',
    status: 'completed',
    method: 'Bet',
    createdAt: '2024-03-23T12:00:00Z',
  },
]

export function WalletPage() {
  const [activeTab, setActiveTab] = useState<'deposit' | 'withdraw' | 'history'>('deposit')

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold font-display text-gray-900 dark:text-white mb-6">
        Кошелёк
      </h1>

      {/* Balances */}
      <Balances />

      {/* Tabs */}
      <div className="flex space-x-4 mb-6 border-b border-gray-200 dark:border-gray-700">
        <button
          onClick={() => setActiveTab('deposit')}
          className={`pb-3 px-4 text-sm font-medium transition-colors ${
            activeTab === 'deposit'
              ? 'text-primary-600 border-b-2 border-primary-600'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white'
          }`}
        >
          Депозит
        </button>
        <button
          onClick={() => setActiveTab('withdraw')}
          className={`pb-3 px-4 text-sm font-medium transition-colors ${
            activeTab === 'withdraw'
              ? 'text-primary-600 border-b-2 border-primary-600'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white'
          }`}
        >
          Вывод
        </button>
        <button
          onClick={() => setActiveTab('history')}
          className={`pb-3 px-4 text-sm font-medium transition-colors ${
            activeTab === 'history'
              ? 'text-primary-600 border-b-2 border-primary-600'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white'
          }`}
        >
          История
        </button>
      </div>

      {/* Content */}
      <div className="card">
        {activeTab === 'deposit' && <DepositForm />}
        {activeTab === 'withdraw' && <WithdrawForm />}
        {activeTab === 'history' && <TransactionHistory transactions={mockTransactions} />}
      </div>
    </div>
  )
}
