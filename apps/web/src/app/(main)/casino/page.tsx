import { Suspense } from 'react'
import { CasinoPage } from '@components/pages/casino'

export default function Casino() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-screen">Загрузка...</div>}>
      <CasinoPage />
    </Suspense>
  )
}
