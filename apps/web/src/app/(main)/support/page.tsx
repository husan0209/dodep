import { Suspense } from 'react'
import { SupportPage } from '@components/pages/support'

export default function Support() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-screen">Загрузка...</div>}>
      <SupportPage />
    </Suspense>
  )
}
