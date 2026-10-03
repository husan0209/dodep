import { Suspense } from 'react'
import { CasinoPage } from '@components/pages/casino'

export default function Casino() {
  return (
    <Suspense
      fallback={<div className="flex h-screen items-center justify-center">Загрузка...</div>}
    >
      <CasinoPage />
    </Suspense>
  )
}
