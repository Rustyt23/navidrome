const listeners = new Set()

const initialNotifications = [
  {
    id: 1,
    title: 'New album added',
    description: 'Across 110th Street by Bobby Womack is now available.',
    time: '2 minutes ago',
  },
  {
    id: 2,
    title: 'Playlist updated',
    description: 'Jazz Essentials was refreshed with 5 new tracks.',
    time: '12 minutes ago',
  },
  {
    id: 3,
    title: 'Sync complete',
    description: 'Library sync finished without any conflicts.',
    time: 'Yesterday',
  },
]

let notifications = [...initialNotifications]
let counter = initialNotifications.length + 1

const notifySubscribers = () => {
  listeners.forEach((listener) => {
    try {
      listener([...notifications])
    } catch (err) {
      // Ignore subscriber errors
    }
  })
}

export const subscribeToNotifications = (listener) => {
  listeners.add(listener)
  listener([...notifications])
  return () => {
    listeners.delete(listener)
  }
}

export const getNotifications = () => [...notifications]

export const clearNotifications = () => {
  if (notifications.length === 0) {
    return
  }
  notifications = []
  notifySubscribers()
}

export const pushNotification = ({ id, ...rest }) => {
  const notification = {
    id: id ?? counter++,
    ...rest,
  }
  notifications = [notification, ...notifications]
  notifySubscribers()
  return notification
}

