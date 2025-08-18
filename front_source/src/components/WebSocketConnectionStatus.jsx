import { Circle, SignalWifi4Bar, SignalWifiOff } from '@mui/icons-material'
import { Box, Chip, Tooltip, Typography } from '@mui/joy'
import { useWebSocketContext } from '../contexts/WebSocketContext'

const WebSocketConnectionStatus = ({
  variant = 'minimal',
  showError = false,
  sx = {},
}) => {
  const { isConnected, isConnecting, error, getConnectionStatus } =
    useWebSocketContext()

  const getStatusColor = () => {
    if (isConnected) return 'success'
    if (isConnecting) return 'warning'
    return 'danger'
  }

  const getStatusIcon = () => {
    if (isConnected) return <SignalWifi4Bar />
    if (isConnecting) return <Circle />
    return <SignalWifiOff />
  }

  const getStatusText = () => {
    if (isConnected) return 'Connected'
    if (isConnecting) return 'Connecting...'
    return 'Disconnected'
  }

  const getTooltipText = () => {
    const status = getConnectionStatus()
    if (error) return `Real-time updates: ${status} - ${error}`
    if (!isConnected && !isConnecting) {
      return `Real-time updates: ${status} - Join a circle to enable real-time updates`
    }
    return `Real-time updates: ${status}`
  }

  if (variant === 'minimal') {
    return (
      <Tooltip title={getTooltipText()} size='sm'>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 0.5,
            ...sx,
          }}
        >
          <Circle
            sx={{
              fontSize: 8,
              color:
                getStatusColor() === 'success'
                  ? 'success.main'
                  : getStatusColor() === 'warning'
                    ? 'warning.main'
                    : 'danger.main',
            }}
          />
          {showError && error && (
            <Typography level='body-xs' color='danger'>
              {error}
            </Typography>
          )}
        </Box>
      </Tooltip>
    )
  }

  if (variant === 'chip') {
    return (
      <Tooltip title={getTooltipText()} size='sm'>
        <Chip
          color={getStatusColor()}
          size='sm'
          variant='soft'
          startDecorator={getStatusIcon()}
          sx={sx}
        >
          {getStatusText()}
        </Chip>
      </Tooltip>
    )
  }

  // Full variant
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, ...sx }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {getStatusIcon()}
        <Typography level='body-sm' color={getStatusColor()}>
          {getStatusText()}
        </Typography>
      </Box>
      {showError && error && (
        <Typography level='body-xs' color='danger'>
          {error}
        </Typography>
      )}
    </Box>
  )
}

export default WebSocketConnectionStatus
