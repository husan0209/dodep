import { Suspense } from 'react'
import { BonusesPage } from '@components/pages/bonuses'

export default function Bonuses() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-screen">Загрузка...</div>}>
      <BonusesPage />
    </Suspense>
  )
}
