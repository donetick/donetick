import {
  Flag,
  OpenInFull,
  Pause,
  PlayArrow,
  Schedule,
} from '@mui/icons-material'
import { Box, Card, Chip, Typography } from '@mui/joy'
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

const TimePassedCard = ({ chore, handleAction, onShowDetails }) => {
  const navigate = useNavigate()
  const [time, setTime] = useState(0)
  const [shouldAnimate, setShouldAnimate] = useState(false)
  const [prevStatus, setPrevStatus] = useState(null) // Initialize as null
  const intervalRef = useRef(null)

  // Track status changes to trigger animation
  useEffect(() => {
    // Only trigger animation if we have a previous status and it changed from 0 to 1
    if (prevStatus !== null && prevStatus === 0 && chore.status === 1) {
      setShouldAnimate(true)
      // Reset animation after it completes
      const timer = setTimeout(() => setShouldAnimate(false), 300)
      return () => clearTimeout(timer)
    }
    setPrevStatus(chore.status)
  }, [chore.status, prevStatus])

  // Single effect to handle both time calculation and timer
  useEffect(() => {
    // Calculate current time based on chore data
    const calculateCurrentTime = () => {
      if (chore.timerUpdatedAt && chore.status === 1) {
        // Active session: base duration + time since start
        const timeSinceStart = Math.floor(
          (Date.now() - new Date(chore.timerUpdatedAt).getTime()) / 1000,
        )

        return timeSinceStart + (chore.duration || 0)
      }
      // Not active: just return accumulated duration
      return chore.duration || 0
    }

    // Clear any existing timer first
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }

    // Set initial time
    const currentTime = calculateCurrentTime()
    setTime(currentTime)

    // Handle timer based on status
    if (chore.status === 1) {
      // Active: start interval timer
      intervalRef.current = setInterval(() => {
        const newTime = calculateCurrentTime()
        setTime(newTime)
      }, 1000)
    }

    // Cleanup function
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [chore.status, chore.duration, chore.timerUpdatedAt])

  const formatTime = seconds => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = seconds % 60
    return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  return (
    <Card
      variant='soft'
      sx={{
        borderRadius: 'md',
        boxShadow: 1,
        gap: 0,
        px: 2,
        py: 1,
        height: '75px',
        alignItems: 'center',
        ...(shouldAnimate && {
          animation: 'slideInUp 0.3s ease-out',
        }),
        '@keyframes slideInUp': {
          '0%': {
            opacity: 0,
            transform: 'translateY(20px) scale(0.95)',
          },
          '100%': {
            opacity: 1,
            transform: 'translateY(0) scale(1)',
          },
        },
        transition: 'all 0.3s ease',
        cursor: 'pointer',
      }}
      onClick={e => {
        // if this click on this element itself and not its children:
        if (e.target !== e.currentTarget) return
        navigate('./timer')
      }}
    >
      <OpenInFull
        sx={{
          position: 'absolute',
          top: 8,
          right: 8,
          zIndex: 2,
          cursor: 'pointer',
          color: 'text.secondary',
          fontSize: '15px',
          '&:hover': { color: 'primary.main' },
        }}
        onClick={e => {
          e.stopPropagation()
          navigate('./timer')
        }}
      ></OpenInFull>
      <Typography
        level='h4'
        sx={{
          fontWeight: 600,
          color: chore.status === 1 ? 'success.main' : 'text.primary',
          // mb: 0.5,
          mb: 0.5,
          transition: 'all 0.3s ease',
          transform: chore.status === 1 ? 'scale(1.40)' : 'scale(1)',
          cursor: 'pointer',
          '&:hover': {
            textDecoration: 'underline',
          },
        }}
        onClick={() => onShowDetails?.()}
      >
        {formatTime(time)}
      </Typography>

      {/* Status and info section */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0 }}>
        {/* Show start time and user if active */}
        {chore.status === 1 ? (
          <Chip
            variant='soft'
            color='warning'
            size='md'
            startDecorator={<Pause sx={{ fontSize: 14 }} />}
            onClick={() => {
              handleAction('pause')
            }}
          >
            Pause
          </Chip>
        ) : (
          <Chip
            variant='solid'
            color='success'
            size='md'
            startDecorator={<PlayArrow sx={{ fontSize: 14 }} />}
            onClick={() => {
              handleAction('resume')
            }}
          >
            Resume
          </Chip>
        )}

        {/* Chips for start time and current session */}
        {chore.status === 1 && chore.timerUpdatedAt && (
          <>
            {/* Original start time */}
            {chore.startTime && (
              <Chip
                variant='plain'
                color='primary'
                size='md'
                startDecorator={<Flag sx={{ fontSize: 14 }} />}
              >
                {new Date(chore.startTime).toLocaleTimeString([], {
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </Chip>
            )}

            {/* Current session start time */}
            {chore.timerUpdatedAt !== chore.startTime && (
              <Chip
                variant='plain'
                color='neutral'
                size='md'
                startDecorator={<Schedule sx={{ fontSize: 14 }} />}
              >
                {new Date(chore.timerUpdatedAt).toLocaleTimeString([], {
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </Chip>
            )}
          </>
        )}

        {/* Chips for paused state */}
        {chore.status === 2 && (
          <Chip
            variant='plain'
            color='neutral'
            size='md'
            startDecorator={<Schedule sx={{ fontSize: 14 }} />}
          >
            {new Date(chore.timerUpdatedAt).toLocaleTimeString([], {
              hour: '2-digit',
              minute: '2-digit',
            })}
          </Chip>
        )}
      </Box>
    </Card>
  )
}

export default TimePassedCard
