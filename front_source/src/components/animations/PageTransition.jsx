import React, { useEffect, useRef } from 'react'
import { CSSTransition, TransitionGroup } from 'react-transition-group'
import { useLocation } from 'react-router-dom'
import './PageTransition.css'

// Route hierarchy for determining navigation direction
const routeHierarchy = {
  '/': 0,
  '/my/chores': 0,
  '/chores': 1,
  '/chores/create': 2,
  '/settings': 1,
  '/things': 1,
  '/activities': 1,
  '/points': 1,
  '/labels': 1,
  '/login': 0,
  '/signup': 1,
  '/landing': 0,
}

const getRouteLevel = pathname => {
  // Check for exact matches first
  if (routeHierarchy[pathname] !== undefined) {
    return routeHierarchy[pathname]
  }
  
  // Check for dynamic routes (e.g., /chores/123/edit)
  if (pathname.includes('/chores/') && pathname.includes('/edit')) {
    return 3
  }
  if (pathname.includes('/chores/') && pathname.includes('/history')) {
    return 3
  }
  if (pathname.includes('/chores/') && !pathname.includes('/edit') && !pathname.includes('/history')) {
    return 2
  }
  if (pathname.includes('/things/')) {
    return 2
  }
  
  // Default level
  return 1
}

const PageTransition = ({ children }) => {
  const location = useLocation()
  const prevLocation = useRef(location)
  const isNavigatingBack = useRef(false)

  useEffect(() => {
    const currentLevel = getRouteLevel(location.pathname)
    const previousLevel = getRouteLevel(prevLocation.current.pathname)
    
    // Determine if we're navigating back (to a higher level in hierarchy)
    isNavigatingBack.current = currentLevel < previousLevel
    
    prevLocation.current = location
  }, [location])

  const getTransitionClasses = () => {
    if (location.pathname.includes('/login') || location.pathname.includes('/signup') || location.pathname.includes('/landing')) {
      return 'fade' // Use fade for auth pages
    }
    
    return isNavigatingBack.current ? 'page-back' : 'page'
  }

  return (
    <TransitionGroup component={null}>
      <CSSTransition
        key={location.pathname}
        classNames={getTransitionClasses()}
        timeout={{
          enter: 300,
          exit: 200,
        }}
        unmountOnExit
      >
        <div className="page-wrapper">
          {children}
        </div>
      </CSSTransition>
    </TransitionGroup>
  )
}

export default PageTransition
