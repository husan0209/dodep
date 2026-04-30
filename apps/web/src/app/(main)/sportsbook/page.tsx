import { Suspense } from 'react'
import { SportsbookPage } from '@components/pages/sportsbook'

export default function Sportsbook() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-screen">Загрузка...</div>}>
      <SportsbookPage />
    </Suspense>
  )
}
