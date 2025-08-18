import { useQueryClient } from '@tanstack/react-query'
import { EventSourcePolyfill } from 'event-source-polyfill'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useUserProfile } from '../queries/UserQueries'
import { useAlerts } from '../service/AlertsProvider'
import { useNotification } from '../service/NotificationProvider'
import { apiManager, isTokenValid } from '../utils/TokenManager'
const SSE_STATES = {
  CONNECTING: 0,
  OPEN: 1,
  CLOSED: 2,
}

const RECONNECT_INTERVALS = [2000, 5000, 10000, 30000, 360000, 600000, 900000] //  2s, 5s, 10s, 30s, 6m, 10m, 15m
const MAX_RECONNECT_ATTEMPTS = 10 // Circuit breaker limit
const CIRCUIT_BREAKER_RESET_TIME = 600000 // 10 minutes

export const useSSE = () => {
  const { data: userProfile } = useUserProfile()
  const [connectionState, setConnectionState] = useState(SSE_STATES.CLOSED)
  const [lastEvent, setLastEvent] = useState(null)
  const [error, setError] = useState(null)
  const [isCircuitBreakerOpen, setIsCircuitBreakerOpen] = useState(false)

  const eventSourceRef = useRef(null)
  const reconnectTimeoutRef = useRef(null)
  const reconnectAttemptsRef = useRef(0)
  const isManuallyClosedRef = useRef(false)
  const lastHeartbeatRef = useRef(Date.now())
  const heartbeatMonitorRef = useRef(null)

  const queryClient = useQueryClient()
  const { showError, showNotification } = useNotification()
  const { showAlert } = useAlerts()

  const getSSEUrl = useCallback(() => {
    const token = localStorage.getItem('ca_token')
    if (!token || !isTokenValid()) {
      console.log('SSE: No valid authentication token')
      return null
    }

    // Get the API URL from apiManager
    const apiUrl = apiManager.getApiURL() // e.g., "http://localhost:8080/api/v1"

    // Build SSE URL - let backend determine circle from authenticated user
    const sseUrl = `${apiUrl}/realtime/sse`

    return { url: sseUrl, token }
  }, [])

  const handleSSEMessage = useCallback(
    event => {
      try {
        const eventData = JSON.parse(event.data)
        setLastEvent(eventData)

        // Update heartbeat timestamp
        if (eventData.type === 'heartbeat') {
          lastHeartbeatRef.current = Date.now()
        }
        console.log('SSE Message received:', eventData)

        // Handle different event types and update React Query cache accordingly
        switch (eventData.type) {
          case 'chore.created':
          case 'chore.updated':
          case 'chore.completed':
          case 'chore.skipped': {
            if (eventData?.data?.user?.id !== userProfile?.id) {
              showNotification({
                type: 'info',
                title: `Task ${eventData.type.replace('chore.', '')}`,
                message: `${eventData.data.user.displayName}  ${eventData.type.replace('chore.', '')} "${eventData.data.chore.name}"`,
                duration: 5000,
              })
            }
            const updatedChore = eventData.data.chore

            // Update individual chore cache
            queryClient.setQueryData(['chore', updatedChore.id], oldData => {
              if (!oldData) return { res: updatedChore }
              return { res: { ...oldData.res, ...updatedChore } }
            })

            // Update chores list cache - add debugging
            queryClient.setQueryData(['chores'], oldData => {
              if (!oldData) return { res: [updatedChore] }

              if (!oldData.res || !Array.isArray(oldData.res)) {
                return { res: [updatedChore] }
              }

              // Check if the chore exists in the cache
              const choreExists = oldData.res.some(
                chore => chore.id === updatedChore.id,
              )

              // If it's a one-time chore that's completed, we might need to remove it
              if (
                eventData.type === 'chore.completed' &&
                updatedChore.frequencyType === 'once'
              ) {
                return {
                  res: oldData.res.filter(
                    chore => chore.id !== updatedChore.id,
                  ),
                }
              }

              // If chore update then also refetch chore details:
              if (eventData.type === 'chore.updated') {
                queryClient.invalidateQueries(['choreDetails', updatedChore.id])
                queryClient.refetchQueries({
                  queryKey: ['choreDetails', updatedChore.id],
                })
              }

              // Otherwise update the existing chore or add if it doesn't exist
              return {
                res: choreExists
                  ? oldData.res.map(chore => {
                      if (chore.id === updatedChore.id) {
                        return { ...chore, ...updatedChore }
                      }
                      return chore
                    })
                  : [...oldData.res, updatedChore],
              }
            })

            break
          }

          case 'chore.deleted':
            // update chores list cache
            queryClient.setQueryData(['chores'], oldData => {
              if (!oldData || !oldData.res) return oldData
              return {
                res: oldData.res.filter(
                  chore => chore.id !== eventData.data.choreId,
                ),
              }
            })

            break

          case 'subtask.updated':
          case 'subtask.completed':
            queryClient.refetchQueries({
              queryKey: ['choreDetails', eventData.data.choreId],
            })

            // Invalidate the specific chore that contains this subtask
            // if (eventData.data.choreId) {
            //   queryClient.invalidateQueries(['chore', eventData.data.choreId])
            //   queryClient.invalidateQueries([
            //     'choreDetails',
            //     eventData.data.choreId,
            //   ])
            // }
            // Also invalidate general chores list
            // queryClient.invalidateQueries(['chores'])
            break
          case 'chore.status':
            console.log('SSE chore.status event received:', eventData.data)

            break
          case 'heartbeat':
            // Heartbeat events don't need cache invalidation
            console.debug('SSE Heartbeat received at', new Date().toISOString())
            break

          case 'connection.established':
            console.log('SSE connection established')
            setError(null)
            lastHeartbeatRef.current = Date.now()
            showAlert({
              type: 'success',
              color: 'success',
              message: 'You are now receiving real-time as they happen.',
            })
            break

          case 'error':
            console.error('SSE error event:', eventData.data)
            showError({
              title: 'Real-time Error',
              message:
                eventData.data.message ||
                'An error occurred with real-time updates',
            })
            break

          default:
            console.log('Unknown SSE event type:', eventData.type)
        }
      } catch (err) {
        console.error('Failed to parse SSE message:', err)
        showError({
          title: 'Message Error',
          message: 'Failed to parse server message',
        })
        return // Stop processing if JSON parsing fails
      }
    },
    [queryClient, showNotification, showError],
  )

  const stopHeartbeatMonitor = useCallback(() => {
    if (heartbeatMonitorRef.current) {
      clearInterval(heartbeatMonitorRef.current)
      heartbeatMonitorRef.current = null
    }
  }, [])

  // Create connect function that can be called from anywhere
  const connect = useCallback(() => {
    if (isCircuitBreakerOpen) {
      console.log('SSE: Circuit breaker is open, preventing connection attempt')
      showError({
        title: 'Connection Temporarily Disabled',
        message:
          'Connection blocked due to repeated failures. Please try again later.',
      })
      return
    }

    if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
      console.error(
        'SSE: Maximum reconnection attempts reached, opening circuit breaker',
      )
      setIsCircuitBreakerOpen(true)
      showError({
        title: 'Connection Failed',
        message:
          'Maximum connection attempts reached. SSE disabled for 10 minutes.',
      })

      // Reset circuit breaker after timeout
      setTimeout(() => {
        console.log('SSE: Resetting circuit breaker')
        setIsCircuitBreakerOpen(false)
        reconnectAttemptsRef.current = 0
      }, CIRCUIT_BREAKER_RESET_TIME)
      return
    }

    // Prevent race conditions by checking if already connecting or connected
    if (eventSourceRef.current?.readyState === SSE_STATES.OPEN) {
      console.log('SSE: Already connected')
      return // Already connected
    }

    if (eventSourceRef.current?.readyState === SSE_STATES.CONNECTING) {
      console.log('SSE: Connection already in progress')
      return // Already connecting
    }

    const sseConfig = getSSEUrl()
    console.log('SSE connect - Config:', sseConfig)

    if (!sseConfig) {
      console.log('Cannot connect to SSE: missing URL, token, or user profile')
      return
    }

    // Create connection logic inline to avoid circular dependency
    try {
      console.log('Connecting to SSE:', sseConfig.url)
      setConnectionState(SSE_STATES.CONNECTING)
      isManuallyClosedRef.current = false

      // here use EventSource polyfill with Authorization header as the native EventSource does not support headers
      // the other option was to pass via query param which is less secure and also there.
      // TODO: use cookie-based once/if at all i move from local storage to httpOnly cookies.
      eventSourceRef.current = new EventSourcePolyfill(sseConfig.url, {
        headers: {
          Authorization: `Bearer ${sseConfig.token}`,
          'Cache-Control': 'no-cache',
          Accept: 'text/event-stream',
        },
        // Increase timeout to prevent premature disconnections
        // Default is 45000ms (45s), increasing to 2 minutes
        // TODO: send this in the resource object so it can be configured per instance
        heartbeatTimeout: 120000,
        // Enable silentTimeoutRetry to handle temporary network issues
        silentTimeoutRetry: true,
      })

      eventSourceRef.current.onopen = () => {
        console.log('SSE connection opened')
        setConnectionState(SSE_STATES.OPEN)
        setError(null)
        reconnectAttemptsRef.current = 0
        lastHeartbeatRef.current = Date.now()

        // Start heartbeat monitor
        if (heartbeatMonitorRef.current) {
          clearInterval(heartbeatMonitorRef.current)
        }
        heartbeatMonitorRef.current = setInterval(() => {
          const timeSinceLastHeartbeat = Date.now() - lastHeartbeatRef.current
          const heartbeatTimeout = 150000 // 2.5 minutes - should be longer than server heartbeat interval

          if (timeSinceLastHeartbeat > heartbeatTimeout) {
            console.warn(
              `SSE: No heartbeat received for ${Math.round(timeSinceLastHeartbeat / 1000)}s, connection may be stale. Reconnecting...`,
            )
            if (!isManuallyClosedRef.current) {
              // Clear current heartbeat monitor before reconnecting
              stopHeartbeatMonitor()

              // Close current connection gracefully
              if (eventSourceRef.current) {
                eventSourceRef.current.close()
                eventSourceRef.current = null
              }
              setConnectionState(SSE_STATES.CLOSED)

              // Schedule reconnect
              if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current)
              }

              const attemptIndex = Math.min(
                reconnectAttemptsRef.current,
                RECONNECT_INTERVALS.length - 1,
              )
              const delay = RECONNECT_INTERVALS[attemptIndex]

              console.log(
                `Scheduling SSE reconnect in ${delay}ms (attempt ${
                  reconnectAttemptsRef.current + 1
                })`,
              )

              reconnectTimeoutRef.current = setTimeout(() => {
                reconnectAttemptsRef.current++
                connect()
              }, delay)
            }
          }
        }, 60000) // Check every minute
      }

      eventSourceRef.current.onmessage = handleSSEMessage

      eventSourceRef.current.onerror = error => {
        console.error('SSE error:', error)
        setConnectionState(SSE_STATES.CLOSED)
        stopHeartbeatMonitor()

        if (!isManuallyClosedRef.current) {
          // Check if this is a timeout error specifically
          const isTimeoutError =
            error.error?.message?.includes('No activity within') ||
            error.error?.message?.includes('timeout')

          if (isTimeoutError) {
            console.log('SSE timeout detected, attempting reconnection...')
            setError('Connection timeout - reconnecting...')
          } else {
            setError('Connection error occurred')
          }

          // Schedule reconnect
          if (reconnectTimeoutRef.current) {
            clearTimeout(reconnectTimeoutRef.current)
          }

          const attemptIndex = Math.min(
            reconnectAttemptsRef.current,
            RECONNECT_INTERVALS.length - 1,
          )
          const delay = RECONNECT_INTERVALS[attemptIndex]

          console.log(
            `Scheduling SSE reconnect in ${delay}ms (attempt ${
              reconnectAttemptsRef.current + 1
            })`,
          )

          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttemptsRef.current++
            connect()
          }, delay)
        }
      }
    } catch (err) {
      console.error('Failed to create SSE connection:', err)
      showError({
        title: 'Connection Error',
        message: 'Failed to establish real-time connection. Please try again.',
      })
      setConnectionState(SSE_STATES.CLOSED)
    }
  }, [
    getSSEUrl,
    handleSSEMessage,
    stopHeartbeatMonitor,
    isCircuitBreakerOpen,
    showError,
  ])

  const disconnect = useCallback(() => {
    isManuallyClosedRef.current = true

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }

    stopHeartbeatMonitor()

    if (eventSourceRef.current) {
      eventSourceRef.current.close()
      eventSourceRef.current = null
    }

    setConnectionState(SSE_STATES.CLOSED)
  }, [stopHeartbeatMonitor])

  const toggleSSEEnabled = useCallback(
    enabled => {
      console.log('SSE toggleSSEEnabled called:', {
        enabled,
        isTokenValid: isTokenValid(),
      })
      localStorage.setItem('sse_enabled', enabled.toString())
      if (enabled && isTokenValid()) {
        console.log('SSE toggleSSEEnabled: Calling connect()')
        connect()
      } else {
        console.log('SSE toggleSSEEnabled: Calling disconnect()')
        disconnect()
      }
    },
    [connect, disconnect],
  )

  const isSSEEnabled = useCallback(() => {
    return localStorage.getItem('sse_enabled') === 'true'
  }, [])

  // Auto-connect when SSE is enabled and token is valid
  useEffect(() => {
    console.log('SSE auto-connect effect triggered')
    console.log('Token valid:', isTokenValid())

    // Check if SSE is enabled in settings
    const isSSEEnabledSetting = localStorage.getItem('sse_enabled') === 'true'
    console.log('SSE enabled in settings:', isSSEEnabledSetting)

    if (isTokenValid() && isSSEEnabledSetting) {
      console.log('SSE: Conditions met, attempting to connect')
      connect()
    } else {
      console.log('SSE: Conditions not met, disconnecting')
      disconnect()
    }

    // Cleanup on unmount
    return () => {
      disconnect()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // Only run once on mount

  // Cleanup timeouts on unmount
  useEffect(() => {
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      stopHeartbeatMonitor()
    }
  }, [stopHeartbeatMonitor])

  // Handle visibility changes for better performance
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.hidden) {
        // App went to background, maintain connection but log the state
        console.log(
          'SSE: App backgrounded, maintaining connection but reducing activity',
        )
      } else {
        // App came to foreground, ensure connection is active
        console.log('SSE: App foregrounded, ensuring connection is active')

        const isSSEEnabledSetting =
          localStorage.getItem('sse_enabled') === 'true'
        if (
          isTokenValid() &&
          isSSEEnabledSetting &&
          connectionState !== SSE_STATES.OPEN
        ) {
          connect()
        }
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }, [connectionState, connect])

  return {
    connectionState,
    isConnected: connectionState === SSE_STATES.OPEN,
    isConnecting: connectionState === SSE_STATES.CONNECTING,
    lastEvent,
    error,
    connect,
    disconnect,
    toggleSSEEnabled,
    isSSEEnabled,
    // Helper function to check connection status
    getConnectionStatus: () => {
      switch (connectionState) {
        case SSE_STATES.CONNECTING:
          return 'connecting'
        case SSE_STATES.OPEN:
          return 'connected'
        case SSE_STATES.CLOSED:
        default:
          return 'disconnected'
      }
    },
    // Additional debugging information
    getDebugInfo: () => ({
      connectionState,
      reconnectAttempts: reconnectAttemptsRef.current,
      isCircuitBreakerOpen,
      lastHeartbeat: lastHeartbeatRef.current,
      timeSinceLastHeartbeat: Date.now() - lastHeartbeatRef.current,
      isManuallyCloseRef: isManuallyClosedRef.current,
    }),
  }
}
