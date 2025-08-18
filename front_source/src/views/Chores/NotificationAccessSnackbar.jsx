import { Capacitor } from '@capacitor/core'
import { LocalNotifications } from '@capacitor/local-notifications'
import { Preferences } from '@capacitor/preferences'
import { Button, Snackbar, Stack, Typography } from '@mui/joy'
import { useEffect, useState } from 'react'

const NotificationAccessSnackbar = () => {
  const [open, setOpen] = useState(false)

  // Define the function outside of useEffect
  const getNotificationPreferences = async () => {
    const ret = await Preferences.get({ key: 'notificationPreferences' })
    return JSON.parse(ret.value) || {}
  }

  useEffect(() => {
    // Only run the effect on native platforms
    if (Capacitor.isNativePlatform()) {
      getNotificationPreferences().then(data => {
        // if optOut is true then don't show the snackbar
        if (data?.optOut === true || data?.granted === true) {
          return
        }
        setOpen(true)
      })
    }
  }, [])

  // Return early if not on a native platform
  if (!Capacitor.isNativePlatform()) {
    return null
  }

  return (
    <Snackbar
      // autoHideDuration={5000}
      variant='solid'
      color='primary'
      size='lg'
      invertedColors
      open={open}
      onClose={() => setOpen(false)}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      sx={theme => ({
        background: `linear-gradient(45deg, ${theme.palette.primary[600]} 30%, ${theme.palette.primary[500]} 90%})`,
        maxWidth: 360,
      })}
    >
      <div>
        <Typography level='title-lg'>Need Notification?</Typography>
        <Typography sx={{ mt: 1, mb: 2 }}>
          You need to enable permission to receive notifications, do you want to
          enable it?
        </Typography>
        <Stack direction='row' spacing={1}>
          <Button
            variant='solid'
            color='primary'
            onClick={() => {
              const notificationPreferences = { optOut: false }
              LocalNotifications.requestPermissions().then(resp => {
                if (resp.display === 'granted') {
                  notificationPreferences['granted'] = true
                }
              })
              Preferences.set({
                key: 'notificationPreferences',
                value: JSON.stringify(notificationPreferences),
              })
              setOpen(false)
            }}
          >
            Yes
          </Button>
          <Button
            variant='outlined'
            color='primary'
            onClick={() => {
              const notificationPreferences = { optOut: true }
              Preferences.set({
                key: 'notificationPreferences',
                value: JSON.stringify(notificationPreferences),
              })
              setOpen(false)
            }}
          >
            No, Keep it Disabled
          </Button>
        </Stack>
      </div>
    </Snackbar>
  )
}

export default NotificationAccessSnackbar
