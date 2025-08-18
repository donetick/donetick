import {
  AddHome,
  AutoAwesome,
  Cloud,
  GitHub,
  InstallMobile,
  Storage,
} from '@mui/icons-material'
import { Box, Button, Card, Grid, styled, Typography } from '@mui/joy'
import { useNavigate } from 'react-router-dom'
const IconContainer = styled('div')({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: '50%',
  minWidth: '60px',
  height: '60px',
  marginRight: '16px',
})

const ButtonContainer = styled('div')({
  display: 'flex',
  justifyContent: 'center',
  marginTop: 'auto',
})

function StartOptionCard({ icon: Icon, title, description, button, index }) {
  return (
    <Card
      variant='plain'
      sx={{
        p: 2,
        display: 'flex',
        minHeight: '300px',
        py: 4,
        flexDirection: 'column',
        justifyContent: 'space-between',
      }}
      data-aos-delay={100 * index}
      data-aos-anchor='[data-aos-id-getting-started-container]'
      data-aos='fade-up'
    >
      {/* Changes are within this div */}
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <IconContainer>{Icon}</IconContainer>

        <Typography level='h4' textAlign={'center'}>
          {title}
        </Typography>
      </Box>

      <Typography level='body-md' color='neutral' lineHeight={1.6}>
        {description}
      </Typography>

      <ButtonContainer>{button}</ButtonContainer>
    </Card>
  )
}

const GettingStarted = () => {
  const navigate = useNavigate()
  const information = [
    {
      title: 'Donetick Web',
      icon: <Cloud style={{ fontSize: '48px' }} />,
      description:
        'The easiest way! Just create account and start using Donetick',
      button: (
        <Button
          size='lg'
          fullWidth
          startDecorator={<AutoAwesome />}
          onClick={() => {
            navigate('/my/chores')
          }}
        >
          Start Now!
        </Button>
      ),
    },
    {
      title: 'Selfhosted',
      icon: <Storage style={{ fontSize: '48px' }} />,
      description: 'Download the binary and manage your own Donetick instance',
      button: (
        <Button
          size='lg'
          fullWidth
          startDecorator={<GitHub />}
          onClick={() => {
            window.open(
              'https://github.com/donetick/donetick/releases',
              '_blank',
            )
          }}
        >
          Github Releases
        </Button>
      ),
    },
    {
      title: 'Hassio Addon',
      icon: <AddHome style={{ fontSize: '48px' }} />,
      description:
        'Have Home Assistant? Install Donetick as a Home Assistant Addon with single click',
      button: (
        <Button
          size='lg'
          fullWidth
          startDecorator={<InstallMobile />}
          onClick={() => {
            window.open(
              'https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdonetick%2Fhassio-addons',
            )
          }}
        >
          Add Addon
        </Button>
      ),
    },
  ]
  return (
    <Box
      sx={{
        alignContent: 'center',
        textAlign: 'center',
        display: 'flex',
        flexDirection: 'column',
        mt: 2,
      }}
    >
      <Typography level='h4' mt={2} mb={4}>
        Getting Started
      </Typography>

      <Box maxWidth={'lg'} sx={{ mb: 8 }}>
        <Typography level='body-md' color='neutral'>
          You can start using Donetick in multiple ways, the easiest of which is
          to use Donetick Web so you can get started in seconds, or if you are
          into selfhosting you can download the binary and run it on your own
          server, or if you are using Home Assistant you can install Donetick as
          a Home Assistant Addon
        </Typography>
        <div data-aos-id-getting-started-container>
          <Grid container spacing={4} mt={4}>
            {information.map((info, index) => (
              <Grid item xs={12} md={4} key={index}>
                <StartOptionCard
                  icon={info.icon}
                  title={info.title}
                  description={info.description}
                  button={info.button}
                />
              </Grid>
            ))}
          </Grid>
        </div>
      </Box>
    </Box>
  )
}

export default GettingStarted
