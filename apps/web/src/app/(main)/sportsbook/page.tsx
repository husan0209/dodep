import { Suspense } from 'react'
import { SportsbookPage } from '@components/pages/sportsbook'

export default function Sportsbook() {
  return (
    <Suspense
      fallback={<div className="flex h-screen items-center justify-center">Загрузка...</div>}
    >
      <SportsbookPage />
    </Suspense>
  )
}
