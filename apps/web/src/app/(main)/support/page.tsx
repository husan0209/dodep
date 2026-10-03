import { Suspense } from 'react'
import { SupportPage } from '@components/pages/support'

export default function Support() {
  return (
    <Suspense
      fallback={<div className="flex h-screen items-center justify-center">Загрузка...</div>}
    >
      <SupportPage />
    </Suspense>
  )
}
