/* eslint-disable react/jsx-key */
import { CheckRounded } from '@mui/icons-material'
import { Box, Button, Card, Container, Typography } from '@mui/joy'
import React from 'react'
import { useNavigate } from 'react-router-dom'

const PricingSection = () => {
  const navigate = useNavigate()
  const FEATURES_FREE = [
    ['Create Tasks and Chores', <CheckRounded color='primary' />],
    ['Limited Task History', <CheckRounded color='primary' />],
    ['Circle up to two members', <CheckRounded color='primary' />],
  ]
  const FEATURES_PREMIUM = [
    ['All Basic Features', <CheckRounded color='primary' />],
    ['Hosted on DoneTick servers', <CheckRounded color='primary' />],
    ['Up to 8 Circle Members', <CheckRounded color='primary' />],
    [
      'Notification through Telegram (Discord coming soon)',
      <CheckRounded color='primary' />,
    ],
    ['Unlimited History', <CheckRounded color='primary' />],
    [
      'All circle members get the same features as the owner',
      <CheckRounded color='primary' />,
    ],
  ]
  const FEATURES_YEARLY = [
    // ['All Basic Features', <CheckRounded color='primary' />],
    // ['Up to 8 Circle Members', <CheckRounded color='primary' />],
    ['Notification through Telegram bot', <CheckRounded color='primary' />],
    ['Custom Webhook/API Integration', <CheckRounded color='primary' />],
    ['Unlimited History', <CheckRounded color='primary' />],

    ['Priority Support', <CheckRounded color='primary' />],
  ]
  const PRICEITEMS = [
    {
      title: 'Basic',
      description:
        'Hosted on Donetick servers, supports up to 2 circle members and includes all the features of the free plan.',
      price: 0,
      previousPrice: 0,
      interval: 'month',
      discount: false,
      features: FEATURES_FREE,
    },

    {
      title: 'Plus',
      description:
        // 'Supports up to 8 circle members and includes all the features of the Basic plan.',
        'Hosted on Donetick servers, supports up to 8 circle members and includes all the features of the Basic plan.',
      price: 30.0,
      //   previousPrice: 76.89,
      interval: 'year',
      //   discount: true,
      features: FEATURES_YEARLY,
    },
  ]
  return (
    <Container
      sx={{ textAlign: 'center', mb: 2 }}
      maxWidth={'lg'}
      id='pricing-tiers'
    >
      <Typography level='h4' mt={2} mb={2}>
        Pricing
      </Typography>

      <Container maxWidth={'sm'} sx={{ mb: 8 }}>
        <Typography level='body-md' color='neutral'>
          Choose the plan that works best for you.
        </Typography>
      </Container>

      <div
        className='mt-8 grid grid-cols-1 gap-2 sm:grid-cols-1 lg:grid-cols-2'
        data-aos-id-pricing
      >
        {PRICEITEMS.map((pi, index) => (
          <Card
            key={index}
            data-aos-delay={50 * (1 + index)}
            data-aos-anchor='[data-aos-id-pricing]'
            data-aos='fade-up'
            className='hover:bg-white dark:hover:bg-teal-900'
            sx={{
              textAlign: 'center',
              p: 5,
              minHeight: 400,
              // maxWidth: 400,
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
              // when top reach the top change the background color:
              '&:hover': {
                // backgroundColor: '#FFFFFF',
                boxShadow: '0px 0px 20px rgba(0, 0, 0, 0.1)',
              },
            }}
          >
            <Box
              display='flex'
              flexDirection='column'
              justifyContent='flex-start' // Updated property
              alignItems='center'
            >
              <Typography level='h2'>{pi.title}</Typography>
              <Typography level='body-md'>{pi.description}</Typography>
            </Box>
            <Box
              display='flex'
              flexDirection='column'
              justifyContent='center'
              alignItems='center'
            >
              <Box
                display='flex'
                flexDirection='row'
                alignItems='baseline'
                sx={{ my: 4 }}
              >
                {pi.discount && (
                  <Typography
                    level='h3'
                    component='span'
                    sx={{ textDecoration: 'line-through', opacity: 0.5 }}
                  >
                    ${pi.previousPrice}&nbsp;
                  </Typography>
                )}
                <Typography level='h2' component='span'>
                  ${pi.price}
                </Typography>
                <Typography level='body-md' component='span'>
                  / {pi.interval}
                </Typography>
              </Box>

              <Typography level='title-md'>Features</Typography>
              {pi.features.map(feature => (
                <Typography
                  startDecorator={feature[1]}
                  level='body-md'
                  color='neutral'
                  lineHeight={1.6}
                >
                  {feature[0]}
                </Typography>
              ))}

              {/* Here start the test */}
              <div style={{ marginTop: 'auto' }}>
                <Button
                  sx={{ mt: 5 }}
                  onClick={() => {
                    navigate('/settings#account')
                  }}
                >
                  Get Started
                </Button>
                <Typography
                  level='body-md'
                  color='neutral'
                  lineHeight={1.6}
                ></Typography>
              </div>
            </Box>
          </Card>
        ))}
      </div>

      {/* Here start the test */}
    </Container>
  )
}

export default PricingSection
