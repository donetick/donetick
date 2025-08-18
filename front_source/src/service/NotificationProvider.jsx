import { CheckCircle, Error, Info, Warning } from '@mui/icons-material'
import { Box, Button, Snackbar, Typography } from '@mui/joy'
import React, { createContext, useContext, useState } from 'react'

const NotificationContext = createContext()

export const useNotification = () => useContext(NotificationContext)

// For backward compatibility
export const useError = () => {
  const { showError } = useNotification()
  return { showError }
}

// Notification types configuration with default titles
const NOTIFICATION_TYPES = {
  error: {
    color: 'danger',
    icon: <Error color='danger' />,
    autoHideDuration: 6000,
    showDismissButton: true,
    defaultTitle: 'Error',
  },
  success: {
    color: 'success',
    icon: <CheckCircle color='success' />,
    autoHideDuration: 3000,
    showDismissButton: false,
    defaultTitle: 'Success',
  },
  warning: {
    color: 'warning',
    icon: <Warning color='warning' />,
    autoHideDuration: 4000,
    showDismissButton: false,
    defaultTitle: 'Warning',
  },
  info: {
    color: 'primary',
    icon: <Info color='primary' />,
    autoHideDuration: 4000,
    showDismissButton: false,
    defaultTitle: 'Information',
  },
  custom: {
    color: 'neutral',
    icon: null,
    autoHideDuration: null,
    showDismissButton: false,
    defaultTitle: 'Notification',
  },
}

export const NotificationProvider = ({ children }) => {
  const [notifications, setNotifications] = useState([])

  const addNotification = notification => {
    const id = Date.now() + Math.random()
    const newNotification = {
      id,
      ...notification,
      timestamp: Date.now(),
    }

    setNotifications(prev => [...prev, newNotification])

    // Auto-remove notification if it has a duration
    const config =
      NOTIFICATION_TYPES[notification.type] || NOTIFICATION_TYPES.info
    if (config.autoHideDuration) {
      setTimeout(() => {
        removeNotification(id)
      }, config.autoHideDuration)
    }

    return id
  }

  const removeNotification = id => {
    setNotifications(prev => prev.filter(n => n.id !== id))
  }

  const clearAllNotifications = () => {
    setNotifications([])
  }

  // Helper function to normalize notification input
  const normalizeNotification = (input, type) => {
    if (typeof input === 'string') {
      return {
        type,
        message: input,
      }
    }

    if (typeof input === 'object' && input !== null) {
      // If it's already a properly structured notification
      if (input.title || input.message) {
        return {
          type,
          ...input,
        }
      }

      // If it's a simple object with just message content
      return {
        type,
        message: input.message || input.toString(),
        title: input.title,
        ...input,
      }
    }

    return {
      type,
      message: input?.toString() || 'Unknown notification',
    }
  }

  // Unified notification method
  const showNotification = notification => {
    // Handle different input formats
    if (typeof notification === 'string') {
      return addNotification(normalizeNotification(notification, 'info'))
    }

    return addNotification(
      normalizeNotification(notification, notification.type || 'info'),
    )
  }

  // Specific notification methods with enhanced language
  const showError = error => {
    return addNotification(normalizeNotification(error, 'error'))
  }

  const showSuccess = message => {
    return addNotification(normalizeNotification(message, 'success'))
  }

  const showWarning = message => {
    return addNotification(normalizeNotification(message, 'warning'))
  }

  const showInfo = message => {
    return addNotification(normalizeNotification(message, 'info'))
  }

  const renderNotification = notification => {
    const config =
      NOTIFICATION_TYPES[notification.type] || NOTIFICATION_TYPES.info

    // Handle custom notifications with components
    if (notification.type === 'custom' && notification.component) {
      return (
        <Snackbar
          key={notification.id}
          open={true}
          onClose={() => removeNotification(notification.id)}
          anchorOrigin={
            notification.anchorOrigin || {
              vertical: 'bottom',
              horizontal: 'right',
            }
          }
          {...(notification.snackbarProps || {})}
        >
          {React.cloneElement(notification.component, {
            onClose: () => removeNotification(notification.id),
            ...notification.componentProps,
          })}
        </Snackbar>
      )
    }

    // Handle standard notifications
    // Determine the icon to use
    const notificationIcon = notification.icon || config.icon

    // Determine title and message
    const title = notification.title || config.defaultTitle
    const message = notification.message

    return (
      <Snackbar
        key={notification.id}
        open={true}
        autoHideDuration={config.autoHideDuration}
        onClose={() => removeNotification(notification.id)}
        startDecorator={notificationIcon}
        endDecorator={
          config.showDismissButton ? (
            <Button
              variant='outlined'
              color={config.color}
              onClick={() => removeNotification(notification.id)}
            >
              Dismiss
            </Button>
          ) : null
        }
        anchorOrigin={
          notification.anchorOrigin || {
            vertical: 'bottom',
            horizontal: 'right',
          }
        }
        {...(notification.snackbarProps || {})}
      >
        {/* Enhanced structure like ErrorProvider - always show title and message for consistency */}
        {title && message ? (
          <Box>
            <Typography color={config.color} level='title-sm'>
              {title}
            </Typography>
            <Typography color={config.color} level='body-sm'>
              {message}
            </Typography>
          </Box>
        ) : (
          <Typography color={config.color} level='body-md'>
            {message || title || 'Notification'}
          </Typography>
        )}
      </Snackbar>
    )
  }

  return (
    <NotificationContext.Provider
      value={{
        showNotification,
        showError,
        showSuccess,
        showWarning,
        showInfo,
        removeNotification,
        clearAllNotifications,
        notifications,
      }}
    >
      {children}
      {notifications.map(renderNotification)}
    </NotificationContext.Provider>
  )
}
