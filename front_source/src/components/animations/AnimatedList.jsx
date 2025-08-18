import React from 'react'
import { Box } from '@mui/joy'
import { CSSTransition, TransitionGroup } from 'react-transition-group'
import { useStaggeredAnimation, useReducedMotion } from '../../hooks/useAnimations'
import './PageTransition.css'

const AnimatedList = ({ 
  children, 
  staggerDelay = 50,
  animationType = 'stagger', // 'stagger', 'fade', 'slide'
  direction = 'up', // 'up', 'down', 'left', 'right'
  renderItem,
  keyExtractor,
  items,
  ...boxProps 
}) => {
  // Handle both children and items patterns
  let childrenArray
  if (items && renderItem) {
    childrenArray = items.map((item, index) => 
      React.cloneElement(renderItem(item, index), { 
        key: keyExtractor ? keyExtractor(item, index) : index 
      })
    )
  } else {
    childrenArray = React.Children.toArray(children)
  }
  
  const visibleItems = useStaggeredAnimation(childrenArray.length, staggerDelay)
  const prefersReducedMotion = useReducedMotion()

  // If user prefers reduced motion, render without animations
  if (prefersReducedMotion) {
    return (
      <Box {...boxProps}>
        {items && renderItem ? childrenArray : children}
      </Box>
    )
  }

  const getAnimationClass = () => {
    switch (animationType) {
      case 'fade':
        return 'fade'
      case 'slide':
        return direction === 'up' ? 'slide-up' : 'page'
      case 'stagger':
      default:
        return 'stagger'
    }
  }

  return (
    <Box {...boxProps}>
      <TransitionGroup component={null}>
        {childrenArray.map((child, index) => {
          const isVisible = visibleItems.has(index)
          
          return (
            <CSSTransition
              key={child.key || index}
              in={isVisible}
              timeout={{
                enter: 300,
                exit: 200,
              }}
              classNames={getAnimationClass()}
              unmountOnExit={false}
            >
              <Box
                sx={{
                  opacity: isVisible ? 1 : 0,
                  transform: isVisible ? 'none' : 'translateY(20px)',
                  transition: 'opacity 0.3s ease, transform 0.3s ease',
                  transitionDelay: `${index * staggerDelay}ms`,
                }}
              >
                {child}
              </Box>
            </CSSTransition>
          )
        })}
      </TransitionGroup>
    </Box>
  )
}

export default AnimatedList
