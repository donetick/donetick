import React from 'react'
import { Card } from '@mui/joy'
import { styled } from '@mui/joy/styles'

const AnimatedCard = styled(Card)(({ theme }) => ({
  transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
  transform: 'translateZ(0)', // Enable GPU acceleration
  cursor: 'pointer',
  position: 'relative',
  
  '&:hover': {
    transform: 'translateY(-4px) translateZ(0)',
    boxShadow: theme.shadow.lg,
  },
  
  '&:active': {
    transform: 'translateY(-2px) translateZ(0)',
    transition: 'all 0.1s cubic-bezier(0.4, 0, 0.2, 1)',
  },
  
  // Subtle background animation on hover
  '&::before': {
    content: '""',
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'linear-gradient(45deg, transparent, rgba(255,255,255,0.1), transparent)',
    opacity: 0,
    transition: 'opacity 0.3s ease',
    pointerEvents: 'none',
    borderRadius: 'inherit',
  },
  
  '&:hover::before': {
    opacity: 1,
  },
  
  // Focus states for accessibility
  '&:focus-visible': {
    outline: '2px solid',
    outlineColor: theme.palette.primary[500],
    outlineOffset: '2px',
  },
  
  // Reduced motion support
  '@media (prefers-reduced-motion: reduce)': {
    transition: 'none',
    transform: 'none !important',
    
    '&:hover': {
      transform: 'none',
      boxShadow: theme.shadow.md, // Still provide visual feedback
    },
    
    '&:active': {
      transform: 'none',
    },
    
    '&::before': {
      display: 'none',
    },
  },
}))

const SmoothCard = ({ 
  children, 
  onClick,
  animationDisabled = false,
  ...props 
}) => {
  if (animationDisabled) {
    return (
      <Card {...props} onClick={onClick}>
        {children}
      </Card>
    )
  }

  return (
    <AnimatedCard {...props} onClick={onClick}>
      {children}
    </AnimatedCard>
  )
}

export default SmoothCard
