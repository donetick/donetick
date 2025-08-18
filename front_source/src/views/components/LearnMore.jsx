import { Info } from '@mui/icons-material'
import { Box, IconButton, Sheet } from '@mui/joy'
import React, { useRef, useState } from 'react'

const LearnMoreButton = ({ content }) => {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)

  const handleToggle = () => {
    setOpen(prev => !prev)
  }

  const handleClickOutside = event => {
    if (anchorRef.current && !anchorRef.current.contains(event.target)) {
      setOpen(false)
    }
  }

  React.useEffect(() => {
    if (open) {
      document.addEventListener('mousedown', handleClickOutside)
    } else {
      document.removeEventListener('mousedown', handleClickOutside)
    }
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [open])

  return (
    <Box sx={{ position: 'relative', display: 'inline-block' }}>
      <IconButton
        ref={anchorRef}
        variant='plain'
        size='sm'
        color='primary'
        onClick={handleToggle}
      >
        <Info />
      </IconButton>
      {open && (
        <Sheet
          variant='outlined'
          sx={{
            position: 'absolute',
            top: '100%',
            left: 0,
            mt: 1,
            zIndex: 1000,
            p: 2,
            borderRadius: 'sm',
            boxShadow: 'md',
            backgroundColor: 'background.surface',
            minWidth: 240,
            maxHeight: 260,
            overflowY: 'auto',
          }}
        >
          {content}
        </Sheet>
      )}
    </Box>
  )
}

export default LearnMoreButton
