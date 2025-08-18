import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * Custom hook for timer functionality with high-resolution timing
 * Fixes timing drift issues by using timestamps instead of interval counting
 */
const useTimer = (onTimeUpdate = () => {}) => {
  const [timerState, setTimerState] = useState('stopped') // 'stopped' | 'running' | 'paused'
  const [time, setTime] = useState(0) // Current time in seconds

  // Refs for timing calculations
  const startTimeRef = useRef(null)
  const pausedTimeRef = useRef(0)
  const intervalRef = useRef(null)
  const lastNotifiedTimeRef = useRef(0)

  // Update display and notify parent
  const updateTime = useCallback(() => {
    if (timerState === 'running' && startTimeRef.current) {
      const elapsed = Math.floor((Date.now() - startTimeRef.current) / 1000)
      const newTime = pausedTimeRef.current + elapsed

      setTime(newTime)

      // Only call onTimeUpdate when the second changes to avoid excessive calls
      if (newTime !== lastNotifiedTimeRef.current) {
        lastNotifiedTimeRef.current = newTime
        onTimeUpdate(newTime)
      }
    }
  }, [timerState, onTimeUpdate])

  // Timer effect with high-frequency updates for smooth display
  useEffect(() => {
    if (timerState === 'running') {
      intervalRef.current = setInterval(updateTime, 200)
    } else {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [timerState, updateTime])

  // Timer control functions
  const startTimer = useCallback(() => {
    const now = Date.now()
    startTimeRef.current = now
    pausedTimeRef.current = 0
    lastNotifiedTimeRef.current = 0
    setTime(0)
    setTimerState('running')
    onTimeUpdate(0)
  }, [onTimeUpdate])

  const pauseTimer = useCallback(() => {
    if (timerState === 'running' && startTimeRef.current) {
      // Calculate and store the elapsed time
      const elapsed = Math.floor((Date.now() - startTimeRef.current) / 1000)
      pausedTimeRef.current = pausedTimeRef.current + elapsed
      setTimerState('paused')
    }
  }, [timerState])

  const resumeTimer = useCallback(() => {
    if (timerState === 'paused') {
      // Reset start time for resumed session
      startTimeRef.current = Date.now()
      setTimerState('running')
    }
  }, [timerState])

  const stopTimer = useCallback(() => {
    setTimerState('stopped')
    setTime(0)
    pausedTimeRef.current = 0
    startTimeRef.current = null
    lastNotifiedTimeRef.current = 0
    onTimeUpdate(0)
  }, [onTimeUpdate])

  const resetTimer = useCallback(() => {
    stopTimer()
  }, [stopTimer])

  // Computed properties
  const isRunning = timerState === 'running'
  const isPaused = timerState === 'paused'
  const isStopped = timerState === 'stopped'

  return {
    // State
    time,
    timerState,
    isRunning,
    isPaused,
    isStopped,

    // Actions
    startTimer,
    pauseTimer,
    resumeTimer,
    stopTimer,
    resetTimer,
  }
}

export default useTimer
