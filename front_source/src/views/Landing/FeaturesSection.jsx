import {
  AutoAwesomeMosaicOutlined,
  AutoAwesomeRounded,
  CodeRounded,
  GroupRounded,
  HistoryRounded,
  Webhook,
} from '@mui/icons-material'
import Card from '@mui/joy/Card'
import Container from '@mui/joy/Container'
import Typography from '@mui/joy/Typography'
import { styled } from '@mui/system'

const FeatureIcon = styled('div')({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  backgroundColor: '#f0f0f0', // Adjust the background color as needed
  borderRadius: '50%',
  minWidth: '60px',
  height: '60px',
  marginRight: '16px',
})

const CardData = [
  {
    title: 'Open Source & Transparent',

    description:
      'Donetick is open source software. You can view, modify, and contribute to the code on GitHub.',
    icon: CodeRounded,
  },
  {
    title: 'Circles: Your Task Hub',
    description:
      'Built with sharing in mind. Invite others a circle so you can assign tasks to each other, only seeing the tasks the should be shared.',
    icon: GroupRounded,
  },
  {
    title: 'Track Your Progress',
    description:
      "See a full history of completed tasks so you can keep track of what's been achieved!",
    icon: HistoryRounded,
  },
  {
    title: 'Automated Task Scheduling',
    description:
      'Set up tasks to repeat daily, weekly, monthly, or even specific days in specific months? Donetick has a flexible scheduling system.',
    icon: AutoAwesomeMosaicOutlined,
  },
  {
    title: 'Automated Task Assignment',
    description:
      'For shared tasks Donetick can randomly rotate assignments, or choose based on last completion or least assigned.',
    icon: AutoAwesomeRounded,
  },
  {
    title: 'Integrations & Webhooks',
    description:
      'Donetick can update things programmatically with API calls. It can integrate with IFTTT, Home Assistant or even your own services.',
    icon: Webhook,
  },
]

function Feature2({ icon: Icon, title, headline, description, index }) {
  return (
    <Card
      variant='plain'
      sx={{ textAlign: 'left', p: 2 }}
      data-aos-delay={100 * index}
      data-aos-anchor='[data-aos-id-features2-blocks]'
      data-aos='fade-up'
    >
      <div style={{ display: 'flex', alignItems: 'center' }}>
        <FeatureIcon>
          <Icon
            color='primary'
            style={{ Width: '30px', height: '30px', fontSize: '30px' }}
            stroke={1.5}
          />
        </FeatureIcon>
        <div>
          {/* Changes are within this div */}
          <Typography level='h4' mt={1} mb={0.5}>
            {title}
          </Typography>
          <Typography level='body-sm' color='neutral' lineHeight={1.4}>
            {headline}
          </Typography>
        </div>
      </div>
      <Typography level='body-md' color='neutral' lineHeight={1.6}>
        {description}
      </Typography>
    </Card>
  )
}

function FeaturesSection() {
  const features = CardData.map((feature, index) => (
    <Feature2
      icon={feature.icon}
      title={feature.title}
      // headline={feature.headline}
      description={feature.description}
      index={index}
      key={index}
    />
  ))

  return (
    <Container sx={{ textAlign: 'center' }}>
      <Typography level='h4' mt={2} mb={4}>
        Why Donetick?
      </Typography>

      <Container maxWidth={'lg'} sx={{ mb: 8 }}></Container>
      <div
        className='align-center mt-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'
        data-aos-id-features2-blocks
      >
        {features}
      </div>
    </Container>
  )
}

export default FeaturesSection
