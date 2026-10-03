import { Suspense } from 'react'
import { ProfilePage } from '@components/pages/profile'

export default function Profile() {
  return (
    <Suspense
      fallback={<div className="flex h-screen items-center justify-center">Загрузка...</div>}
    >
      <ProfilePage />
    </Suspense>
  )
}
