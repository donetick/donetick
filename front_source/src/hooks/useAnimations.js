import { useEffect, useState } from 'react'

// Hook to detect user's motion preferences
export const useReducedMotion = () => {
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false)

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    setPrefersReducedMotion(mediaQuery.matches)

    const handleChange = (event) => {
      setPrefersReducedMotion(event.matches)
    }

    mediaQuery.addEventListener('change', handleChange)
    return () => mediaQuery.removeEventListener('change', handleChange)
  }, [])

  return prefersReducedMotion
}

// Hook for staggered animations
export const useStaggeredAnimation = (itemCount, delay = 50) => {
  const [visibleItems, setVisibleItems] = useState(new Set())
  const prefersReducedMotion = useReducedMotion()

  useEffect(() => {
    if (prefersReducedMotion) {
      // Show all items immediately if reduced motion is preferred
      setVisibleItems(new Set(Array.from({ length: itemCount }, (_, i) => i)))
      return
    }

    const timeouts = []
    
    // Stagger the appearance of items
    for (let i = 0; i < itemCount; i++) {
      const timeout = setTimeout(() => {
        setVisibleItems(prev => new Set([...prev, i]))
      }, i * delay)
      
      timeouts.push(timeout)
    }

    return () => {
      timeouts.forEach(clearTimeout)
    }
  }, [itemCount, delay, prefersReducedMotion])

  return visibleItems
}

// Hook for intersection observer animations
export const useInViewAnimation = (threshold = 0.1) => {
  const [isInView, setIsInView] = useState(false)
  const [element, setElement] = useState(null)

  useEffect(() => {
    if (!element) return

    const observer = new IntersectionObserver(
      ([entry]) => {
        setIsInView(entry.isIntersecting)
      },
      { threshold }
    )

    observer.observe(element)

    return () => {
      observer.unobserve(element)
    }
  }, [element, threshold])

  return [setElement, isInView]
}

// Hook for page transition context
export const usePageTransition = () => {
  const [isTransitioning, setIsTransitioning] = useState(false)

  const startTransition = () => setIsTransitioning(true)
  const endTransition = () => setIsTransitioning(false)

  return {
    isTransitioning,
    startTransition,
    endTransition,
  }
}
