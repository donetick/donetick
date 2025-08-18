import { Button } from '@mui/joy'
import { styled } from '@mui/joy/styles'

const AnimatedButton = styled(Button)(({ theme }) => ({
  transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
  transform: 'translateZ(0)', // Enable GPU acceleration
  position: 'relative',
  overflow: 'hidden',

  '&:hover:not(:disabled)': {
    transform: 'translateY(-2px) translateZ(0)',
    boxShadow: theme.shadow.lg,
  },

  '&:active:not(:disabled)': {
    transform: 'translateY(0) translateZ(0)',
    transition: 'all 0.1s cubic-bezier(0.4, 0, 0.2, 1)',
  },

  '&:focus-visible': {
    outline: '2px solid',
    outlineColor: theme.palette.primary[500],
    outlineOffset: '2px',
  },

  // Ripple effect
  '&::before': {
    content: '""',
    position: 'absolute',
    top: '50%',
    left: '50%',
    width: '0',
    height: '0',
    borderRadius: '50%',
    background: 'currentColor',
    opacity: 0.1,
    transform: 'translate(-50%, -50%)',
    transition: 'width 0.6s, height 0.6s',
  },

  '&:active::before': {
    width: '300px',
    height: '300px',
  },

  // Loading state
  '&[data-loading="true"]': {
    pointerEvents: 'none',
    position: 'relative',

    '& > *': {
      opacity: 0.6,
    },

    '&::after': {
      content: '""',
      position: 'absolute',
      top: '50%',
      left: '50%',
      width: '16px',
      height: '16px',
      border: '2px solid currentColor',
      borderTop: '2px solid transparent',
      borderRadius: '50%',
      transform: 'translate(-50%, -50%)',
      animation: 'spin 1s linear infinite',
    },
  },

  // Reduced motion support
  '@media (prefers-reduced-motion: reduce)': {
    transition: 'none',
    transform: 'none !important',

    '&:hover:not(:disabled)': {
      transform: 'none',
    },

    '&:active:not(:disabled)': {
      transform: 'none',
    },

    '&::before': {
      display: 'none',
    },
  },
}))

const SmoothButton = ({ children, loading = false, onClick, ...props }) => {
  const handleClick = event => {
    if (loading) return
    onClick?.(event)
  }

  return (
    <AnimatedButton
      {...props}
      onClick={handleClick}
      data-loading={loading}
      disabled={props.disabled || loading}
    >
      {children}
    </AnimatedButton>
  )
}

export default SmoothButton
