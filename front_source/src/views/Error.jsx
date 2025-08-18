import { HomeRounded, Login } from '@mui/icons-material'
import {
  Box,
  Button,
  CircularProgress,
  Container,
  Textarea,
  Typography,
} from '@mui/joy'
import { Link } from 'react-router-dom'
import Logo from '../Logo' // Adjust the import path as necessary

const Error = () => {
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
          Ops, something went wrong
        </Box>
        <Typography level='body-md' fontWeight={500} textAlign={'center'}>
          if you think this is a mistake, please contact us or{' '}
          <a
            href='https://github.com/donetick/donetick/issues/new'
            style={{
              textDecoration: 'underline',
            }}
          >
            open issue here
          </a>{' '}
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

export default Error
