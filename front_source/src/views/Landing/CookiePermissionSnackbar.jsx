import { Button, Snackbar } from '@mui/joy'
import Cookies from 'js-cookie'
import { useEffect, useState } from 'react'

const CookiePermissionSnackbar = () => {
  useEffect(() => {
    const cookiePermission = Cookies.get('cookies_permission')

    if (cookiePermission !== 'true') {
      setOpen(true)
    }
  }, [])

  const [open, setOpen] = useState(false)
  const handleClose = () => {
    Cookies.set('cookies_permission', 'true')
    setOpen(false)
  }

  return (
    <Snackbar
      open={open}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      onClose={(event, reason) => {
        if (reason === 'clickaway') {
          return
        }
        // Cookies.set('cookies_permission', 'true')
        handleClose()
      }}
    >
      We use cookies to ensure you get the best experience on our website.
      <Button variant='soft' onClick={handleClose}>
        Accept
      </Button>
    </Snackbar>
  )
}

export default CookiePermissionSnackbar
