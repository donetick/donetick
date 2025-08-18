import { Preferences } from '@capacitor/preferences'
import { Box, Button, Container, Input, Sheet, Typography } from '@mui/joy'
import React from 'react'
import { useNavigate } from 'react-router-dom'
import { API_URL } from '../../Config'
import Logo from '../../Logo'
import { useNotification } from '../../service/NotificationProvider'
import { apiManager } from '../../utils/TokenManager'
const LoginSettings = () => {
  const Navigate = useNavigate()
  const [serverURL, setServerURL] = React.useState('')
  const { showError } = useNotification()

  React.useEffect(() => {
    Preferences.get({ key: 'customServerUrl' }).then(result => {
      setServerURL(result.value || API_URL)
    })
  }, [])

  const isValidServerURL = () => {
    return serverURL.match(/^(http|https):\/\/[^ "]+$/)
  }

  return (
    <Container component='main' maxWidth='xs'>
      <Box
        sx={{
          marginTop: 4,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
        }}
      >
        <Sheet
          component='form'
          sx={{
            mt: 1,
            width: '100%',

            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            padding: 2,
            borderRadius: '8px',
            boxShadow: 'md',
          }}
        >
          <Logo />

          <Typography level='h2'>
            Done
            <span
              style={{
                color: '#06b6d4',
              }}
            >
              tick
            </span>
          </Typography>

          <Typography level='body2' alignSelf={'start'} mt={4}>
            Server URL
          </Typography>
          <Input
            margin='normal'
            required
            fullWidth
            id='serverURL'
            name='serverURL'
            autoFocus
            value={serverURL}
            onChange={e => {
              setServerURL(e.target.value)
            }}
          />

          <Typography mt={1} level='body-xs'>
            Change the server URL to connect to a different server, such as your
            own self-hosted Donetick server.
          </Typography>
          <Typography mt={1} level='body-xs'>
            Please ensure to include the protocol (http:// or https://) and the
            port number if necessary (default Donetick port is 2021).
          </Typography>
          <Button
            fullWidth
            size='lg'
            variant='solid'
            sx={{
              width: '100%',
              mt: 3,
              mb: 2,
              border: 'moccasin',
              borderRadius: '8px',
            }}
            onClick={() => {
              if (serverURL === '') {
                Preferences.set({
                  key: 'customServerUrl',
                  value: API_URL,
                }).then(() => {
                  Navigate('/login')
                })
                return
              }
              if (!isValidServerURL()) {
                showError({
                  title: 'Invalid Server URL',
                  message:
                    'Please enter a valid server URL with protocol (http:// or https://)',
                })
                return
              }
              Preferences.set({
                key: 'customServerUrl',
                value: serverURL,
              }).then(() => {
                apiManager.updateApiURL(serverURL + '/api/v1')
                Navigate('/login')
              })
            }}
          >
            Save
          </Button>
          <Button
            fullWidth
            size='lg'
            variant='soft'
            color='danger'
            sx={{
              width: '100%',

              mb: 2,
              border: 'moccasin',
              borderRadius: '8px',
            }}
            onClick={() => {
              Preferences.set({ key: 'customServerUrl', value: API_URL }).then(
                () => {
                  Navigate('/login')
                },
              )
            }}
          >
            Cancel and Reset
          </Button>
        </Sheet>
      </Box>
    </Container>
  )
}

export default LoginSettings
