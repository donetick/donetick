import { Box, Button, CircularProgress, Container, Typography } from '@mui/joy'
import { useEffect, useState } from 'react'
import Logo from '../../Logo'
import { apiManager } from '../../utils/TokenManager'

import Cookies from 'js-cookie'
import { useRef } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useUserProfile } from '../../queries/UserQueries'
import { GetUserProfile } from '../../utils/Fetcher'

const AuthenticationLoading = () => {
  const { data: userProfile, refetch: refetchUserProfile } = useUserProfile()
  const Navigate = useNavigate()
  const hasCalledHandleOAuth2 = useRef(false)
  const [message, setMessage] = useState('Authenticating')
  const [subMessage, setSubMessage] = useState('Please wait')
  const [status, setStatus] = useState('pending')
  const { provider } = useParams()
  useEffect(() => {
    if (provider === 'oauth2' && !hasCalledHandleOAuth2.current) {
      hasCalledHandleOAuth2.current = true
      handleOAuth2()
    } else if (provider !== 'oauth2') {
      setMessage('Unknown Authentication Provider')
      setSubMessage('Please contact support')
    }
  }, [provider])
  const getUserProfileAndNavigateToHome = () => {
    GetUserProfile().then(data => {
      data.json().then(data => {
        refetchUserProfile().then(() => {
          // check if redirect url is set in cookie:
          const redirectUrl = Cookies.get('ca_redirect')
          if (redirectUrl) {
            Cookies.remove('ca_redirect')
            Navigate(redirectUrl)
          } else {
            Navigate('/my/chores')
          }
        })
      })
    })
  }
  const handleOAuth2 = () => {
    // get provider from params:
    const urlParams = new URLSearchParams(window.location.search)
    const code = urlParams.get('code')
    const returnedState = urlParams.get('state')

    const storedState = localStorage.getItem('authState')

    if (returnedState !== storedState) {
      setMessage('Authentication failed')
      setSubMessage('State does not match')
      setStatus('error')
      return
    }

    if (code) {
      const baseURL = apiManager.getApiURL()
      fetch(`${baseURL}/auth/${provider}/callback`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          code,
          state: returnedState,
        }),
      }).then(response => {
        if (response.status === 200) {
          return response.json().then(data => {
            localStorage.setItem('ca_token', data.token)
            localStorage.setItem('ca_expiration', data.expire)

            const redirectUrl = Cookies.get('ca_redirect')
            if (redirectUrl) {
              Cookies.remove('ca_redirect')
              Navigate(redirectUrl)
            } else {
              getUserProfileAndNavigateToHome()
            }
          })
        } else {
          console.error('Authentication failed')
          setMessage('Authentication failed')
          setSubMessage('Please try again')
          setStatus('error')
        }
      })
    }
  }

  return (
    <Container className='flex h-full items-center justify-center'>
      <Box
        className='flex flex-col items-center justify-center'
        sx={{
          minHeight: '80vh',
        }}
      >
        <CircularProgress
          determinate={status === 'error'}
          color={status === 'pending' ? 'primary' : 'danger'}
          sx={{ '--CircularProgress-size': '200px' }}
        >
          <Logo />
        </CircularProgress>
        <Box
          className='flex items-center gap-2'
          sx={{
            fontWeight: 700,
            fontSize: 24,
            mt: 2,
          }}
        >
          {message}
        </Box>
        <Typography level='body-md' fontWeight={500} textAlign={'center'}>
          {subMessage}
        </Typography>

        {status === 'error' && (
          <Button
            size='lg'
            variant='outlined'
            sx={{
              mt: 4,
            }}
          >
            <Link to='/login'>Go back Login</Link>
          </Button>
        )}
      </Box>
    </Container>
  )
}

export default AuthenticationLoading
