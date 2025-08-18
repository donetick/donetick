import LogoSVG from '@/assets/logo.svg'
import { Card, Grid } from '@mui/joy'
import Box from '@mui/joy/Box'
import Link from '@mui/joy/Link'
import Typography from '@mui/joy/Typography'
import * as React from 'react'
import { version } from '../../../package.json'

function Footer() {
  return (
    <Card
      data-aos-landing-footer
      data-aos-delay={200}
      data-aos-anchor='[data-aos-landing-footer]'
      data-aos='zoom-in-up'
    >
      <Grid
        container
        component='footer'
        sx={{
          display: 'flex',
          justifyContent: 'space-around',
          p: 4,
          // borderTop: '1px solid',
          bottom: 0,
          width: '100%',
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center' }}>
          <div className='logo'>
            <img src={LogoSVG} alt='logo' width='64px' height='64px' />
          </div>
          <Box className='flex items-center gap-2'>
            <Typography
              level='title-lg'
              sx={{
                fontWeight: 700,
                fontSize: 24,
              }}
            >
              Done
              <span
                style={{
                  color: '#06b6d4',
                  fontWeight: 600,
                }}
              >
                tick✓
              </span>
            </Typography>
          </Box>
        </Box>
        <Box>
          <Typography level='body2' fontWeight='bold' mb={1}>
            Github
          </Typography>
          <Link
            href='https://github.com/donetick/donetick'
            level='body2'
            sx={{ display: 'block' }}
          >
            Donetick
          </Link>
          <Link
            href='https://github.com/donetick/frontend'
            level='body2'
            sx={{ display: 'block' }}
          >
            Frontend
          </Link>
          <Link
            href='https://github.com/donetick/hassio-addons'
            level='body2'
            sx={{ display: 'block' }}
          >
            Home Assistant Addon
          </Link>
          <Link
            href='https://hub.docker.com/r/donetick/donetick'
            level='body2'
            sx={{ display: 'block' }}
          >
            Docker Images
          </Link>
          <Link
            href='https://github.com/donetick/donetick/releases'
            level='body2'
            sx={{ display: 'block' }}
          >
            Releases
          </Link>
        </Box>
        <Box>
          <Typography level='body2' fontWeight='bold' mb={1}>
            Links
          </Typography>
          <Link
            level='body2'
            sx={{ display: 'block' }}
            href='https://github.com/donetick/donetick/discussions'
          >
            Discussions
          </Link>
          <Link
            level='body2'
            sx={{ display: 'block' }}
            href='https://github.com/orgs/donetick/projects/3'
          >
            Roadmap
          </Link>
          <Link
            href='https://docs.donetick.com/'
            level='body2'
            sx={{ display: 'block' }}
          >
            Documentation
          </Link>
          <Link
            href='https://github.com/donetick/donetick/releases'
            level='body2'
            sx={{ display: 'block' }}
          >
            Changelog
          </Link>
          <Link disabled={true} level='body2' sx={{ display: 'block' }}>
            V{version}
          </Link>
        </Box>
        {/* <Box>
        <Typography level='body2' fontWeight='bold' mb={1}>
          Others
        </Typography>
        <Link href='#' level='body2' sx={{ display: 'block' }}>
          Telegram Integration
        </Link>
        <Link href='#' level='body2' sx={{ display: 'block' }}>
          Slash Commands
        </Link>
      </Box> */}
      </Grid>
    </Card>
  )
}

export default Footer
