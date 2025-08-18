import { Capacitor } from '@capacitor/core'
// import { GoogleAuth } from '@codetrix-studio/capacitor-google-auth'
import { SocialLogin } from '@capgo/capacitor-social-login'
import { Settings } from '@mui/icons-material'
import GoogleIcon from '@mui/icons-material/Google'
import {
  Avatar,
  Box,
  Button,
  Container,
  Divider,
  IconButton,
  Input,
  Sheet,
  Typography,
} from '@mui/joy'
import { useQueryClient } from '@tanstack/react-query'
import Cookies from 'js-cookie'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LoginSocialGoogle } from 'reactjs-social-login'
import { GOOGLE_CLIENT_ID, REDIRECT_URL } from '../../Config'
import Logo from '../../Logo'
import { useResource } from '../../queries/ResourceQueries'
import { useNotification } from '../../service/NotificationProvider'
import { GetUserProfile, login } from '../../utils/Fetcher'
import { apiManager, isTokenValid } from '../../utils/TokenManager'
import MFAVerificationModal from './MFAVerificationModal'

const LoginView = () => {
  // Use React Query client directly to invalidate the user profile query
  const queryClient = useQueryClient()
  const [userProfile, setUserProfile] = useState(null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [mfaModalOpen, setMfaModalOpen] = useState(false)
  const [mfaSessionToken, setMfaSessionToken] = useState('')
  const { data: resource } = useResource()
  const { showError } = useNotification()
  const Navigate = useNavigate()
  useEffect(() => {
    const initializeSocialLogin = async () => {
      await SocialLogin.initialize({
        google: {
          webClientId: import.meta.env.VITE_APP_GOOGLE_CLIENT_ID,
          iOSClientId: import.meta.env.VITE_APP_IOS_CLIENT_ID,
          mode: 'online', // replaces grantOfflineAccess
        },
      })
    }
    initializeSocialLogin()
  }, [])
  useEffect(() => {
    if (isTokenValid()) {
      GetUserProfile().then(response => {
        if (response.status === 200) {
          return response.json().then(data => {
            setUserProfile(data.res)
          })
        } else {
          console.log('Failed to fetch user profile')
        }
      })
    }
  }, [])
  const handleSubmit = async e => {
    e.preventDefault()
    login(username, password)
      .then(response => {
        if (response.status === 200) {
          return response.json().then(data => {
            // Check if MFA is required
            if (data.mfaRequired) {
              setMfaSessionToken(data.sessionToken)
              setMfaModalOpen(true)
              return
            }

            // Normal login without MFA
            localStorage.setItem('ca_token', data.token)
            localStorage.setItem('ca_expiration', data.expire)

            // Refetch user profile after successful login
            queryClient.refetchQueries(['userProfile'])

            const redirectUrl = Cookies.get('ca_redirect')

            if (redirectUrl && redirectUrl !== '/') {
              console.log('Redirecting to', redirectUrl)

              Cookies.remove('ca_redirect')
              Navigate(redirectUrl)
            } else {
              Cookies.remove('ca_redirect')
              Navigate('/my/chores')
            }
          })
        } else if (response.status === 401) {
          showError({
            title: 'Login Failed',
            message: 'Wrong username or password',
          })
        } else {
          showError({
            title: 'Login Failed',
            message: 'An error occurred, please try again',
          })
          console.log('Login failed')
        }
      })
      .catch(err => {
        showError({
          title: 'Connection Error',
          message: 'Unable to communicate with server, please try again',
        })
        console.log('Login failed', err)
      })
  }

  const loggedWithProvider = function (provider, data) {
    const baseURL = apiManager.getApiURL()

    const getAccessToken = data => {
      if (data['access_token']) {
        // data["access_token"] is for Google
        return data['access_token']
      } else if (data['accessToken']) {
        // data["accessToken"] is for Google Capacitor
        return data['accessToken']['token']
      }
    }

    return fetch(`${baseURL}/auth/${provider}/callback`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider: provider,
        token: getAccessToken(data),
        data: data,
      }),
    }).then(response => {
      if (response.status === 200) {
        return response.json().then(data => {
          // Check if MFA is required for OAuth login
          if (data.mfaRequired) {
            setMfaSessionToken(data.sessionToken)
            setMfaModalOpen(true)
            return
          }

          // Normal OAuth login without MFA
          localStorage.setItem('ca_token', data.token)
          localStorage.setItem('ca_expiration', data.expire)

          // Refetch user profile after successful OAuth login
          queryClient.invalidateQueries(['userProfile'])

          const redirectUrl = Cookies.get('ca_redirect')
          if (redirectUrl) {
            Cookies.remove('ca_redirect')
            Navigate(redirectUrl)
          } else {
            getUserProfileAndNavigateToHome()
          }
        })
      }
      return response.json().then(() => {
        showError({
          title: 'Google Login Failed',
          message: "Couldn't log in with Google, please try again",
        })
      })
    })
  }
  const getUserProfileAndNavigateToHome = () => {
    // Refetch user profile after login using React Query
    queryClient.invalidateQueries(['userProfile']).then(() => {
      // check if redirect url is set in cookie:
      const redirectUrl = Cookies.get('ca_redirect')
      if (redirectUrl) {
        Cookies.remove('ca_redirect')
        Navigate(redirectUrl)
      } else {
        Navigate('/my/chores')
      }
    })
  }

  const handleMFASuccess = data => {
    localStorage.setItem('ca_token', data.token)
    localStorage.setItem('ca_expiration', data.expire)
    setMfaModalOpen(false)
    setMfaSessionToken('')

    // Refetch user profile after MFA success
    queryClient.invalidateQueries(['userProfile'])

    const redirectUrl = Cookies.get('ca_redirect')
    if (redirectUrl) {
      Cookies.remove('ca_redirect')
      Navigate(redirectUrl)
    } else {
      Navigate('/my/chores')
    }
  }

  const handleMFAError = errorMessage => {
    showError({
      title: 'Two-Factor Authentication Failed',
      message: errorMessage,
    })
  }

  const handleMFAClose = () => {
    setMfaModalOpen(false)
    setMfaSessionToken('')
  }

  const handleForgotPassword = () => {
    Navigate('/forgot-password')
  }
  const generateRandomState = () => {
    const randomState = Math.random().toString(32).substring(5)
    localStorage.setItem('authState', randomState)

    return randomState
  }

  const handleAuthentikLogin = () => {
    const authentikAuthorizeUrl = resource?.identity_provider?.auth_url

    const params = new URLSearchParams({
      response_type: 'code',
      client_id: resource?.identity_provider?.client_id,
      redirect_uri: `${window.location.origin}/auth/oauth2`,
      scope: 'openid profile email', // Your scopes
      state: generateRandomState(),
    })
    console.log('redirect', `${authentikAuthorizeUrl}?${params.toString()}`)

    window.location.href = `${authentikAuthorizeUrl}?${params.toString()}`
  }

  return (
    <Container
      component='main'
      maxWidth='xs'

      // make content center in the middle of the page:
    >
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
          {Capacitor.isNativePlatform() && (
            <IconButton
              //  on top right of the screen:
              sx={{ position: 'absolute', top: 2, right: 2, color: 'black' }}
              onClick={() => {
                Navigate('/login/settings')
              }}
            >
              {' '}
              <Settings />
            </IconButton>
          )}
          <Logo />

          <Typography level='h2'>
            Done
            <span style={{ color: '#06b6d4' }}>tick</span>
          </Typography>

          {userProfile && (
            <>
              <Avatar
                src={userProfile?.image}
                alt={userProfile?.username}
                size='lg'
                sx={{ mt: 2, width: '96px', height: '96px', mb: 1 }}
              />
              <Typography level='body-md' alignSelf={'center'}>
                Welcome back,{' '}
                {userProfile?.displayName || userProfile?.username}
              </Typography>

              <Button
                fullWidth
                size='lg'
                sx={{ mt: 3, mb: 2 }}
                onClick={() => {
                  getUserProfileAndNavigateToHome()
                }}
              >
                Continue as {userProfile.displayName || userProfile.username}
              </Button>
              <Button
                type='submit'
                fullWidth
                size='lg'
                variant='plain'
                sx={{
                  width: '100%',
                  mb: 2,
                  border: 'moccasin',
                  borderRadius: '8px',
                }}
                onClick={() => {
                  localStorage.removeItem('ca_token')
                  localStorage.removeItem('ca_expiration')
                  // go to login page:
                  window.location.href = '/login'
                }}
              >
                Logout
              </Button>
            </>
          )}
          {!userProfile && (
            <>
              <Typography level='body2'>
                Sign in to your account to continue
              </Typography>
              <Typography level='body2' alignSelf={'start'} mt={4}>
                Username
              </Typography>
              <Input
                margin='normal'
                required
                fullWidth
                id='email'
                label='Email Address'
                name='email'
                autoComplete='email'
                autoFocus
                value={username}
                onChange={e => {
                  setUsername(e.target.value)
                }}
              />
              <Typography level='body2' alignSelf={'start'}>
                Password:
              </Typography>
              <Input
                margin='normal'
                required
                fullWidth
                name='password'
                label='Password'
                type='password'
                id='password'
                value={password}
                onChange={e => {
                  setPassword(e.target.value)
                }}
              />

              <Button
                type='submit'
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
                onClick={handleSubmit}
              >
                Sign In
              </Button>
              <Button
                type='submit'
                fullWidth
                size='lg'
                variant='plain'
                sx={{
                  width: '100%',
                  mb: 2,
                  border: 'moccasin',
                  borderRadius: '8px',
                }}
                onClick={handleForgotPassword}
              >
                Forgot password?
              </Button>
            </>
          )}
          <Divider> or </Divider>
          {import.meta.env.VITE_IS_SELF_HOSTED !== 'true' && (
            <>
              {!Capacitor.isNativePlatform() && (
                <Box sx={{ width: '100%' }}>
                  <LoginSocialGoogle
                    client_id={GOOGLE_CLIENT_ID}
                    redirect_uri={REDIRECT_URL}
                    scope='openid profile email'
                    discoveryDocs='claims_supported'
                    access_type='online'
                    isOnlyGetToken={true}
                    onResolve={({ provider, data }) => {
                      loggedWithProvider(provider, data)
                    }}
                    onReject={() => {
                      showError({
                        title: 'Google Login Failed',
                        message:
                          "Couldn't log in with Google, please try again",
                      })
                    }}
                  >
                    <Button
                      variant='soft'
                      color='neutral'
                      size='lg'
                      fullWidth
                      sx={{
                        width: '100%',
                        mt: 1,
                        mb: 1,
                        border: 'moccasin',
                        borderRadius: '8px',
                      }}
                    >
                      <div className='flex gap-2'>
                        <GoogleIcon />
                        Continue with Google
                      </div>
                    </Button>
                  </LoginSocialGoogle>
                </Box>
              )}
              {Capacitor.isNativePlatform() && (
                <Box sx={{ width: '100%' }}>
                  <Button
                    fullWidth
                    variant='soft'
                    size='lg'
                    sx={{ mt: 3, mb: 2 }}
                    onClick={() => {
                      // GoogleAuth.initialize({
                      //   clientId: import.meta.env.VITE_APP_GOOGLE_CLIENT_ID,
                      //   scopes: ['profile', 'email', 'openid'],
                      //   grantOfflineAccess: true,
                      // })
                      // GoogleAuth.signIn().then(user => {
                      //   console.log('Google user', user)
                      //   loggedWithProvider('google', user.authentication)
                      // })

                      SocialLogin.login({
                        provider: 'google',
                        options: { scopes: ['profile', 'email', 'openid'] },
                      }).then(user => {
                        console.log('Google user', user)
                        loggedWithProvider('google', user.result)
                      })
                    }}
                  >
                    <div className='flex gap-2'>
                      <GoogleIcon />
                      Continue with Google
                    </div>
                  </Button>
                </Box>
              )}
            </>
          )}
          {resource?.identity_provider?.client_id && (
            <Button
              fullWidth
              color='neutral'
              variant='soft'
              size='lg'
              sx={{ mt: 3, mb: 2 }}
              onClick={handleAuthentikLogin}
            >
              Continue with {resource?.identity_provider?.name}
            </Button>
          )}

          <Button
            onClick={() => {
              Navigate('/signup')
            }}
            fullWidth
            variant='soft'
            size='lg'
            // sx={{ mt: 3, mb: 2 }}
          >
            Create new account
          </Button>
        </Sheet>
      </Box>

      <MFAVerificationModal
        open={mfaModalOpen}
        onClose={handleMFAClose}
        sessionToken={mfaSessionToken}
        onSuccess={handleMFASuccess}
        onError={handleMFAError}
      />
    </Container>
  )
}

export default LoginView
