import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'
import { apiManager, isTokenValid } from '../utils/TokenManager'

const WEBSOCKET_STATES = {
  CONNECTING: 0,
  OPEN: 1,
  CLOSING: 2,
  CLOSED: 3,
}

const RECONNECT_INTERVALS = [1000, 2000, 5000, 10000, 30000] // Progressive backoff

export const useWebSocket = () => {
  const [connectionState, setConnectionState] = useState(
    WEBSOCKET_STATES.CLOSED,
  )
  const [lastEvent, setLastEvent] = useState(null)
  const [error, setError] = useState(null)

  const wsRef = useRef(null)
  const reconnectTimeoutRef = useRef(null)
  const reconnectAttemptsRef = useRef(0)
  const isManuallyClosedRef = useRef(false)

  const queryClient = useQueryClient()

  const getWebSocketUrl = useCallback(() => {
    const token = localStorage.getItem('ca_token')
    if (!token || !isTokenValid()) {
      console.log('WebSocket: No valid authentication token')
      return null
    }

    const apiUrl = apiManager.getApiURL()

    // Convert HTTP/HTTPS to WebSocket protocol and remove /api/v1 suffix
    let wsUrl = apiUrl.replace(/\/api\/v1$/, '')
    if (wsUrl.startsWith('http://')) {
      wsUrl = wsUrl.replace('http://', 'ws://')
    } else if (wsUrl.startsWith('https://')) {
      wsUrl = wsUrl.replace('https://', 'wss://')
    } else {
      const isHttps = window.location.protocol === 'https:'
      wsUrl = `${isHttps ? 'wss:' : 'ws:'}//${wsUrl}`
    }

    // Let backend determine circle from authenticated user
    wsUrl = `${wsUrl}/api/v1/realtime/ws?token=${token}`

    return wsUrl
  }, [])

  const handleWebSocketMessage = useCallback(
    event => {
      try {
        const eventData = JSON.parse(event.data)
        setLastEvent(eventData)

        console.debug('WebSocket event received:', eventData.type, eventData)

        // Handle different event types and update React Query cache accordingly
        switch (eventData.type) {
          case 'chore.created':
          case 'chore.updated':
          case 'chore.completed':
          case 'chore.skipped':
          case 'chore.deleted':
            // Invalidate chores queries to refetch data
            queryClient.invalidateQueries(['chores'])

            // If it's a specific chore event, also invalidate that chore's details
            if (eventData.data.chore?.id) {
              queryClient.invalidateQueries(['chore', eventData.data.chore.id])
              queryClient.invalidateQueries([
                'choreDetails',
                eventData.data.chore.id,
              ])
            }
            // expire the history so feed on dashboard gert updated :
            // need to find a better way to do this as we don't need to do it with every single update for anything
            // but not sure if i can do it with the fall-through switch case in javascript :)
            queryClient.invalidateQueries(['choresHistory', 7])

            break

          case 'subtask.updated':
          case 'subtask.completed':
            // Invalidate the specific chore that contains this subtask
            if (eventData.data.choreId) {
              queryClient.invalidateQueries(['chore', eventData.data.choreId])
              queryClient.invalidateQueries([
                'choreDetails',
                eventData.data.choreId,
              ])
            }
            // Also invalidate general chores list
            queryClient.invalidateQueries(['chores'])
            break

          case 'heartbeat':
            // Heartbeat events don't need cache invalidation
            console.debug('Heartbeat!')
            break

          case 'connection.established':
            console.log('WebSocket connection established')
            setError(null)
            break

          case 'error':
            console.error('WebSocket error event:', eventData.data)
            setError(eventData.data.message || 'WebSocket error occurred')
            break

          default:
            console.log('Unknown WebSocket event type:', eventData.type)
        }
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err)
        setError('Failed to parse server message')
      }
    },
    [queryClient],
  )

  const createWebSocketConnection = useCallback(
    wsUrl => {
      try {
        setConnectionState(WEBSOCKET_STATES.CONNECTING)
        isManuallyClosedRef.current = false

        // Use query parameter authentication (token already included in URL)
        wsRef.current = new WebSocket(wsUrl)

        wsRef.current.onopen = () => {
          setConnectionState(WEBSOCKET_STATES.OPEN)
          setError(null)
          reconnectAttemptsRef.current = 0
        }

        wsRef.current.onmessage = handleWebSocketMessage

        wsRef.current.onerror = error => {
          console.error('WebSocket error:', error)
          setError('Connection error occurred')
        }
      } catch (err) {
        console.error('Failed to create WebSocket connection:', err)
        setError('Failed to establish connection')
        setConnectionState(WEBSOCKET_STATES.CLOSED)
      }
    },
    [handleWebSocketMessage],
  )

  const scheduleReconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }

    const attemptIndex = Math.min(
      reconnectAttemptsRef.current,
      RECONNECT_INTERVALS.length - 1,
    )
    const delay = RECONNECT_INTERVALS[attemptIndex]

    console.log(
      `Scheduling WebSocket reconnect in ${delay}ms (attempt ${reconnectAttemptsRef.current + 1})`,
    )

    reconnectTimeoutRef.current = setTimeout(() => {
      reconnectAttemptsRef.current++
      // Trigger reconnection
      const wsUrl = getWebSocketUrl()
      if (wsUrl && wsRef.current?.readyState !== WEBSOCKET_STATES.OPEN) {
        createWebSocketConnection(wsUrl)
      }
    }, delay)
  }, [getWebSocketUrl, createWebSocketConnection])

  // Set up the onclose handler separately to avoid circular dependency
  useEffect(() => {
    if (wsRef.current) {
      wsRef.current.onclose = event => {
        console.log('WebSocket connection closed:', event.code, event.reason)
        setConnectionState(WEBSOCKET_STATES.CLOSED)

        // Handle different close codes
        if (event.code === 4000) {
          setError('Authentication failed - please refresh the page')
          return // Don't attempt to reconnect for auth failures
        } else if (event.code === 4001) {
          setError('Authorization failed - check circle access')
          return // Don't attempt to reconnect for auth failures
        }

        // Attempt to reconnect if not manually closed
        if (!isManuallyClosedRef.current && event.code !== 1000) {
          scheduleReconnect()
        }
      }
    }
  }, [scheduleReconnect])

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WEBSOCKET_STATES.OPEN) {
      console.log('WebSocket: Already connected')
      return // Already connected
    }

    const wsUrl = getWebSocketUrl()
    console.log('WebSocket connect - URL:', wsUrl)

    if (!wsUrl) {
      console.log(
        'Cannot connect to WebSocket: missing URL, token, or user profile',
      )
      return
    }

    createWebSocketConnection(wsUrl)
  }, [getWebSocketUrl, createWebSocketConnection])

  const disconnect = useCallback(() => {
    isManuallyClosedRef.current = true

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }

    if (wsRef.current) {
      wsRef.current.close(1000, 'Manual disconnect')
      wsRef.current = null
    }

    setConnectionState(WEBSOCKET_STATES.CLOSED)
  }, [])

  const toggleWebSocketEnabled = useCallback(
    enabled => {
      localStorage.setItem('websocket_enabled', enabled.toString())
      if (enabled && isTokenValid()) {
        connect()
      } else {
        disconnect()
      }
    },
    [connect, disconnect],
  )

  const isWebSocketEnabled = useCallback(() => {
    return localStorage.getItem('websocket_enabled') !== 'false'
  }, [])

  // Auto-connect when WebSocket is enabled and token is valid
  useEffect(() => {
    // Check if WebSocket is enabled in settings
    const isWebSocketEnabledSetting =
      localStorage.getItem('websocket_enabled') !== 'false'
    console.log('WebSocket enabled in settings:', isWebSocketEnabledSetting)

    if (isTokenValid() && isWebSocketEnabledSetting) {
      console.log('WebSocket: Conditions met, attempting to connect')
      connect()
    } else {
      console.log('WebSocket: Conditions not met, disconnecting')
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
    }
  }, [])

  return {
    connectionState,
    isConnected: connectionState === WEBSOCKET_STATES.OPEN,
    isConnecting: connectionState === WEBSOCKET_STATES.CONNECTING,
    lastEvent,
    error,
    connect,
    disconnect,
    toggleWebSocketEnabled,
    isWebSocketEnabled,
    // Helper function to check connection status
    getConnectionStatus: () => {
      switch (connectionState) {
        case WEBSOCKET_STATES.CONNECTING:
          return 'connecting'
        case WEBSOCKET_STATES.OPEN:
          return 'connected'
        case WEBSOCKET_STATES.CLOSING:
          return 'disconnecting'
        case WEBSOCKET_STATES.CLOSED:
        default:
          return 'disconnected'
      }
    },
  }
}
