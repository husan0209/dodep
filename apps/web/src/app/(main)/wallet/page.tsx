import { Suspense } from 'react'
import { WalletPage } from '@components/pages/wallet'

export default function Wallet() {
  return (
    <Suspense
      fallback={<div className="flex h-screen items-center justify-center">Загрузка...</div>}
    >
      <WalletPage />
    </Suspense>
  )
}
