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
import { useSSEContext } from '../hooks/useSSEContext'
import { useUserProfile } from '../queries/UserQueries'
import { isPlusAccount } from '../utils/Helpers'
import SSEConnectionStatus from './SSEConnectionStatus'

const SSESettings = () => {
  const { data: userProfile } = useUserProfile()
  const {
    isConnected,
    isConnecting,
    error,
    getConnectionStatus,
    toggleSSEEnabled,
    isSSEEnabled,
  } = useSSEContext()

  const handleToggle = () => {
    console.log('=== TOGGLE CLICKED ===')
    if (!isPlusAccount(userProfile)) {
      console.log('Not a Plus account, returning early')
      return // Don't allow toggle for non-Plus users
    }
    const currentlyEnabled = isSSEEnabled()
    console.log('SSE Settings - Toggle clicked:', {
      currentlyEnabled,
      newState: !currentlyEnabled,
      userProfile,
      isPlusAccount: isPlusAccount(userProfile),
    })
    toggleSSEEnabled(!currentlyEnabled)
  }

  const getStatusDescription = () => {
    if (!isPlusAccount(userProfile)) {
      return 'Real-time updates (SSE) are not available in the Basic plan. Upgrade to Plus to receive instant notifications when chores are updated.'
    }

    if (!isSSEEnabled()) {
      return 'Real-time updates (SSE) are disabled. Enable to see live changes when you or other circle members complete, skip, or modify chores.'
    }

    if (isConnected) {
      return "Real-time updates (SSE) are working. You'll see live changes when you or other circle members complete, skip, or modify chores."
    }

    if (isConnecting) {
      return 'Connecting to real-time updates (SSE)...'
    }

    if (error) {
      return `Real-time updates (SSE) are enabled but not working: ${error}`
    }

    return 'Real-time updates (SSE) are enabled but not currently connected.'
  }

  return (
    <Card sx={{ mt: 2, p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        {isSSEEnabled() && isPlusAccount(userProfile) ? (
          <Sync color={isConnected ? 'success' : 'disabled'} />
        ) : (
          <SyncDisabled color='disabled' />
        )}
        <Box sx={{ flex: 1 }}>
          <Typography level='title-md'>
            Real-time Updates (SSE)
            {!isPlusAccount(userProfile) && (
              <Chip variant='soft' color='warning' sx={{ ml: 1 }}>
                Plus Feature
              </Chip>
            )}
          </Typography>
          <Typography level='body-sm' color='neutral'>
            Get instant notifications via Server-Sent Events
          </Typography>
        </Box>
        {isSSEEnabled() && isPlusAccount(userProfile) && (
          <SSEConnectionStatus variant='chip' />
        )}
      </Box>

      <FormControl orientation='horizontal' sx={{ mb: 2 }}>
        <Box sx={{ flex: 1 }}>
          <FormLabel>Enable Real-time Updates (SSE)</FormLabel>
          <FormHelperText sx={{ mt: 0 }}>
            {getStatusDescription()}
          </FormHelperText>
        </Box>
        <Switch
          checked={isSSEEnabled() && isPlusAccount(userProfile)}
          onChange={handleToggle}
          disabled={!isPlusAccount(userProfile)}
          color={
            isSSEEnabled() && isPlusAccount(userProfile) ? 'success' : 'neutral'
          }
          variant='solid'
          endDecorator={
            isSSEEnabled() && isPlusAccount(userProfile) ? 'On' : 'Off'
          }
          slotProps={{ endDecorator: { sx: { minWidth: 24 } } }}
        />
      </FormControl>

      {isSSEEnabled() && isPlusAccount(userProfile) && (
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
          Real-time updates (SSE) are not available in the Basic plan. Upgrade
          to Plus to receive instant notifications when you or other circle
          members complete, skip, or modify chores.
        </Typography>
      )}
    </Card>
  )
}

export default SSESettings
