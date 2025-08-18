import { Box } from '@mui/joy'
import React, { useEffect, useState } from 'react'
import { CSSTransition, TransitionGroup } from 'react-transition-group'
import './PageTransition.css'

const StaggeredList = ({
  children,
  staggerDelay = 50,
  initialDelay = 0,
  animate = true,
}) => {
  const [isVisible, setIsVisible] = useState(!animate)

  useEffect(() => {
    if (animate) {
      const timer = setTimeout(() => {
        setIsVisible(true)
      }, initialDelay)

      return () => clearTimeout(timer)
    }
  }, [animate, initialDelay])

  if (!animate) {
    return <Box>{children}</Box>
  }

  const childrenArray = React.Children.toArray(children)

  return (
    <Box>
      <TransitionGroup component={null}>
        {isVisible &&
          childrenArray.map((child, index) => (
            <CSSTransition
              key={child.key || index}
              classNames='stagger'
              timeout={{
                enter: 300 + index * staggerDelay,
                exit: 200,
              }}
              style={{
                transitionDelay: `${index * staggerDelay}ms`,
              }}
            >
              <Box sx={{ mb: 1 }}>{child}</Box>
            </CSSTransition>
          ))}
      </TransitionGroup>
    </Box>
  )
}

export default StaggeredList
