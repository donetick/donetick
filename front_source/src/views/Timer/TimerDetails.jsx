import {
  AccessTime,
  Add,
  BrowseGallery,
  Delete,
  Edit,
  PauseCircle,
  PlayArrow,
} from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Container,
  FormControl,
  FormHelperText,
  Grid,
  IconButton,
  Input,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useNotification } from '../../service/NotificationProvider'
import {
  GetChoreTimer,
  PauseChore,
  StartChore,
  UpdateTimeSession,
} from '../../utils/Fetcher'

const TimerDetails = () => {
  const { choreId } = useParams()
  const [timerData, setTimerData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [editingSessions, setEditingSessions] = useState({})
  const [currentTime, setCurrentTime] = useState(new Date())
  const [timerActionLoading, setTimerActionLoading] = useState(false)
  const { showError, showSuccess } = useNotification()

  // Swipe functionality state for session cards
  const [sessionSwipeStates, setSessionSwipeStates] = useState({})
  const swipeThreshold = 80
  const maxSwipeDistance = 160
  const dragStartX = useRef(0)
  const [isDragging, setIsDragging] = useState(false)
  const [isTouchDevice, setIsTouchDevice] = useState(false)

  // Detect if device supports touch
  useEffect(() => {
    const checkTouchDevice = () => {
      setIsTouchDevice('ontouchstart' in window || navigator.maxTouchPoints > 0)
    }
    checkTouchDevice()
  }, [])

  // Fetch timer data when component mounts
  useEffect(() => {
    if (choreId) {
      fetchTimerData()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [choreId])

  // Real-time update interval for active timers
  useEffect(() => {
    let interval
    if (timerData && !timerData.endTime) {
      // Update every second if timer is active
      interval = setInterval(() => {
        setCurrentTime(new Date())
      }, 1000)
    }
    return () => {
      if (interval) clearInterval(interval)
    }
  }, [timerData])

  const fetchTimerData = async () => {
    setLoading(true)
    try {
      const response = await GetChoreTimer(choreId)
      if (response.ok) {
        const data = await response.json()
        setTimerData(data.res)
      } else {
        showError({
          title: 'Failed to fetch timer data',
          message: 'Please try again.',
        })
      }
    } catch (error) {
      showError({
        title: 'Error fetching timer data',
        message: error.message,
      })
    } finally {
      setLoading(false)
    }
  }

  const formatTime = seconds => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = seconds % 60
    return `${hours.toString().padStart(2, '0')}:${minutes
      .toString()
      .padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  const formatDuration = seconds => {
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return `${hours}h ${minutes}m`
  }

  const startEditingSession = () => {
    if (timerData) {
      setEditingSessions(prev => ({
        ...prev,
        [timerData.id]: {
          startTime: moment(timerData.startTime).format('YYYY-MM-DDTHH:mm:ss'),
          endTime: timerData.endTime
            ? moment(timerData.endTime).format('YYYY-MM-DDTHH:mm:ss')
            : '',
          duration: timerData.duration,
          formattedDuration: formatTime(timerData.duration),
          pauseLog: timerData.pauseLog || [],
        },
      }))
    }
  }

  const addPauseLogEntry = sessionId => {
    setEditingSessions(prev => ({
      ...prev,
      [sessionId]: {
        ...prev[sessionId],
        pauseLog: [
          ...prev[sessionId].pauseLog,
          {
            start: new Date().toISOString(),
            end: null,
            duration: 0,
            updatedBy: 0, // This should be current user ID
          },
        ],
      },
    }))
  }

  const updatePauseLogEntry = (sessionId, pauseIndex, field, value) => {
    setEditingSessions(prev => {
      const updatedPauseLog = prev[sessionId].pauseLog.map((pause, index) => {
        if (index === pauseIndex) {
          const updatedPause = { ...pause, [field]: value }

          // Auto-calculate duration if both start and end are present
          if (updatedPause.start && updatedPause.end) {
            const startTime = new Date(updatedPause.start)
            const endTime = new Date(updatedPause.end)
            updatedPause.duration = Math.floor((endTime - startTime) / 1000)
          }

          return updatedPause
        }
        return pause
      })

      return {
        ...prev,
        [sessionId]: {
          ...prev[sessionId],
          pauseLog: updatedPauseLog,
        },
      }
    })
  }

  const deletePauseLogEntry = (sessionId, pauseIndex) => {
    setEditingSessions(prev => ({
      ...prev,
      [sessionId]: {
        ...prev[sessionId],
        pauseLog: prev[sessionId].pauseLog.filter(
          (_, index) => index !== pauseIndex,
        ),
      },
    }))
  }

  const cancelEditingSession = sessionId => {
    setEditingSessions(prev => {
      // eslint-disable-next-line no-unused-vars
      const { [sessionId]: removed, ...rest } = prev
      return rest
    })
  }

  const saveSession = async sessionId => {
    const editingData = editingSessions[sessionId]
    if (!editingData) return

    setLoading(true)
    try {
      // Use the auto-calculated duration from the editing session
      const updateData = {
        startTime: new Date(editingData.startTime).toISOString(),
        endTime: editingData.endTime
          ? new Date(editingData.endTime).toISOString()
          : null,
        duration: editingData.duration,
        pauseLog: editingData.pauseLog,
      }

      const response = await UpdateTimeSession(choreId, sessionId, updateData)
      if (response.ok) {
        showSuccess({
          title: 'Session updated',
          message: 'Timer session has been updated successfully.',
        })
        await fetchTimerData()
        cancelEditingSession(sessionId)
      } else {
        showError({
          title: 'Failed to update session',
          message: 'Please try again.',
        })
      }
    } catch (error) {
      showError({
        title: 'Error updating session',
        message: error.message,
      })
    } finally {
      setLoading(false)
    }
  }

  // Timer control functions
  const handleStartTimer = async () => {
    setTimerActionLoading(true)
    try {
      const response = await StartChore(choreId)
      if (response.ok) {
        showSuccess({
          title: 'Timer Started',
          message: 'Work session has been started successfully.',
        })
        await fetchTimerData()
      } else {
        showError({
          title: 'Failed to start timer',
          message: 'Please try again.',
        })
      }
    } catch (error) {
      showError({
        title: 'Error starting timer',
        message: error.message,
      })
    } finally {
      setTimerActionLoading(false)
    }
  }

  const handlePauseTimer = async () => {
    setTimerActionLoading(true)
    try {
      const response = await PauseChore(choreId)
      if (response.ok) {
        showSuccess({
          title: 'Timer Paused',
          message: 'Work session has been paused.',
        })
        await fetchTimerData()
      } else {
        showError({
          title: 'Failed to pause timer',
          message: 'Please try again.',
        })
      }
    } catch (error) {
      showError({
        title: 'Error pausing timer',
        message: error.message,
      })
    } finally {
      setTimerActionLoading(false)
    }
  }

  // Determine if timer is currently running
  const isTimerRunning = () => {
    if (!timerData || !timerData.pauseLog) return false
    return timerData.pauseLog.some(session => session.start && !session.end)
  }

  // Calculate total duration from start to now/end (real-time)
  const calculateTotalDuration = () => {
    if (!timerData) return 0

    const startTime = new Date(timerData.startTime)
    const endTime = timerData.endTime
      ? new Date(timerData.endTime)
      : currentTime

    return Math.floor((endTime - startTime) / 1000) // in seconds
  }

  // Calculate current active duration (including ongoing session) (real-time)
  const calculateCurrentActiveDuration = () => {
    if (!timerData || !timerData.pauseLog) return 0

    let totalActive = 0
    const now = currentTime

    timerData.pauseLog.forEach(session => {
      if (session.start && session.end) {
        // Completed session
        totalActive += Math.floor(
          (new Date(session.end) - new Date(session.start)) / 1000,
        )
      } else if (session.start && !session.end) {
        // Ongoing session - real-time calculation
        totalActive += Math.floor((now - new Date(session.start)) / 1000)
      }
    })

    return totalActive
  }

  // Calculate idle time (total time minus active time) (real-time)
  const calculateIdleTime = () => {
    const totalDuration = calculateTotalDuration()
    const activeDuration = calculateCurrentActiveDuration()

    return Math.max(0, totalDuration - activeDuration)
  }

  // Swipe functionality methods
  const getSessionSwipeState = sessionIndex => {
    return (
      sessionSwipeStates[sessionIndex] || {
        translateX: 0,
        isRevealed: false,
      }
    )
  }

  const updateSessionSwipeState = (sessionIndex, newState) => {
    setSessionSwipeStates(prev => ({
      ...prev,
      [sessionIndex]: {
        ...prev[sessionIndex],
        ...newState,
      },
    }))
  }

  const resetSessionSwipe = sessionIndex => {
    updateSessionSwipeState(sessionIndex, {
      translateX: 0,
      isRevealed: false,
    })
  }

  const resetAllSwipes = () => {
    setSessionSwipeStates({})
  }

  // Touch handlers for swipe
  const handleSessionTouchStart = e => {
    dragStartX.current = e.touches[0].clientX
    setIsDragging(true)
  }

  const handleSessionTouchMove = (e, sessionIndex) => {
    if (!isDragging) return

    const currentX = e.touches[0].clientX
    const deltaX = currentX - dragStartX.current
    const currentState = getSessionSwipeState(sessionIndex)

    if (currentState.isRevealed) {
      if (deltaX > 0) {
        const clampedDelta = Math.min(deltaX - maxSwipeDistance, 0)
        updateSessionSwipeState(sessionIndex, { translateX: clampedDelta })
      }
    } else {
      if (deltaX < 0) {
        const clampedDelta = Math.max(deltaX, -maxSwipeDistance)
        updateSessionSwipeState(sessionIndex, { translateX: clampedDelta })
      }
    }
  }

  const handleSessionTouchEnd = (e, sessionIndex) => {
    if (!isDragging) return
    setIsDragging(false)

    const currentState = getSessionSwipeState(sessionIndex)

    if (currentState.isRevealed) {
      if (currentState.translateX > -swipeThreshold) {
        resetSessionSwipe(sessionIndex)
      } else {
        updateSessionSwipeState(sessionIndex, {
          translateX: -maxSwipeDistance,
          isRevealed: true,
        })
      }
    } else {
      if (Math.abs(currentState.translateX) > swipeThreshold) {
        updateSessionSwipeState(sessionIndex, {
          translateX: -maxSwipeDistance,
          isRevealed: true,
        })
      } else {
        resetSessionSwipe(sessionIndex)
      }
    }
  }

  // Mouse handlers for swipe (desktop)
  const handleSessionMouseDown = e => {
    dragStartX.current = e.clientX
    setIsDragging(true)
  }

  const handleSessionMouseMove = (e, sessionIndex) => {
    if (!isDragging) return

    const currentX = e.clientX
    const deltaX = currentX - dragStartX.current
    const currentState = getSessionSwipeState(sessionIndex)

    if (currentState.isRevealed) {
      if (deltaX > 0) {
        const clampedDelta = Math.min(deltaX - maxSwipeDistance, 0)
        updateSessionSwipeState(sessionIndex, { translateX: clampedDelta })
      }
    } else {
      if (deltaX < 0) {
        const clampedDelta = Math.max(deltaX, -maxSwipeDistance)
        updateSessionSwipeState(sessionIndex, { translateX: clampedDelta })
      }
    }
  }

  const handleSessionMouseUp = (e, sessionIndex) => {
    if (!isDragging) return
    setIsDragging(false)

    const currentState = getSessionSwipeState(sessionIndex)

    if (currentState.isRevealed) {
      if (currentState.translateX > -swipeThreshold) {
        resetSessionSwipe(sessionIndex)
      } else {
        updateSessionSwipeState(sessionIndex, {
          translateX: -maxSwipeDistance,
          isRevealed: true,
        })
      }
    } else {
      if (Math.abs(currentState.translateX) > swipeThreshold) {
        updateSessionSwipeState(sessionIndex, {
          translateX: -maxSwipeDistance,
          isRevealed: true,
        })
      } else {
        resetSessionSwipe(sessionIndex)
      }
    }
  }

  const handleEditSession = () => {
    resetAllSwipes()
    // Trigger the existing edit functionality
    startEditingSession()
  }

  const handleDeleteSession = sessionIndex => {
    resetAllSwipes()
    // For now, just show an alert since we'd need to implement session deletion API
    showError({
      title: 'Delete Session',
      message: `Session #${sessionIndex + 1} deletion would be implemented here`,
    })
  }

  // Reset swipes when editing mode changes
  useEffect(() => {
    resetAllSwipes()
  }, [editingSessions])

  return (
    <Container maxWidth='lg' sx={{ py: 2 }}>
      {/* Header */}

      {loading && (
        <Alert color='neutral' sx={{ mb: 2 }}>
          Loading timer data...
        </Alert>
      )}

      {!loading && !timerData && (
        <Alert color='warning' sx={{ mb: 2 }}>
          No timer data found for this chore.
        </Alert>
      )}

      {!loading && timerData && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {/* Timer Summary */}
          <Card
            variant='plain'
            sx={{
              p: 0,
            }}
          >
            {/* Stats Grid */}
            <Grid container spacing={2} sx={{ mb: 3 }}>
              {/* Active Time */}
              <Grid item xs={6} sm={6} md={3}>
                <Card
                  variant='soft'
                  sx={{
                    borderRadius: 'md',
                    boxShadow: 1,
                    px: 2,
                    py: 1,
                    minHeight: 90,
                    height: '100%',
                    justifyContent: 'start',
                  }}
                >
                  <CardContent>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'start',
                        mb: 0.5,
                      }}
                    >
                      <PlayArrow
                        sx={{
                          fontSize: 16,
                          mr: 1,
                        }}
                      />
                      <Typography
                        level='body-md'
                        sx={{
                          fontWeight: '500',
                          color: 'text.primary',
                        }}
                      >
                        Active Work
                      </Typography>
                    </Box>
                    <Box>
                      <Typography
                        level='h4'
                        color='success'
                        sx={{
                          fontWeight: 'bold',
                          lineHeight: 1.5,
                        }}
                      >
                        {formatDuration(calculateCurrentActiveDuration())}
                      </Typography>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>

              {/* Idle Time */}
              <Grid item xs={6} sm={6} md={3}>
                <Card
                  variant='soft'
                  sx={{
                    borderRadius: 'md',
                    boxShadow: 1,
                    px: 2,
                    py: 1,
                    minHeight: 90,
                    height: '100%',
                    justifyContent: 'start',
                  }}
                >
                  <CardContent>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'start',
                        mb: 0.5,
                      }}
                    >
                      <PauseCircle
                        sx={{
                          fontSize: 16,
                          mr: 1,
                        }}
                      />
                      <Typography
                        level='body-md'
                        sx={{
                          fontWeight: '500',
                          color: 'text.primary',
                        }}
                      >
                        Break Time
                      </Typography>
                    </Box>
                    <Box>
                      <Typography
                        level='h4'
                        color='warning'
                        sx={{
                          fontWeight: 'bold',
                          lineHeight: 1.5,
                        }}
                      >
                        {formatDuration(calculateIdleTime())}
                      </Typography>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>

              {/* Total Sessions */}
              <Grid item xs={6} sm={6} md={3}>
                <Card
                  variant='soft'
                  sx={{
                    borderRadius: 'md',
                    boxShadow: 1,
                    px: 2,
                    py: 1,
                    minHeight: 90,
                    height: '100%',
                    justifyContent: 'start',
                  }}
                >
                  <CardContent>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'start',
                        mb: 0.5,
                      }}
                    >
                      <BrowseGallery
                        sx={{
                          fontSize: 16,
                          mr: 1,
                        }}
                      />
                      <Typography
                        level='body-md'
                        sx={{
                          fontWeight: '500',
                          color: 'text.primary',
                        }}
                      >
                        Sessions
                      </Typography>
                    </Box>
                    <Box>
                      <Typography
                        level='h4'
                        sx={{
                          color: 'text.secondary',
                          fontWeight: 'bold',
                          lineHeight: 1.5,
                        }}
                      >
                        {timerData.pauseLog?.length || 0}
                      </Typography>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>

              {/* Total Session Time */}
              <Grid item xs={6} sm={6} md={3}>
                <Card
                  variant='soft'
                  sx={{
                    borderRadius: 'md',
                    boxShadow: 1,
                    px: 2,
                    py: 1,
                    minHeight: 90,
                    height: '100%',
                    justifyContent: 'start',
                  }}
                >
                  <CardContent>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'start',
                        mb: 0.5,
                      }}
                    >
                      <AccessTime
                        sx={{
                          fontSize: 16,
                          mr: 1,
                        }}
                      />
                      <Typography
                        level='body-md'
                        sx={{
                          fontWeight: '500',
                          color: 'text.primary',
                        }}
                      >
                        Total Time
                      </Typography>
                    </Box>
                    <Box>
                      <Typography
                        level='h4'
                        sx={{
                          color: 'text.secondary',
                          fontWeight: 'bold',
                          lineHeight: 1.5,
                        }}
                      >
                        {formatTime(calculateTotalDuration())}
                      </Typography>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>
            </Grid>

            {/* Progress Bar */}
            <Box>
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  mb: 1,
                }}
              >
                <Typography
                  level='body-sm'
                  sx={{ color: 'text.secondary', fontWeight: 'medium' }}
                >
                  Work vs Break Distribution
                </Typography>
                <Typography level='body-sm' sx={{ color: 'text.tertiary' }}>
                  {calculateCurrentActiveDuration() > 0
                    ? `${Math.round((calculateCurrentActiveDuration() / calculateTotalDuration()) * 100)}% active`
                    : 'No active time yet'}
                </Typography>
              </Box>
              <Box
                sx={{
                  height: 8,
                  backgroundColor: 'neutral.200',
                  borderRadius: 'sm',
                  overflow: 'hidden',
                  position: 'relative',
                }}
              >
                <Box
                  sx={{
                    height: '100%',
                    width: `${Math.round((calculateCurrentActiveDuration() / Math.max(calculateTotalDuration(), 1)) * 100)}%`,
                    backgroundColor: 'success.400',
                    borderRadius: 'sm',
                    transition: 'width 0.3s ease-in-out',
                  }}
                />
              </Box>

              {/* Timeline Graph */}
              <Box sx={{ mt: 3 }}>
                <Typography
                  level='body-sm'
                  sx={{ color: 'text.secondary', fontWeight: 'medium', mb: 2 }}
                >
                  Activity Timeline
                </Typography>

                {timerData &&
                timerData.pauseLog &&
                timerData.pauseLog.length > 0 ? (
                  <Box>
                    {/* Timeline visualization */}
                    <Box
                      sx={{
                        height: 40,
                        backgroundColor: 'neutral.100',
                        borderRadius: 'sm',
                        position: 'relative',
                        overflow: 'hidden',
                        border: '1px solid',
                        borderColor: 'divider',
                        mb: 2,
                      }}
                    >
                      {(() => {
                        const totalDuration = calculateTotalDuration()
                        const startTime = new Date(timerData.startTime)

                        return timerData.pauseLog.map((session, index) => {
                          const sessionStart = new Date(session.start)
                          const sessionEnd = session.end
                            ? new Date(session.end)
                            : currentTime

                          // Calculate position and width as percentages
                          const startOffset = Math.max(
                            0,
                            (sessionStart - startTime) / 1000,
                          )
                          const sessionDuration = Math.max(
                            0,
                            (sessionEnd - sessionStart) / 1000,
                          )

                          const leftPercent =
                            (startOffset / Math.max(totalDuration, 1)) * 100
                          const widthPercent =
                            (sessionDuration / Math.max(totalDuration, 1)) * 100

                          const isOngoing = !session.end

                          return (
                            <Box
                              key={index}
                              sx={{
                                position: 'absolute',
                                left: `${leftPercent}%`,
                                width: `${widthPercent}%`,
                                height: '100%',
                                backgroundColor: isOngoing
                                  ? 'success.500'
                                  : 'success.400',
                                borderRight: isOngoing ? '2px solid' : 'none',
                                borderRightColor: 'success.600',
                                transition: 'all 0.3s ease-in-out',
                                '&:hover': {
                                  backgroundColor: isOngoing
                                    ? 'success.600'
                                    : 'success.500',
                                  zIndex: 1,
                                },
                              }}
                              title={`Session ${index + 1}: ${formatDuration(sessionDuration)} ${isOngoing ? '(ongoing)' : ''}`}
                            />
                          )
                        })
                      })()}
                    </Box>

                    {/* Legend and time markers */}
                    <Box
                      sx={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: 2,
                      }}
                    >
                      {/* Legend */}
                      <Box
                        sx={{ display: 'flex', gap: 2, alignItems: 'center' }}
                      >
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 0.5,
                          }}
                        >
                          <Box
                            sx={{
                              width: 12,
                              height: 12,
                              backgroundColor: 'success.400',
                              borderRadius: 'xs',
                            }}
                          />
                          <Typography
                            level='body-xs'
                            sx={{ color: 'text.tertiary' }}
                          >
                            Active Work
                          </Typography>
                        </Box>
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 0.5,
                          }}
                        >
                          <Box
                            sx={{
                              width: 12,
                              height: 12,
                              backgroundColor: 'neutral.100',
                              borderRadius: 'xs',
                              border: '1px solid',
                              borderColor: 'divider',
                            }}
                          />
                          <Typography
                            level='body-xs'
                            sx={{ color: 'text.tertiary' }}
                          >
                            Break Time
                          </Typography>
                        </Box>
                        {isTimerRunning() && (
                          <Box
                            sx={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: 0.5,
                            }}
                          >
                            <Box
                              sx={{
                                width: 12,
                                height: 12,
                                backgroundColor: 'success.500',
                                borderRadius: 'xs',
                                border: '2px solid',
                                borderColor: 'success.600',
                              }}
                            />
                            <Typography
                              level='body-xs'
                              sx={{ color: 'text.tertiary' }}
                            >
                              Live Session
                            </Typography>
                          </Box>
                        )}
                      </Box>

                      {/* Time markers */}
                      <Box
                        sx={{ display: 'flex', gap: 2, alignItems: 'center' }}
                      >
                        <Typography
                          level='body-xs'
                          sx={{ color: 'text.tertiary' }}
                        >
                          Started: {moment(timerData.startTime).format('HH:mm')}
                        </Typography>
                        {timerData.endTime && (
                          <Typography
                            level='body-xs'
                            sx={{ color: 'text.tertiary' }}
                          >
                            Ended: {moment(timerData.endTime).format('HH:mm')}
                          </Typography>
                        )}
                        {!timerData.endTime && (
                          <Typography
                            level='body-xs'
                            sx={{ color: 'success.500' }}
                          >
                            Now: {moment(currentTime).format('HH:mm')}
                          </Typography>
                        )}
                        <Typography
                          level='body-xs'
                          sx={{ color: 'text.tertiary' }}
                        >
                          Active:{' '}
                          {calculateCurrentActiveDuration() > 0
                            ? `${Math.round((calculateCurrentActiveDuration() / calculateTotalDuration()) * 100)}%`
                            : '0%'}
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                ) : (
                  <Alert color='neutral' variant='soft' sx={{ py: 2 }}>
                    <Typography level='body-sm'>
                      No activity timeline available. Start working to see your
                      activity pattern.
                    </Typography>
                  </Alert>
                )}
              </Box>
            </Box>
          </Card>

          {/* Session Breakdown */}
          <Box sx={{ mt: 2 }}>
            <Box
              sx={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                mb: 2,
              }}
            >
              <Typography level='h4'>Session Breakdown</Typography>
              {!editingSessions[timerData.id] && (
                <Button
                  variant='outlined'
                  color='primary'
                  startDecorator={<Edit />}
                  onClick={() => startEditingSession()}
                  size='sm'
                >
                  Edit
                </Button>
              )}
              {editingSessions[timerData.id] && (
                <Box sx={{ display: 'flex', gap: 1 }}>
                  <Button
                    variant='outlined'
                    onClick={() => cancelEditingSession(timerData.id)}
                    size='sm'
                  >
                    Cancel
                  </Button>
                  <Button
                    variant='solid'
                    color='primary'
                    onClick={() => saveSession(timerData.id)}
                    loading={loading}
                    size='sm'
                  >
                    Save Changes
                  </Button>
                </Box>
              )}
            </Box>

            {!editingSessions[timerData.id] ? (
              <Box>
                {/* Read-only view */}
                {timerData.pauseLog && timerData.pauseLog.length > 0 && (
                  <Box>
                    <Typography
                      level='body-md'
                      sx={{ fontWeight: 'bold', mb: 2 }}
                    >
                      Work Sessions ({timerData.pauseLog.length})
                    </Typography>

                    <Box
                      sx={{
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 1.5,
                      }}
                    >
                      {timerData.pauseLog
                        .sort((a, b) => moment(b.start) - moment(a.start))
                        .map((pause, pauseIndex) => {
                          const isOngoing = !pause.end
                          const sessionDate = moment(pause.start).format(
                            'MMM DD',
                          )
                          const startTime = moment(pause.start).format('HH:mm')
                          const endTime = pause.end
                            ? moment(pause.end).format('HH:mm')
                            : null

                          const realTimeDuration = isOngoing
                            ? Math.max(
                                0,
                                Math.floor(
                                  (currentTime - new Date(pause.start)) / 1000,
                                ),
                              )
                            : pause.duration

                          const swipeState = getSessionSwipeState(pauseIndex)

                          return (
                            <Box
                              key={pauseIndex}
                              sx={{
                                position: 'relative',
                                overflow: 'hidden',
                                borderRadius: 'md',
                              }}
                            >
                              {/* Action buttons underneath (revealed on swipe) */}
                              <Box
                                sx={{
                                  position: 'absolute',
                                  right: 0,
                                  top: 0,
                                  bottom: 0,
                                  width: maxSwipeDistance,
                                  display: 'flex',
                                  alignItems: 'center',
                                  boxShadow: 'inset 2px 0 4px rgba(0,0,0,0.06)',
                                  zIndex: 0,
                                }}
                              >
                                <IconButton
                                  variant='soft'
                                  color='primary'
                                  size='sm'
                                  onClick={e => {
                                    e.stopPropagation()
                                    handleEditSession()
                                  }}
                                  sx={{
                                    width: 40,
                                    height: 40,
                                    mx: 1,
                                  }}
                                >
                                  <Edit sx={{ fontSize: 16 }} />
                                </IconButton>

                                <IconButton
                                  variant='soft'
                                  color='danger'
                                  size='sm'
                                  onClick={e => {
                                    e.stopPropagation()
                                    handleDeleteSession(pauseIndex)
                                  }}
                                  sx={{
                                    width: 40,
                                    height: 40,
                                    mx: 1,
                                  }}
                                >
                                  <Delete sx={{ fontSize: 16 }} />
                                </IconButton>
                              </Box>

                              {/* Session Card */}
                              <Card
                                variant='soft'
                                sx={{
                                  p: 2,
                                  display: 'flex',
                                  flexDirection: 'row',
                                  alignItems: 'center',
                                  gap: 2,
                                  minHeight: 'auto',
                                  borderColor: isOngoing
                                    ? 'success.300'
                                    : 'divider',
                                  position: 'relative',
                                  transform: `translateX(${swipeState.translateX}px)`,
                                  transition: isDragging
                                    ? 'none'
                                    : 'transform 0.3s ease-out',
                                  zIndex: 1,
                                  cursor: 'pointer',
                                  '&:hover': {
                                    bgcolor: swipeState.isRevealed
                                      ? 'background.surface'
                                      : 'background.level1',
                                  },
                                }}
                                onClick={() => {
                                  if (swipeState.isRevealed) {
                                    resetSessionSwipe(pauseIndex)
                                    return
                                  }
                                  // Optional: Navigate to session details
                                }}
                                onTouchStart={handleSessionTouchStart}
                                onTouchMove={e =>
                                  handleSessionTouchMove(e, pauseIndex)
                                }
                                onTouchEnd={e =>
                                  handleSessionTouchEnd(e, pauseIndex)
                                }
                                onMouseDown={handleSessionMouseDown}
                                onMouseMove={e =>
                                  handleSessionMouseMove(e, pauseIndex)
                                }
                                onMouseUp={e =>
                                  handleSessionMouseUp(e, pauseIndex)
                                }
                              >
                                {/* Session indicator */}
                                <Box
                                  sx={{
                                    width: 8,
                                    height: 8,
                                    borderRadius: '50%',
                                    backgroundColor: isOngoing
                                      ? 'success.500'
                                      : 'neutral.400',
                                    flexShrink: 0,
                                  }}
                                />

                                {/* Duration - Main focus */}
                                <Box sx={{ flexShrink: 0 }}>
                                  <Typography
                                    level='h4'
                                    sx={{
                                      fontWeight: 'bold',
                                      color: isOngoing
                                        ? 'success.600'
                                        : 'text.primary',
                                      lineHeight: 1,
                                      mb: 0.3,
                                    }}
                                  >
                                    {formatDuration(realTimeDuration)}
                                  </Typography>
                                  {isOngoing && (
                                    <Chip
                                      size='sm'
                                      color='success'
                                      variant='soft'
                                      sx={{ fontSize: '0.7rem' }}
                                    >
                                      Live
                                    </Chip>
                                  )}
                                </Box>

                                {/* Session details */}
                                <Box
                                  sx={{
                                    flex: 1,
                                    minWidth: 0,
                                    textAlign: 'right',
                                  }}
                                >
                                  <Typography
                                    level='body-sm'
                                    sx={{
                                      fontWeight: 'medium',
                                      color: 'text.secondary',
                                      mb: 0.2,
                                    }}
                                  >
                                    Session #{pauseIndex + 1} • {sessionDate}
                                  </Typography>
                                  <Typography
                                    level='body-xs'
                                    sx={{
                                      color: 'text.tertiary',
                                      fontFamily: 'monospace',
                                    }}
                                  >
                                    {startTime}{' '}
                                    {endTime ? `→ ${endTime}` : '→ ongoing'}
                                  </Typography>
                                </Box>

                                {/* Right drag indicator (desktop only) */}
                                {!isTouchDevice && (
                                  <Box
                                    sx={{
                                      position: 'absolute',
                                      right: 8,
                                      top: '50%',
                                      transform: 'translateY(-50%)',
                                      width: '20px',
                                      height: '20px',
                                      display: 'flex',
                                      alignItems: 'center',
                                      justifyContent: 'center',
                                      opacity: swipeState.isRevealed ? 0 : 0.3,
                                      transition: 'opacity 0.2s ease',
                                      pointerEvents: swipeState.isRevealed
                                        ? 'none'
                                        : 'auto',
                                      '&:hover': {
                                        opacity: swipeState.isRevealed
                                          ? 0
                                          : 0.7,
                                      },
                                    }}
                                  >
                                    {/* Drag indicator dots */}
                                    <Box
                                      sx={{
                                        display: 'flex',
                                        flexDirection: 'column',
                                        gap: 0.25,
                                      }}
                                    >
                                      {[...Array(3)].map((_, i) => (
                                        <Box
                                          key={i}
                                          sx={{
                                            width: 3,
                                            height: 3,
                                            borderRadius: '50%',
                                            bgcolor: 'text.tertiary',
                                          }}
                                        />
                                      ))}
                                    </Box>
                                  </Box>
                                )}
                              </Card>
                            </Box>
                          )
                        })}
                    </Box>
                  </Box>
                )}

                {(!timerData.pauseLog || timerData.pauseLog.length === 0) && (
                  <Alert color='neutral'>
                    No work sessions found for this timer.
                  </Alert>
                )}
              </Box>
            ) : (
              <Box>
                {/* Editing view */}
                <Box
                  sx={{
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 2,
                  }}
                >
                  {/* Session Editor */}
                  <Box>
                    <Box
                      sx={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        mb: 2,
                      }}
                    >
                      <Typography level='body-md' sx={{ fontWeight: 'bold' }}>
                        Sessions
                      </Typography>
                      <Button
                        size='sm'
                        variant='outlined'
                        startDecorator={<Add />}
                        onClick={() => addPauseLogEntry(timerData.id)}
                      >
                        Add Session
                      </Button>
                    </Box>

                    {editingSessions[timerData.id].pauseLog.map(
                      (pause, pauseIndex) => (
                        <Card
                          key={pauseIndex}
                          variant='soft'
                          sx={{ mb: 2, p: 2 }}
                        >
                          <Box
                            sx={{
                              display: 'flex',
                              justifyContent: 'space-between',
                              alignItems: 'center',
                              mb: 2,
                            }}
                          >
                            <Typography
                              level='body-md'
                              sx={{ fontWeight: 'bold' }}
                            >
                              Session #{pauseIndex + 1}
                            </Typography>
                            <Button
                              size='sm'
                              variant='outlined'
                              color='danger'
                              onClick={() =>
                                deletePauseLogEntry(timerData.id, pauseIndex)
                              }
                            >
                              <Delete />
                            </Button>
                          </Box>

                          <Box
                            sx={{
                              display: 'grid',
                              gridTemplateColumns:
                                'repeat(auto-fit, minmax(250px, 1fr))',
                              gap: 2,
                            }}
                          >
                            <FormControl size='sm'>
                              <Typography
                                level='body-sm'
                                sx={{ fontWeight: 'bold', mb: 1 }}
                              >
                                Start Time
                              </Typography>
                              <Input
                                type='datetime-local'
                                value={moment(pause.start).format(
                                  'YYYY-MM-DDTHH:mm:ss',
                                )}
                                onChange={e =>
                                  updatePauseLogEntry(
                                    timerData.id,
                                    pauseIndex,
                                    'start',
                                    new Date(e.target.value).toISOString(),
                                  )
                                }
                              />
                            </FormControl>

                            <FormControl size='sm'>
                              <Typography
                                level='body-sm'
                                sx={{ fontWeight: 'bold', mb: 1 }}
                              >
                                End Time
                              </Typography>
                              <Input
                                type='datetime-local'
                                value={
                                  pause.end
                                    ? moment(pause.end).format(
                                        'YYYY-MM-DDTHH:mm:ss',
                                      )
                                    : ''
                                }
                                onChange={e =>
                                  updatePauseLogEntry(
                                    timerData.id,
                                    pauseIndex,
                                    'end',
                                    e.target.value
                                      ? new Date(e.target.value).toISOString()
                                      : null,
                                  )
                                }
                              />
                              <FormHelperText>
                                Leave empty if session is ongoing
                              </FormHelperText>
                            </FormControl>

                            <Box>
                              <Typography
                                level='body-sm'
                                sx={{ fontWeight: 'bold', mb: 1 }}
                              >
                                Duration (Auto-calculated)
                              </Typography>
                              <Typography
                                level='body-sm'
                                sx={{
                                  p: 1.5,
                                  bgcolor: 'background.surface',
                                  borderRadius: 'sm',
                                  border: '1px solid',
                                  borderColor: 'divider',
                                }}
                              >
                                {formatDuration(pause.duration)} (
                                {pause.duration}s)
                              </Typography>
                            </Box>
                          </Box>
                        </Card>
                      ),
                    )}
                  </Box>
                </Box>
              </Box>
            )}
          </Box>
        </Box>
      )}

      {/* Floating Timer Control Button */}
      {!loading && timerData && (
        <IconButton
          color={isTimerRunning() ? 'warning' : 'success'}
          variant='solid'
          onClick={isTimerRunning() ? handlePauseTimer : handleStartTimer}
          loading={timerActionLoading}
          disabled={loading}
          sx={{
            position: 'fixed',
            bottom: 16,
            left: 16,
            width: 56,
            height: 56,
            borderRadius: '50%',
            zIndex: 1000,
            boxShadow: 'lg',
            transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
            transform: timerActionLoading ? 'scale(0.95)' : 'scale(1)',
            animation: isTimerRunning()
              ? 'pulse-warning 2s infinite'
              : 'pulse-success 2s infinite',
            '&:hover': {
              transform: 'scale(1.1)',
              boxShadow: 'xl',
            },
            '&:active': {
              transform: 'scale(0.95)',
            },
            '@keyframes pulse-success': {
              '0%': {
                boxShadow:
                  '0 4px 12px rgba(76, 175, 80, 0.3), 0 0 0 0 rgba(76, 175, 80, 0.7)',
              },
              '70%': {
                boxShadow:
                  '0 4px 12px rgba(76, 175, 80, 0.3), 0 0 0 10px rgba(76, 175, 80, 0)',
              },
              '100%': {
                boxShadow:
                  '0 4px 12px rgba(76, 175, 80, 0.3), 0 0 0 0 rgba(76, 175, 80, 0)',
              },
            },
            '@keyframes pulse-warning': {
              '0%': {
                boxShadow:
                  '0 4px 12px rgba(255, 152, 0, 0.3), 0 0 0 0 rgba(255, 152, 0, 0.7)',
              },
              '70%': {
                boxShadow:
                  '0 4px 12px rgba(255, 152, 0, 0.3), 0 0 0 10px rgba(255, 152, 0, 0)',
              },
              '100%': {
                boxShadow:
                  '0 4px 12px rgba(255, 152, 0, 0.3), 0 0 0 0 rgba(255, 152, 0, 0)',
              },
            },
          }}
          title={isTimerRunning() ? 'Pause Timer' : 'Start Timer'}
        >
          {isTimerRunning() ? (
            <PauseCircle sx={{ fontSize: 24 }} />
          ) : (
            <PlayArrow sx={{ fontSize: 24 }} />
          )}
        </IconButton>
      )}
    </Container>
  )
}

export default TimerDetails
