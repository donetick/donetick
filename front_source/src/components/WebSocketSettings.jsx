import { Sync, SyncDisabled } from '@mui/icons-material'
import {
  Box,
  Card,
  Chip,
  FormControl,
  FormHelperText,
  FormLabel,
  Switch,
  Typography,
} from '@mui/joy'
import { useWebSocketContext } from '../contexts/WebSocketContext'
import { useUserProfile } from '../queries/UserQueries'
import { isPlusAccount } from '../utils/Helpers'
import WebSocketConnectionStatus from './WebSocketConnectionStatus'

const WebSocketSettings = () => {
  const { data: userProfile } = useUserProfile()
  const {
    isConnected,
    isConnecting,
    error,
    getConnectionStatus,
    toggleWebSocketEnabled,
    isWebSocketEnabled,
  } = useWebSocketContext()

  const handleToggle = () => {
    if (!isPlusAccount(userProfile)) {
      return // Don't allow toggle for non-Plus users
    }
    const currentlyEnabled = isWebSocketEnabled()
    toggleWebSocketEnabled(!currentlyEnabled)
  }

  const getStatusDescription = () => {
    if (!isPlusAccount(userProfile)) {
      return 'Real-time updates are not available in the Basic plan. Upgrade to Plus to receive instant notifications when chores are updated.'
    }

    if (!isWebSocketEnabled()) {
      return 'Real-time updates are disabled. Enable to see live changes when you or other circle members complete, skip, or modify chores.'
    }

    if (isConnected) {
      return "Real-time updates are working. You'll see live changes when you or other circle members complete, skip, or modify chores."
    }

    if (isConnecting) {
      return 'Connecting to real-time updates...'
    }

    if (error) {
      return `Real-time updates are enabled but not working: ${error}`
    }

    return 'Real-time updates are enabled but not currently connected.'
  }

  return (
    <Card sx={{ mt: 2, p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        {isWebSocketEnabled() && isPlusAccount(userProfile) ? (
          <Sync color={isConnected ? 'success' : 'disabled'} />
        ) : (
          <SyncDisabled color='disabled' />
        )}
        <Box sx={{ flex: 1 }}>
          <Typography level='title-md'>
            Real-time Updates
            {!isPlusAccount(userProfile) && (
              <Chip variant='soft' color='warning' sx={{ ml: 1 }}>
                Plus Feature
              </Chip>
            )}
          </Typography>
          <Typography level='body-sm' color='neutral'>
            Get instant notifications when chores are updated
          </Typography>
        </Box>
        {isWebSocketEnabled() && isPlusAccount(userProfile) && (
          <WebSocketConnectionStatus variant='chip' />
        )}
      </Box>

      <FormControl orientation='horizontal' sx={{ mb: 2 }}>
        <Box sx={{ flex: 1 }}>
          <FormLabel>Enable Real-time Updates</FormLabel>
          <FormHelperText sx={{ mt: 0 }}>
            {getStatusDescription()}
          </FormHelperText>
        </Box>
        <Switch
          checked={Boolean(isWebSocketEnabled() && isPlusAccount(userProfile))}
          onChange={handleToggle}
          disabled={!isPlusAccount(userProfile)}
          color={
            isWebSocketEnabled() && isPlusAccount(userProfile)
              ? 'success'
              : 'neutral'
          }
          variant='solid'
          endDecorator={
            isWebSocketEnabled() && isPlusAccount(userProfile) ? 'On' : 'Off'
          }
          slotProps={{ endDecorator: { sx: { minWidth: 24 } } }}
        />
      </FormControl>

      {isWebSocketEnabled() && isPlusAccount(userProfile) && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 1 }}>
          <Typography level='body-xs' color='neutral'>
            Status:
          </Typography>
          <Chip
            size='sm'
            variant='soft'
            color={
              isConnected ? 'success' : isConnecting ? 'warning' : 'danger'
            }
          >
            {getConnectionStatus()}
          </Chip>
          {error && (
            <Typography level='body-xs' color='danger'>
              {error}
            </Typography>
          )}
        </Box>
      )}

      {!isPlusAccount(userProfile) && (
        <Typography level='body-sm' color='warning' sx={{ mt: 1 }}>
          Real-time updates are not available in the Basic plan. Upgrade to Plus
          to receive instant notifications when you or other circle members
          complete, skip, or modify chores.
        </Typography>
      )}
    </Card>
  )
}

export default WebSocketSettings
