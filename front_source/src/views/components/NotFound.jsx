import { HomeRounded, Login } from '@mui/icons-material'
import { Box, Button, CircularProgress, Container } from '@mui/joy'
import { Typography } from '@mui/material'
import { Link } from 'react-router-dom' // Assuming you are using React Router
import Logo from '../../Logo'

const NotFound = () => {
  return (
    <Container className='flex h-full items-center justify-center'>
      <Box
        className='flex flex-col items-center justify-center'
        sx={{
          minHeight: '80vh',
        }}
      >
        <CircularProgress
          value={100}
          color='danger' // Set the color to 'error' for danger color
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
          Page Not Found
        </Box>
        <Typography level='h2' fontWeight={500} textAlign={'center'}>
          Sorry, I could be wrong but I think you are lost.
        </Typography>
        <Button
          component={Link}
          to='/my/chores'
          variant='outlined'
          color='primary'
          sx={{ mt: 4 }}
          size='lg'
          startDecorator={<HomeRounded />}
        >
          Home
        </Button>
        <Button
          component={Link}
          to='/login'
          variant='outlined'
          color='primary'
          sx={{ mt: 1 }}
          size='lg'
          startDecorator={<Login />}
        >
          Login
        </Button>
      </Box>
    </Container>
  )
}

export default NotFound
