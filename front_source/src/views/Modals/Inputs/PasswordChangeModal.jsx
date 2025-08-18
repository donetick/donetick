import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  Input,
  Typography,
} from '@mui/joy'
import React, { useEffect } from 'react'
import FadeModal from '../../../components/common/FadeModal'

function PassowrdChangeModal({ isOpen, onClose }) {
  const [password, setPassword] = React.useState('')
  const [confirmPassword, setConfirmPassword] = React.useState('')
  const [passwordError, setPasswordError] = React.useState(false)
  const [passwordTouched, setPasswordTouched] = React.useState(false)
  const [confirmPasswordTouched, setConfirmPasswordTouched] =
    React.useState(false)
  useEffect(() => {
    if (!passwordTouched || !confirmPasswordTouched) {
      return
    } else if (password !== confirmPassword) {
      setPasswordError('Passwords do not match')
    } else if (password.length < 8) {
      setPasswordError('Password must be at least 8 characters')
    } else if (password.length > 50) {
      setPasswordError('Password must be less than 50 characters')
    } else {
      setPasswordError(null)
    }
  }, [password, confirmPassword, passwordTouched, confirmPasswordTouched])

  const handleAction = isConfirmed => {
    if (!isConfirmed) {
      onClose(null)
      return
    }
    onClose(password)
  }

  return (
    <FadeModal open={isOpen} onClose={onClose}>
      <Typography level='h4' mb={1}>
        Change Password
      </Typography>

      <Typography level='body-md' gutterBottom>
        Please enter your new password.
      </Typography>
      <FormControl>
        <Typography level='body2' alignSelf={'start'}>
          New Password
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
            setPasswordTouched(true)
            setPassword(e.target.value)
          }}
        />
      </FormControl>

      <FormControl>
        <Typography level='body2' alignSelf={'start'}>
          Confirm Password
        </Typography>
        <Input
          margin='normal'
          required
          fullWidth
          name='confirmPassword'
          label='confirmPassword'
          type='password'
          id='confirmPassword'
          value={confirmPassword}
          onChange={e => {
            setConfirmPasswordTouched(true)
            setConfirmPassword(e.target.value)
          }}
        />

        <FormHelperText>{passwordError}</FormHelperText>
      </FormControl>
      <Box display={'flex'} justifyContent={'space-around'} mt={1}>
        <Button
          disabled={passwordError != null}
          onClick={() => {
            handleAction(true)
          }}
          fullWidth
          sx={{ mr: 1 }}
        >
          Change Password
        </Button>
        <Button
          onClick={() => {
            handleAction(false)
          }}
          variant='outlined'
        >
          Cancel
        </Button>
      </Box>
    </FadeModal>
  )
}
export default PassowrdChangeModal
