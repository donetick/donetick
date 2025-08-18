import { Security, Smartphone } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Input,
  Link,
  ModalClose,
  Stack,
  Typography,
} from '@mui/joy'
import { useState } from 'react'
import FadeModal from '../../components/common/FadeModal'
import { VerifyMFA } from '../../utils/Fetcher'

const MFAVerificationModal = ({
  open,
  onClose,
  sessionToken,
  onSuccess,
  onError,
}) => {
  const [verificationCode, setVerificationCode] = useState('')
  const [isBackupCode, setIsBackupCode] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleVerify = async () => {
    if (!verificationCode.trim()) {
      setError('Please enter a verification code')
      return
    }

    setLoading(true)
    setError('')

    try {
      const response = await VerifyMFA(sessionToken, verificationCode)

      if (response.ok) {
        const data = await response.json()
        onSuccess(data)
      } else {
        const errorData = await response.json()
        setError(
          errorData.message || 'Invalid verification code. Please try again.',
        )
      }
    } catch (error) {
      setError('Failed to verify code. Please try again.')
      console.error('MFA verification error:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleClose = () => {
    setVerificationCode('')
    setIsBackupCode(false)
    setError('')
    setLoading(false)
    onClose()
  }

  const handleKeyPress = e => {
    if (e.key === 'Enter' && !loading) {
      handleVerify()
    }
  }

  return (
    <FadeModal open={open} onClose={handleClose} size='sm'>
      <ModalClose />

      <Box className='mb-4 text-center'>
        <Security sx={{ fontSize: 48, color: 'primary.main', mb: 2 }} />
        <Typography level='h4' sx={{ mb: 1 }}>
          Two-Factor Authentication
        </Typography>
        <Typography level='body-md' sx={{ color: 'text.secondary' }}>
          Enter the verification code from your authenticator app
        </Typography>
      </Box>

      <Stack spacing={3}>
        <Box>
          <Typography level='body-sm' sx={{ mb: 1 }}>
            {isBackupCode ? 'Backup Code' : 'Verification Code'}
          </Typography>
          <Input
            placeholder={
              isBackupCode ? 'Enter backup code' : 'Enter 6-digit code'
            }
            value={verificationCode}
            onChange={e => setVerificationCode(e.target.value)}
            onKeyPress={handleKeyPress}
            sx={{
              textAlign: 'center',
              fontSize: '1.1em',
              letterSpacing: isBackupCode ? 'normal' : '0.1em',
            }}
            slotProps={{
              input: {
                maxLength: isBackupCode ? 50 : 6,
                pattern: isBackupCode ? undefined : '[0-9]*',
              },
            }}
            startDecorator={<Smartphone />}
            autoFocus
          />
        </Box>

        {error && (
          <Alert color='danger' size='sm'>
            {error}
          </Alert>
        )}

        <Button
          color='primary'
          loading={loading}
          onClick={handleVerify}
          disabled={!verificationCode.trim()}
          size='lg'
        >
          Verify & Sign In
        </Button>

        <Box className='text-center'>
          <Link
            component='button'
            type='button'
            onClick={() => {
              setIsBackupCode(!isBackupCode)
              setVerificationCode('')
              setError('')
            }}
            sx={{ fontSize: 'sm' }}
          >
            {isBackupCode
              ? 'Use authenticator app instead'
              : "Can't access your authenticator? Use a backup code"}
          </Link>
        </Box>

        <Alert color='neutral' size='sm'>
          <Typography level='body-xs'>
            Having trouble? Make sure your authenticator app is synced and try
            again. Each backup code can only be used once.
          </Typography>
        </Alert>
      </Stack>
    </FadeModal>
  )
}

export default MFAVerificationModal
