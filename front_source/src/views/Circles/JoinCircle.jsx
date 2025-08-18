import { Box, Container, Input, Sheet, Typography } from '@mui/joy'
import Logo from '../../Logo'

import { Button } from '@mui/joy'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useUserProfile } from '../../queries/UserQueries'
import { JoinCircle } from '../../utils/Fetcher'
const JoinCircleView = () => {
  const { data: userProfile } = useUserProfile()

  let [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const code = searchParams.get('code')

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
          {code && userProfile && (
            <>
              <Typography level='body-md' alignSelf={'center'}>
                Hi {userProfile?.displayName}, you have been invited to join the
                circle{' '}
              </Typography>
              <Input
                fullWidth
                placeholder='Enter code'
                value={code}
                disabled={!!code}
                size='lg'
                sx={{
                  width: '220px',
                  mb: 1,
                }}
              />
              <Typography level='body-md' alignSelf={'center'}>
                Joining will give you access to the circle's chores and members.
              </Typography>
              <Typography level='body-md' alignSelf={'center'}>
                You can leave the circle later from you Settings page.
              </Typography>
              <Button
                fullWidth
                size='lg'
                sx={{ mt: 3, mb: 2 }}
                onClick={() => {
                  JoinCircle(code).then(resp => {
                    if (resp.ok) {
                      alert(
                        'Joined circle successfully, wait for the circle owner to accept your request.',
                      )
                      navigate('/my/chores')
                    } else {
                      if (resp.status === 409) {
                        alert('You are already a member of this circle')
                      } else {
                        alert('Failed to join circle')
                      }
                      navigate('/my/chores')
                    }
                  })
                }}
              >
                Join Circle
              </Button>
              <Button
                fullWidth
                size='lg'
                q
                variant='plain'
                sx={{
                  width: '100%',
                  mb: 2,
                  border: 'moccasin',
                  borderRadius: '8px',
                }}
                onClick={() => {
                  navigate('/my/chores')
                }}
              >
                Cancel
              </Button>
            </>
          )}
          {!code ||
            (!userProfile && (
              <>
                <Typography level='body-md' alignSelf={'center'}>
                  You need to be logged in to join a circle
                </Typography>
                <Typography level='body-md' alignSelf={'center'} sx={{ mb: 9 }}>
                  Login or sign up to continue
                </Typography>
                <Button
                  fullWidth
                  size='lg'
                  sx={{ mt: 3, mb: 2 }}
                  onClick={() => {
                    navigate('/login')
                  }}
                >
                  Login
                </Button>
              </>
            ))}
        </Sheet>
      </Box>
    </Container>
  )
}

export default JoinCircleView
