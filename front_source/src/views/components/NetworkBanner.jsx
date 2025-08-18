import { WifiOff } from '@mui/icons-material'
import { Alert, Box } from '@mui/joy'
import { useEffect, useState } from 'react'
import Z_INDEX from '../../constants/zIndex'
import { networkManager } from '../../hooks/NetworkManager'

const NetworkBanner = () => {
  const [isOnline, setIsOnline] = useState(networkManager.isOnline)
  useEffect(() => {
    const handleNetworkChange = isOnline => {
      setIsOnline(isOnline)
    }

    networkManager.registerNetworkListener(handleNetworkChange)
  }, [])

  return (
    <Box sx={{}}>
      {!isOnline && (
        <Alert
          variant='soft'
          color='warning'
          sx={{
            position: 'fixed',
            top: 0,
            left: 0,
            zIndex: Z_INDEX.NETWORK_BANNER,
            padding: '4px',
            pt: `calc( env(safe-area-inset-top, 0px))`,
            width: '100%',
            justifyContent: 'center',
            alignItems: 'center',
            fontSize: '10px',
            fontWeight: 'md',
          }}
          startDecorator={<WifiOff />}
        >
          You are currently offline. Some features may not be available.
        </Alert>
      )}
    </Box>
  )
}

export default NetworkBanner
