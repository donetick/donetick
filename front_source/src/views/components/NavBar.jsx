import Logo from '@/assets/logo.svg'
import {
  AccountBox,
  History,
  HomeOutlined,
  ListAlt,
  Logout,
  MenuRounded,
  Message,
  SettingsOutlined,
  ShareOutlined,
  Toll,
  Widgets,
} from '@mui/icons-material'
import {
  Box,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemContent,
  ListItemDecorator,
  Typography,
} from '@mui/joy'

import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { version } from '../../../package.json'
import ThemeToggleButton from '../Settings/ThemeToggleButton'
import NavBarLink from './NavBarLink'
const links = [
  {
    to: '/my/chores',
    label: 'Home',
    icon: <HomeOutlined />,
  },

  // {
  //   to: '/chores',
  //   label: 'Desktop View',
  //   icon: <ListAltRounded />,
  // },
  {
    to: '/things',
    label: 'Things',
    icon: <Widgets />,
  },
  {
    to: 'labels',
    label: 'Labels',
    icon: <ListAlt />,
  },
  {
    to: 'activities',
    label: 'Activities',
    icon: <History />,
  },
  {
    to: 'points',
    label: 'Points',
    icon: <Toll />,
  },
  {
    to: '/settings#sharing',
    label: 'Sharing',
    icon: <ShareOutlined />,
  },
  {
    to: '/settings#notifications',
    label: 'Notifications',
    icon: <Message />,
  },
  {
    to: '/settings#account',
    label: 'Account',
    icon: <AccountBox />,
  },
  {
    to: '/settings',
    label: 'Settings',
    icon: <SettingsOutlined />,
  },
]

import Z_INDEX from '../../constants/zIndex'

const NavBar = () => {
  const navigate = useNavigate()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [openDrawer, closeDrawer] = [
    () => setDrawerOpen(true),
    () => setDrawerOpen(false),
  ]
  const location = useLocation()
  // if url has /landing then remove the navbar:
  if (
    ['/signup', '/login', '/landing', '/forgot-password'].includes(
      location.pathname,
    )
  ) {
    return null
  }
  if (
    location.pathname === '/' &&
    import.meta.env.VITE_IS_LANDING_DEFAULT === 'true'
  ) {
    return null
  }

  return (
    <nav
      className='mt-2 flex gap-2 p-3 pt-5'
      style={{
        paddingTop: `calc( env(safe-area-inset-top, 0px))`,
        position: 'sticky',
        zIndex: Z_INDEX.NAVBAR,
        top: 0,
        minHeight: '45px',
        backgroundColor: 'var(--joy-palette-background-body)',
      }}
    >
      <IconButton size='md' variant='plain' onClick={() => setDrawerOpen(true)}>
        <MenuRounded />
      </IconButton>
      <Box
        className='flex items-center gap-2'
        onClick={() => {
          navigate('/my/chores')
        }}
      >
        <img component='img' src={Logo} width='25' />
        <Typography
          level='title-lg'
          sx={{
            fontWeight: 700,
            fontSize: 20,
            cursor: 'pointer',
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
        <ThemeToggleButton
          sx={{
            position: 'absolute',
            right: 10,
          }}
        />
      </Box>
      <Drawer
        open={drawerOpen}
        onClose={closeDrawer}
        size='sm'
        onClick={closeDrawer}
        sx={{
          '& .MuiDrawer-content': {
            position: 'fixed',
            pt: 'calc(env(safe-area-inset-top, 0px))',
            left: 0,
            height:
              'calc(100vh - env(safe-area-inset-top, 0px) - env(safe-area-inset-bottom, 0px))',
            overflow: 'auto',
            zIndex: Z_INDEX.DRAWER,
          },
        }}
      >
        <div>
          {/* <div className='align-center flex px-5 pt-4'>
            <ModalClose size='sm' sx={{ top: 'unset', right: 20 }} />
          </div> */}
          <List
            // sx={{ p: 2, height: 'min-content' }}
            size='md'
            onClick={openDrawer}
            sx={{ borderRadius: 4, width: '100%', padding: 1 }}
          >
            {links.map((link, index) => (
              <NavBarLink key={index} link={link} />
            ))}
          </List>
        </div>
        <div>
          <List
            sx={{
              p: 2,
              height: 'min-content',
              position: 'absolute',
              bottom: 0,
              borderRadius: 4,
              width: '100%',
              padding: 2,
            }}
            size='md'
            onClick={openDrawer}
          >
            <ListItemButton
              onClick={() => {
                localStorage.removeItem('ca_token')
                localStorage.removeItem('ca_expiration')
                // go to login page:
                window.location.href = '/login'
              }}
              sx={{
                py: 1.2,
              }}
            >
              <ListItemDecorator>
                <Logout />
              </ListItemDecorator>
              <ListItemContent>Logout</ListItemContent>
            </ListItemButton>
            <Typography
              onClick={
                // force service worker to update:
                () => window.location.reload(true)
              }
              level='body-xs'
              sx={{
                // p: 2,
                p: 1,
                color: 'text.tertiary',
                textAlign: 'center',
                mb: 'calc(env(safe-area-inset-bottom, 0px) + 45px)',
                // mb: -2,
              }}
            >
              V{version}
            </Typography>
          </List>
        </div>
      </Drawer>
    </nav>
  )
}

export default NavBar
