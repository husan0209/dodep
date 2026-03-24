import { Suspense } from 'react'
import { ProfilePage } from '@components/pages/profile'

export default function Profile() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-screen">Загрузка...</div>}>
      <ProfilePage />
    </Suspense>
  )
}
