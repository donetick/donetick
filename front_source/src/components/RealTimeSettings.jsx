import { Sync, SyncDisabled } from '@mui/icons-material'
import { Box, Card, Chip, FormHelperText, Switch, Typography } from '@mui/joy'
import { useState } from 'react'
import { useSSEContext } from '../hooks/useSSEContext'
import { useUserProfile } from '../queries/UserQueries'
import { isPlusAccount } from '../utils/Helpers'
import SSEConnectionStatus from './SSEConnectionStatus'

const REALTIME_TYPES = {
  DISABLED: 'disabled',
  SSE: 'sse',
}

const RealTimeSettings = () => {
  const { data: userProfile } = useUserProfile()

  // SSE context
  const sseContext = useSSEContext()

  // Get current realtime type from localStorage
  const getCurrentRealtimeType = () => {
    const sseEnabled = localStorage.getItem('sse_enabled') === 'true'
    return sseEnabled ? REALTIME_TYPES.SSE : REALTIME_TYPES.DISABLED
  }

  const [realtimeType, setRealtimeType] = useState(getCurrentRealtimeType())

  const handleRealtimeTypeChange = (event, newValue) => {
    if (!isPlusAccount(userProfile)) {
      return // Don't allow changes for non-Plus users
    }

    setRealtimeType(newValue)

    // Update localStorage and toggle connections
    switch (newValue) {
      case REALTIME_TYPES.DISABLED:
        localStorage.setItem('sse_enabled', 'false')
        sseContext.disconnect()
        break
      case REALTIME_TYPES.SSE:
        localStorage.setItem('sse_enabled', 'true')
        sseContext.connect()
        break
    }
  }

  const getCurrentContext = () => {
    switch (realtimeType) {
      case REALTIME_TYPES.SSE:
        return sseContext
      default:
        return {
          isConnected: false,
          isConnecting: false,
          error: null,
          getConnectionStatus: () => 'disabled',
        }
    }
  }

  const context = getCurrentContext()

  const getStatusDescription = () => {
    if (!isPlusAccount(userProfile)) {
      return 'Real-time updates are not available in the Basic plan. Upgrade to Plus to receive instant notifications when tasks are updated.'
    }

    if (realtimeType === REALTIME_TYPES.DISABLED) {
      return 'Real-time updates are disabled. Enable them to see live changes when you or other circle members complete, skip, or modify tasks.'
    }

    if (context.isConnected) {
      return "Real-time updates are working. You'll see live changes when you or other circle members complete, skip, or modify tasks."
    }

    if (context.isConnecting) {
      return 'Connecting to real-time updates...'
    }

    if (context.error) {
      return `Real-time updates are enabled but not working: ${context.error}`
    }

    return 'Real-time updates are enabled but not currently connected.'
  }

  const getConnectionStatusComponent = () => {
    switch (realtimeType) {
      case REALTIME_TYPES.SSE:
        return <SSEConnectionStatus variant='chip' />
      default:
        return null
    }
  }

  return (
    <Card sx={{ mt: 2, p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2, mb: 2 }}>
        <Switch
          checked={realtimeType !== REALTIME_TYPES.DISABLED}
          onChange={e => {
            handleRealtimeTypeChange(
              null,
              e.target.checked ? REALTIME_TYPES.SSE : REALTIME_TYPES.DISABLED,
            )
          }}
          color={
            realtimeType !== REALTIME_TYPES.DISABLED ? 'success' : 'neutral'
          }
          disabled={!isPlusAccount(userProfile)}
          inputProps={{ 'aria-label': 'Enable Real-time Updates' }}
        />
        <Box sx={{ flex: 1 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              mb: 0.5,
            }}
          >
            <Typography level='title-md'>
              Real-time Updates
              {!isPlusAccount(userProfile) && (
                <Chip variant='soft' color='warning' sx={{ ml: 1 }}>
                  Plus Feature
                </Chip>
              )}
            </Typography>

            {realtimeType !== REALTIME_TYPES.DISABLED &&
            isPlusAccount(userProfile) ? (
              <Sync color={context.isConnected ? 'success' : 'disabled'} />
            ) : (
              <SyncDisabled color='disabled' />
            )}
          </Box>
          <Typography level='body-sm' color='neutral'>
            Get instant notifications when tasks are updated
          </Typography>
        </Box>
      </Box>

      {/* <FormControl orientation='horizontal' sx={{ mb: 2 }}>
        <Box sx={{ flex: 1 }}>
          <FormLabel>Real-time Connection Type</FormLabel>
          <FormHelperText sx={{ mt: 0 }}>
            Choose how to receive real-time updates
          </FormHelperText>
        </Box>
        <Select
          value={realtimeType}
          onChange={handleRealtimeTypeChange}
          disabled={!isPlusAccount(userProfile)}
          sx={{ minWidth: 140 }}
        >
          <Option value={REALTIME_TYPES.DISABLED}>Disabled</Option>
          <Option value={REALTIME_TYPES.WEBSOCKET}>WebSocket</Option>
          <Option value={REALTIME_TYPES.SSE}>SSE</Option>
        </Select>
      </FormControl> */}

      <FormHelperText sx={{ mb: 2 }}>{getStatusDescription()}</FormHelperText>

      {realtimeType !== REALTIME_TYPES.DISABLED &&
        isPlusAccount(userProfile) && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 1 }}>
            <Typography level='body-xs' color='neutral'>
              Status:
            </Typography>
            {getConnectionStatusComponent()}
            {context.error && (
              <Typography level='body-xs' color='danger'>
                {context.error}
              </Typography>
            )}
          </Box>
        )}

      {!isPlusAccount(userProfile) && (
        <Typography level='body-sm' color='warning' sx={{ mt: 1 }}>
          Real-time updates are not available in the Basic plan. Upgrade to Plus
          to receive instant notifications when you or other circle members
          complete, skip, or modify tasks.
        </Typography>
      )}
    </Card>
  )
}

export default RealTimeSettings
