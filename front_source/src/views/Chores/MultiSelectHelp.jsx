import { Close, HelpOutline, Keyboard } from '@mui/icons-material'
import { Box, Button, Card, Divider, IconButton, Typography } from '@mui/joy'
import { useState } from 'react'
import FadeModal from '../../components/common/FadeModal'

const MultiSelectHelp = ({ isVisible = true }) => {
  const [isHelpOpen, setIsHelpOpen] = useState(false)

  if (!isVisible) return null

  return (
    <>
      {/* Help Button */}
      <IconButton
        size='sm'
        variant='soft'
        color='neutral'
        onClick={() => setIsHelpOpen(true)}
        sx={{
          position: 'fixed',
          bottom: 24,
          right: 24,
          zIndex: 1000,
          width: 48,
          height: 48,
          borderRadius: '50%',
          boxShadow: 'lg',
        }}
        title='Show keyboard shortcuts'
      >
        <HelpOutline />
      </IconButton>

      {/* Help Modal */}
      <FadeModal open={isHelpOpen} onClose={() => setIsHelpOpen(false)}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            mb: 2,
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Keyboard color='primary' />
            <Typography level='title-lg'>Multi-select Mode</Typography>
          </Box>
          <IconButton
            variant='plain'
            size='sm'
            onClick={() => setIsHelpOpen(false)}
          >
            <Close />
          </IconButton>
        </Box>
        <Typography level='body-md' sx={{ mb: 3, color: 'text.secondary' }}>
          Use these keyboard shortcuts to work more efficiently with multiple
          tasks:
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {/* Selection shortcuts */}
          <Card variant='soft' sx={{ p: 2 }}>
            <Typography level='title-sm' sx={{ mb: 1.5, color: 'primary.600' }}>
              Selection
            </Typography>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <ShortcutItem
                keys={['Ctrl', 'A']}
                description='Select all visible tasks'
              />
              <ShortcutItem
                keys={['Esc']}
                description='Clear selection or exit multi-select mode'
              />
            </Box>
          </Card>

          {/* Action shortcuts */}
          <Card variant='soft' sx={{ p: 2 }}>
            <Typography level='title-sm' sx={{ mb: 1.5, color: 'success.600' }}>
              Actions
            </Typography>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <ShortcutItem
                keys={['Enter']}
                description='Mark selected tasks as completed'
              />
              <ShortcutItem
                keys={['Del', '⌫']}
                description='Delete selected tasks'
              />
            </Box>
          </Card>

          {/* Interface shortcuts */}
          <Card variant='soft' sx={{ p: 2 }}>
            <Typography level='title-sm' sx={{ mb: 1.5, color: 'warning.600' }}>
              Interface
            </Typography>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <ShortcutItem
                keys={['Ctrl', 'K']}
                description='Quick add new task'
              />
            </Box>
          </Card>
        </Box>
        <Divider sx={{ my: 3 }} />
        <Box sx={{ display: 'flex', justifyContent: 'center' }}>
          <Button
            variant='soft'
            onClick={() => setIsHelpOpen(false)}
            sx={{ minWidth: 120 }}
          >
            Got it!
          </Button>
        </Box>
      </FadeModal>
    </>
  )
}

const ShortcutItem = ({ keys, description }) => (
  <Box
    sx={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 2,
    }}
  >
    <Box sx={{ flex: 1, display: 'flex', alignItems: 'center' }}>
      <Typography level='body-sm'>{description}</Typography>
    </Box>
    <Box sx={{ display: 'flex', gap: 0.5 }}>
      {keys.map((key, index) => (
        <Box
          key={index}
          sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}
        >
          {index > 0 && (
            <Typography level='body-xs' color='text.secondary'>
              +
            </Typography>
          )}
          <Box
            sx={{
              px: 1,
              py: 0.25,
              bgcolor: 'background.level2',
              borderRadius: 'sm',
              border: '1px solid',
              borderColor: 'divider',
              minWidth: 32,
              textAlign: 'center',
            }}
          >
            <Typography level='body-xs' fontWeight='bold'>
              {key}
            </Typography>
          </Box>
        </Box>
      ))}
    </Box>
  </Box>
)

export default MultiSelectHelp
