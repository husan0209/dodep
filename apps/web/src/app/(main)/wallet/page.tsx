import { Suspense } from 'react'
import { WalletPage } from '@components/pages/wallet'

export default function Wallet() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-screen">Загрузка...</div>}>
      <WalletPage />
    </Suspense>
  )
}
