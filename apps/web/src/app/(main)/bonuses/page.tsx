import { Suspense } from 'react'
import { BonusesPage } from '@components/pages/bonuses'

export default function Bonuses() {
  return (
    <Suspense
      fallback={<div className="flex h-screen items-center justify-center">Загрузка...</div>}
    >
      <BonusesPage />
    </Suspense>
  )
}
