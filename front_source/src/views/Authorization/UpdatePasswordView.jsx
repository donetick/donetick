// create boilerplate for ResetPasswordView:
import {
  Box,
  Button,
  Container,
  FormControl,
  FormHelperText,
  Input,
  Sheet,
  Typography,
} from '@mui/joy'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

import Logo from '../../Logo'
import { useNotification } from '../../service/NotificationProvider'
import { ChangePassword } from '../../utils/Fetcher'

const UpdatePasswordView = () => {
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [passwordError, setPasswordError] = useState(null)
  const [passworConfirmationError, setPasswordConfirmationError] =
    useState(null)
  const [searchParams] = useSearchParams()
  const { showError, showNotification } = useNotification()

  const verifiticationCode = searchParams.get('c')

  const handlePasswordChange = e => {
    const password = e.target.value
    setPassword(password)
    if (password.length < 8) {
      setPasswordError('Password must be at least 8 characters')
    } else {
      setPasswordError(null)
    }
  }
  const handlePasswordConfirmChange = e => {
    setPasswordConfirm(e.target.value)
    if (e.target.value !== password) {
      setPasswordConfirmationError('Passwords do not match')
    } else {
      setPasswordConfirmationError(null)
    }
  }

  const handleSubmit = async () => {
    if (passwordError != null || passworConfirmationError != null) {
      return
    }
    try {
      const response = await ChangePassword(verifiticationCode, password)

      if (response.ok) {
        showNotification({
          type: 'success',
          title: 'Password Updated',
          message:
            'Your password has been updated successfully. Redirecting to login...',
        })
        //  wait 3 seconds and then redirect to login:
        setTimeout(() => {
          navigate('/login')
        }, 3000)
      } else {
        showError({
          title: 'Password Update Failed',
          message: 'Failed to update password, please try again later',
        })
      }
    } catch (error) {
      showError({
        title: 'Password Update Failed',
        message: 'Failed to update password, please try again later',
      })
    }
  }
  return (
    <Container component='main' maxWidth='xs'>
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          marginTop: 4,
        }}
      >
        <Sheet
          component='form'
          sx={{
            mt: 1,
            width: '100%',
            display: 'flex',
            flexDirection: 'column',
            // alignItems: 'center',
            padding: 2,
            borderRadius: '8px',
            boxShadow: 'md',
          }}
        >
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              flexDirection: 'column',
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
            <Typography level='body2' mb={4}>
              Please enter your new password below
            </Typography>
          </Box>

          <FormControl error>
            <Input
              placeholder='Password'
              type='password'
              value={password}
              onChange={handlePasswordChange}
              error={passwordError !== null}
              // onKeyDown={e => {
              //   if (e.key === 'Enter' && validateForm(validateFormInput)) {
              //     handleSubmit(e)
              //   }
              // }}
            />
            <FormHelperText>{passwordError}</FormHelperText>
          </FormControl>

          <FormControl error>
            <Input
              placeholder='Confirm Password'
              type='password'
              value={passwordConfirm}
              onChange={handlePasswordConfirmChange}
              error={passworConfirmationError !== null}
              // onKeyDown={e => {
              //   if (e.key === 'Enter' && validateForm(validateFormInput)) {
              //     handleSubmit(e)
              //   }
              // }}
            />
            <FormHelperText>{passworConfirmationError}</FormHelperText>
          </FormControl>
          {/* helper to show password not matching : */}

          <Button
            fullWidth
            size='lg'
            sx={{
              mt: 5,
              mb: 1,
            }}
            onClick={handleSubmit}
          >
            Save Password
          </Button>
          <Button
            fullWidth
            size='lg'
            variant='soft'
            onClick={() => {
              navigate('/login')
            }}
          >
            Cancel
          </Button>
        </Sheet>
      </Box>
    </Container>
  )
}

export default UpdatePasswordView
