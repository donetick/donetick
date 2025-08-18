import { Box, Container, Sheet, Typography } from '@mui/joy'
import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import Logo from '../../Logo'

const PaymentSuccessView = () => {
  const navigate = useNavigate()

  useEffect(() => {
    const timer = setTimeout(() => {
      navigate('/settings')
    }, 5000)
    return () => clearTimeout(timer)
  }, [navigate])

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
          <Typography level='h2' sx={{ mt: 2, mb: 1 }}>
            Payment Successful!
          </Typography>
          <Typography level='body-md' sx={{ mb: 2 }}>
            You will be redirected to the settings page shortly.
          </Typography>
        </Sheet>
      </Box>
    </Container>
  )
}

export default PaymentSuccessView
