import { create } from 'zustand'

interface Notification {
  id: string
  type: string
  title: string
  message: string
  isRead: boolean
  createdAt: string
}

interface NotificationState {
  notifications: Notification[]
  unreadCount: number
  isLoading: boolean
  
  // Actions
  setNotifications: (notifications: Notification[]) => void
  addNotification: (notification: Notification) => void
  markAsRead: (notificationId: string) => void
  markAllAsRead: () => void
  setUnreadCount: (count: number) => void
  fetchNotifications: () => Promise<void>
}

export const useNotificationStore = create<NotificationState>((set, get) => ({
  notifications: [],
  unreadCount: 0,
  isLoading: false,

  setNotifications: (notifications: Notification[]) => {
    const unreadCount = notifications.filter((n) => !n.isRead).length
    set({ notifications, unreadCount })
  },

  addNotification: (notification: Notification) => {
    const newNotifications = [notification, ...get().notifications]
    const unreadCount = notification.isRead
      ? get().unreadCount
      : get().unreadCount + 1
    set({ notifications: newNotifications, unreadCount })
  },

  markAsRead: (notificationId: string) => {
    const newNotifications = get().notifications.map((n) =>
      n.id === notificationId ? { ...n, isRead: true } : n
    )
    const unreadCount = Math.max(0, get().unreadCount - 1)
    set({ notifications: newNotifications, unreadCount })
  },

  markAllAsRead: () => {
    const newNotifications = get().notifications.map((n) => ({
      ...n,
      isRead: true,
    }))
    set({ notifications: newNotifications, unreadCount: 0 })
  },

  setUnreadCount: (count: number) => {
    set({ unreadCount: count })
  },

  fetchNotifications: async () => {
    set({ isLoading: true })
    try {
      // This would call the API
      // const response = await api.notifications.getList({ limit: 20 })
      // setNotifications(response.data.notifications)
      set({ isLoading: false })
    } catch (error) {
      console.error('Failed to fetch notifications:', error)
      set({ isLoading: false })
    }
  },
}))
