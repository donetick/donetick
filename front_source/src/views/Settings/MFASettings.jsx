import { CheckCircle, Security, Smartphone } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  Divider,
  Input,
  Modal,
  ModalClose,
  ModalDialog,
  Stack,
  Typography,
} from '@mui/joy'
import QRCode from 'qrcode'
import { useEffect, useState } from 'react'
import {
  ConfirmMFA,
  DisableMFA,
  GetMFAStatus,
  RegenerateBackupCodes,
  SetupMFA,
} from '../../utils/Fetcher'

const MFASettings = () => {
  const [mfaEnabled, setMfaEnabled] = useState(false)
  const [loading, setLoading] = useState(true)
  const [setupModalOpen, setSetupModalOpen] = useState(false)
  const [disableModalOpen, setDisableModalOpen] = useState(false)
  const [backupCodesModalOpen, setBackupCodesModalOpen] = useState(false)
  const [setupData, setSetupData] = useState(null)
  const [qrCodeDataUrl, setQrCodeDataUrl] = useState('')
  const [verificationCode, setVerificationCode] = useState('')
  const [disableCode, setDisableCode] = useState('')
  const [backupCodes, setBackupCodes] = useState([])
  const [setupStep, setSetupStep] = useState(1) // 1: QR Code, 2: Verification, 3: Backup Codes
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    fetchMFAStatus()
  }, [])

  const fetchMFAStatus = async () => {
    try {
      setLoading(true)
      const response = await GetMFAStatus()
      if (response.ok) {
        const data = await response.json()
        setMfaEnabled(data.mfaEnabled)
      }
    } catch (error) {
      console.error('Error fetching MFA status:', error)
    } finally {
      setLoading(false)
    }
  }

  const generateQRCode = async url => {
    try {
      const qrCodeDataUrl = await QRCode.toDataURL(url, {
        width: 200,
        margin: 2,
      })
      setQrCodeDataUrl(qrCodeDataUrl)
    } catch (error) {
      console.error('Error generating QR code:', error)
      setError('Failed to generate QR code')
    }
  }

  const handleSetupMFA = async () => {
    try {
      setError('')
      const response = await SetupMFA()

      console.log('MFA Setup Response Status:', response.status)
      console.log('MFA Setup Response:', response)

      if (response.ok) {
        const data = await response.json()
        console.log('MFA Setup Data:', data)

        // Check for either qrCode (base64 image) or qrCodeUrl (TOTP URL)
        if (!data.qrCodeUrl || !data.backupCodes || !data.secret) {
          console.error('Missing required MFA data:', {
            hasQrCode: !!data.qrCode,
            hasQrCodeUrl: !!data.qrCodeUrl,
            hasSecret: !!data.secret,
          })
          setError('Invalid response from server. Missing QR code or secret.')
          return
        }
        if (data.backupCodes) {
          console.log('Backup Codes:', data.backupCodes)

          setBackupCodes(data.backupCodes)
        }
        // If we have a qrCodeUrl, generate the QR code image
        if (data.qrCodeUrl) {
          await generateQRCode(data.qrCodeUrl)
        }

        setSetupData(data)
        setSetupModalOpen(true)
        setSetupStep(1)
      } else {
        // Handle different error status codes
        if (response.status === 404) {
          setError(
            'MFA setup endpoint not found. This feature may not be available yet.',
          )
        } else if (response.status === 401) {
          setError('Unauthorized. Please login again.')
        } else if (response.status === 500) {
          setError('Server error. Please try again later.')
        } else {
          const errorData = await response.json().catch(() => ({}))
          setError(
            errorData.message ||
              `Failed to setup MFA (${response.status}). Please try again.`,
          )
        }
      }
    } catch (error) {
      console.error('Error setting up MFA:', error)
      setError('Network error. Please check your connection and try again.')
    }
  }

  const handleConfirmMFA = async () => {
    try {
      setError('')
      const response = await ConfirmMFA(
        setupData.secret,
        verificationCode,
        setupData.backupCodes,
      )
      if (response.ok) {
        setSetupStep(3)
        setMfaEnabled(true)
        setSuccess('MFA has been successfully enabled!')
      } else {
        setError('Invalid verification code. Please try again.')
      }
    } catch (error) {
      setError('Failed to confirm MFA. Please try again.')
      console.error('Error confirming MFA:', error)
    }
  }

  const handleDisableMFA = async () => {
    try {
      setError('')
      const response = await DisableMFA(disableCode)
      if (response.ok) {
        setMfaEnabled(false)
        setDisableModalOpen(false)
        setDisableCode('')
        setSuccess('MFA has been disabled successfully!')
      } else {
        setError('Invalid verification code. Please try again.')
      }
    } catch (error) {
      setError('Failed to disable MFA. Please try again.')
      console.error('Error disabling MFA:', error)
    }
  }

  const handleRegenerateBackupCodes = async () => {
    try {
      setError('')
      const response = await RegenerateBackupCodes()
      if (response.ok) {
        const data = await response.json()
        setBackupCodes(data.backupCodes)
        setBackupCodesModalOpen(true)
        setSuccess('New backup codes have been generated!')
      } else {
        setError('Failed to regenerate backup codes. Please try again.')
      }
    } catch (error) {
      setError('Failed to regenerate backup codes. Please try again.')
      console.error('Error regenerating backup codes:', error)
    }
  }

  const closeSetupModal = () => {
    setSetupModalOpen(false)
    setSetupStep(1)
    setVerificationCode('')
    setSetupData(null)
    setQrCodeDataUrl('')
    setError('')
  }

  const closeDisableModal = () => {
    setDisableModalOpen(false)
    setDisableCode('')
    setError('')
  }

  if (loading) {
    return (
      <Box className='flex justify-center py-4'>
        <CircularProgress />
      </Box>
    )
  }

  return (
    <div className='grid gap-4 py-4'>
      <Typography level='h3'>Multi-Factor Authentication</Typography>
      <Divider />
      <Typography level='body-md'>
        Add an extra layer of security to your account with multi-factor
        authentication (MFA). When enabled, you&apos;ll need to provide a
        verification code from your authenticator app in addition to your
        password when signing in.
      </Typography>

      {success && (
        <Alert color='success' onClose={() => setSuccess('')}>
          {success}
        </Alert>
      )}

      {error && (
        <Alert color='danger' onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Card variant='outlined'>
        <Box className='flex items-center justify-between'>
          <Box className='flex items-center gap-3'>
            <Security color='primary' />
            <Box>
              <Typography level='title-md'>
                Two-Factor Authentication
              </Typography>
              <Typography level='body-sm' sx={{ color: 'text.secondary' }}>
                {mfaEnabled
                  ? 'Your account is protected with 2FA'
                  : 'Secure your account with an authenticator app'}
              </Typography>
            </Box>
          </Box>
          <Box className='flex items-center gap-2'>
            {mfaEnabled ? (
              <Button
                color='danger'
                variant='outlined'
                onClick={() => setDisableModalOpen(true)}
              >
                Disable
              </Button>
            ) : (
              <Button color='primary' variant='solid' onClick={handleSetupMFA}>
                Enable
              </Button>
            )}
          </Box>
        </Box>
      </Card>
      {/* 
      {mfaEnabled && (
        <Card variant='outlined'>
          <Box className='flex items-center justify-between'>
            <Box className='flex items-center gap-3'>
              <Key color='primary' />
              <Box>
                <Typography level='title-md'>Backup Codes</Typography>
                <Typography level='body-sm' sx={{ color: 'text.secondary' }}>
                  Generate new backup codes in case you lose access to your
                  authenticator
                </Typography>
              </Box>
            </Box>
            <Button
              color='neutral'
              variant='outlined'
              size='sm'
              onClick={handleRegenerateBackupCodes}
            >
              Generate New Codes
            </Button>
          </Box>
        </Card>
      )} */}

      {/* Setup MFA Modal */}
      <Modal open={setupModalOpen} onClose={closeSetupModal}>
        <ModalDialog size='md' sx={{ maxWidth: 500 }}>
          <ModalClose />
          <Typography level='h4' sx={{ mb: 2 }}>
            Set up Multi-Factor Authentication
          </Typography>

          {setupStep === 1 && setupData && (
            <Stack spacing={3}>
              <Typography level='body-md'>
                <strong>Step 1:</strong> Scan the QR code below with your
                authenticator app (Google Authenticator, Authy, etc.)
              </Typography>

              <Box className='flex justify-center rounded bg-white p-4'>
                {qrCodeDataUrl || setupData.qrCode ? (
                  <img
                    src={
                      qrCodeDataUrl ||
                      `data:image/png;base64,${setupData.qrCode}`
                    }
                    alt='MFA QR Code'
                    style={{ maxWidth: '200px', maxHeight: '200px' }}
                  />
                ) : (
                  <Alert color='danger'>
                    QR code could not be generated. Please try again or use the
                    manual entry key below.
                  </Alert>
                )}
              </Box>

              <Alert
                color='neutral'
                variant='soft'
                sx={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'flex-start',
                }}
              >
                <Typography level='title-sm'>
                  <strong>Manual entry key:</strong>
                </Typography>
                <Typography
                  level='body-sm'
                  sx={{ wordBreak: 'break-all', whiteSpace: 'pre-wrap' }}
                >
                  {setupData.secret}
                </Typography>
              </Alert>

              <Button
                color='primary'
                onClick={() => setSetupStep(2)}
                startDecorator={<Smartphone />}
              >
                I've added the account to my app
              </Button>
            </Stack>
          )}

          {setupStep === 2 && (
            <Stack spacing={3}>
              <Typography level='body-md'>
                <strong>Step 2:</strong> Enter the 6-digit verification code
                from your authenticator app
              </Typography>

              <Input
                placeholder='Enter 6-digit code'
                value={verificationCode}
                size='lg'
                //   send on enter:
                onKeyDown={e => {
                  if (e.key === 'Enter' && verificationCode.length === 6) {
                    handleConfirmMFA()
                  }
                }}
                onChange={e => setVerificationCode(e.target.value)}
                sx={{
                  textAlign: 'center',
                  fontSize: '1.2em',
                  letterSpacing: verificationCode.length === 0 ? '' : '0.4em',
                }}
                slotProps={{
                  input: {
                    maxLength: 6,
                    pattern: '[0-9]*',
                  },
                }}
              />

              {error && <Alert color='danger'>{error}</Alert>}

              <Box className='flex gap-2'>
                <Button
                  variant='outlined'
                  onClick={() => setSetupStep(1)}
                  sx={{ flex: 1 }}
                >
                  Back
                </Button>
                <Button
                  color='primary'
                  onClick={handleConfirmMFA}
                  disabled={verificationCode.length !== 6}
                  sx={{ flex: 1 }}
                >
                  Verify & Enable
                </Button>
              </Box>
            </Stack>
          )}

          {setupStep === 3 && (
            <Stack spacing={3}>
              <Box className='text-center'>
                <CheckCircle color='success' sx={{ fontSize: 48, mb: 2 }} />
                <Typography level='h4' color='success'>
                  MFA Successfully Enabled!
                </Typography>
              </Box>

              <Alert color='warning'>
                <Typography level='title-sm' sx={{ mb: 1 }}>
                  Save these backup codes in a safe place
                </Typography>
                <Typography level='body-sm'>
                  You can use these codes to access your account if you lose
                  your authenticator device. Each code can only be used once.
                </Typography>
              </Alert>

              <Card variant='outlined' sx={{ p: 2 }}>
                <Box className='grid grid-cols-2 gap-2 font-mono text-sm'>
                  {backupCodes?.map((code, index) => (
                    <Typography
                      key={index}
                      level='body-sm'
                      sx={{ fontFamily: 'monospace' }}
                    >
                      {code}
                    </Typography>
                  ))}
                </Box>
              </Card>

              <Button color='primary' onClick={closeSetupModal}>
                I've saved my backup codes
              </Button>
            </Stack>
          )}
        </ModalDialog>
      </Modal>

      {/* Disable MFA Modal */}
      <Modal open={disableModalOpen} onClose={closeDisableModal}>
        <ModalDialog size='sm'>
          <ModalClose />
          <Typography level='h4' sx={{ mb: 2 }}>
            Disable Multi-Factor Authentication
          </Typography>

          <Stack spacing={3}>
            <Alert color='warning'>
              <Typography level='body-sm'>
                Disabling MFA will make your account less secure. Are you sure
                you want to continue?
              </Typography>
            </Alert>

            <Typography level='body-md'>
              Enter a verification code from your authenticator app to confirm:
            </Typography>

            <Input
              placeholder='Enter 6-digit code'
              value={disableCode}
              size='lg'
              onKeyDown={e => {
                if (e.key === 'Enter' && disableCode.length === 6) {
                  handleDisableMFA()
                }
              }}
              onChange={e => setDisableCode(e.target.value)}
              sx={{
                textAlign: 'center',
                fontSize: '1.2em',
                letterSpacing: verificationCode.length === 0 ? '' : '0.4em',
              }}
              slotProps={{
                input: {
                  maxLength: 6,
                  pattern: '[0-9]*',
                },
              }}
            />

            {error && <Alert color='danger'>{error}</Alert>}

            <Box className='flex gap-2'>
              <Button
                variant='outlined'
                onClick={closeDisableModal}
                sx={{ flex: 1 }}
              >
                Cancel
              </Button>
              <Button
                color='danger'
                onClick={handleDisableMFA}
                disabled={disableCode.length !== 6}
                sx={{ flex: 1 }}
              >
                Disable MFA
              </Button>
            </Box>
          </Stack>
        </ModalDialog>
      </Modal>

      {/* Backup Codes Modal */}
      <Modal
        open={backupCodesModalOpen}
        onClose={() => setBackupCodesModalOpen(false)}
      >
        <ModalDialog size='sm'>
          <ModalClose />
          <Typography level='h4' sx={{ mb: 2 }}>
            New Backup Codes
          </Typography>

          <Stack spacing={3}>
            <Alert color='warning'>
              <Typography level='body-sm'>
                Your previous backup codes are now invalid. Save these new codes
                in a safe place. Each code can only be used once.
              </Typography>
            </Alert>

            <Card variant='outlined' sx={{ p: 2 }}>
              <Box className='grid grid-cols-2 gap-2 font-mono text-sm'>
                {backupCodes?.map((code, index) => (
                  <Typography
                    key={index}
                    level='body-sm'
                    sx={{ fontFamily: 'monospace' }}
                  >
                    {code}
                  </Typography>
                ))}
              </Box>
            </Card>

            <Button
              color='primary'
              onClick={() => setBackupCodesModalOpen(false)}
            >
              I've saved my backup codes
            </Button>
          </Stack>
        </ModalDialog>
      </Modal>
    </div>
  )
}

export default MFASettings
