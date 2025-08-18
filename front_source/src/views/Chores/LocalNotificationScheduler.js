import { Capacitor } from '@capacitor/core'
import { LocalNotifications } from '@capacitor/local-notifications'
import { Preferences } from '@capacitor/preferences'
import murmurhash from 'murmurhash'

const getNotificationPreferences = async () => {
  const ret = await Preferences.get({ key: 'notificationPreferences' })
  return JSON.parse(ret.value)
}

const canScheduleNotification = async () => {
  if (Capacitor.isNativePlatform() === false) {
    return false
  }
  const notificationPreferences = await getNotificationPreferences()
  console.log('Notification preferences:', notificationPreferences)

  if (notificationPreferences['granted'] === false) {
    return false
  }
  return true
}

const getIdFromTemplate = (choreId, template) => {
  const hash = murmurhash.v3(`${choreId}-${template.value}-${template.unit}`)
  // Use Math.abs() with modulo to ensure positive ID within Java int range
  // This guarantees the ID is always positive and within 1 to 2^31-1
  return Math.abs(hash) % 2147483647
}

const getTimeFromTemplate = (template, relativeTime) => {
  let time = relativeTime
  switch (template.unit) {
    case 'm':
      time = new Date(relativeTime.getTime() + template.value * 60 * 1000)
      break
    case 'h':
      time = new Date(relativeTime.getTime() + template.value * 60 * 60 * 1000)
      break
    case 'd':
      time = new Date(
        relativeTime.getTime() + template.value * 24 * 60 * 60 * 1000,
      )
      break
    default:
      time = relativeTime
  }
  return time
}
const scheduleNotificationFromTemplate = (
  chore,
  userProfile,
  allPerformers,
  notifications,
) => {
  for (const template of chore.notificationMetadata?.templates || []) {
    // convert the template to time:
    const dueDate = new Date(chore.nextDueDate)
    const now = new Date()
    const time = getTimeFromTemplate(template, dueDate)
    const notificationId = getIdFromTemplate(chore.id, template)
    const { title, body } = getNotificationText(chore.name, template)
    if (time > now) {
      notifications.push({
        title,
        body: `${body} at ${time.toLocaleTimeString()}`,
        id: notificationId,
        allowWhileIdle: true,
        schedule: {
          at: time,
        },
        extra: {
          choreId: chore.id,
        },
      })
    }
  }
}

const getNotificationText = (choreName, template = {}) => {
  // Determine notification type based on template value
  const getNotificationType = () => {
    if (!template || template.value === undefined) {
      return 'due'
    }

    if (template.value < 0) {
      return 'reminder'
    } else if (template.value === 0) {
      return 'due'
    } else {
      return 'overdue'
    }
  }

  const notificationType = getNotificationType()

  // Truncate chore name if too long for better readability
  const maxChoreNameLength = 25
  const truncatedName =
    choreName.length > maxChoreNameLength
      ? `${choreName.substring(0, maxChoreNameLength)}...`
      : choreName

  // Generate time-based descriptive text
  const getTimeDescription = () => {
    if (!template || !template.value || !template.unit) {
      return 'soon'
    }

    const { value, unit } = template
    const absValue = Math.abs(value)

    switch (unit) {
      case 'm':
        if (absValue === 1) return value < 0 ? 'in 1 minute' : '1 minute ago'
        if (absValue < 60)
          return value < 0
            ? `in ${absValue} minutes`
            : `${absValue} minutes ago`
        break
      case 'h':
        if (absValue === 1) return value < 0 ? 'in 1 hour' : '1 hour ago'
        if (absValue < 24)
          return value < 0 ? `in ${absValue} hours` : `${absValue} hours ago`
        break
      case 'd':
        if (absValue === 1) return value < 0 ? 'tomorrow' : 'yesterday'
        if (absValue === 7) return value < 0 ? 'next week' : 'last week'
        if (absValue < 7)
          return value < 0 ? `in ${absValue} days` : `${absValue} days ago`
        if (absValue < 30) {
          const weeks = Math.round(absValue / 7)
          return value < 0 ? `in ${weeks} weeks` : `${weeks} weeks ago`
        }
        break
      default:
        return value < 0 ? `in ${absValue} ${unit}` : `${absValue} ${unit} ago`
    }

    return value < 0 ? `in ${absValue} ${unit}` : `${absValue} ${unit} ago`
  }

  const messages = {
    reminder: {
      title: `📋 ${truncatedName}`,
      body: `Reminder: Due ${getTimeDescription()}`,
    },
    due: {
      title: `🔔 ${truncatedName}`,
      body: 'Due now - Time to get started!',
    },
    overdue: {
      title: `❗ ${truncatedName}`,
      body: `Overdue ${getTimeDescription()} - Complete when you can`,
    },
  }

  // Fallback to due if type not found
  const messageTemplate = messages[notificationType] || messages.due

  return {
    title: messageTemplate.title,
    body: messageTemplate.body,
  }
}
const cancelPendingNotifications = async () => {
  try {
    const pending = await LocalNotifications.getPending()
    if (pending.notifications.length > 0) {
      await LocalNotifications.cancel({ notifications: pending.notifications })
      console.log('Cancelled pending notifications:', pending.notifications)
    } else {
      console.log('No pending notifications to cancel.')
    }
  } catch (error) {
    console.error('Error cancelling pending notifications:', error)
  }
}
const scheduleChoreNotification = async (
  chores,
  userProfile,
  allPerformers,
) => {
  await cancelPendingNotifications()
  const notifications = []

  for (let i = 0; i < chores.length; i++) {
    const chore = chores[i]
    try {
      if (chore.notification === false || chore.nextDueDate === null) {
        continue
      }
      scheduleNotificationFromTemplate(
        chore,
        userProfile,
        allPerformers,
        notifications,
      )
    } catch (error) {
      console.error(
        'Error parsing notification metadata for chore:',
        chore.id,
        error,
      )
      continue
    }
  }

  LocalNotifications.schedule({
    notifications,
  })
  return notifications
}

export { canScheduleNotification, scheduleChoreNotification }
