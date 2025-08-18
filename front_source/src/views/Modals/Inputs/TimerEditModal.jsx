import { Add, Delete, Edit } from '@mui/icons-material'
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  FormControl,
  FormHelperText,
  Input,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useEffect, useState } from 'react'
import FadeModal from '../../../components/common/FadeModal'
import { useNotification } from '../../../service/NotificationProvider'
import {
  DeleteTimeSession,
  GetChoreTimer,
  UpdateTimeSession,
} from '../../../utils/Fetcher'
import ConfirmationModal from './ConfirmationModal'

const TimerEditModal = ({ isOpen, onClose, choreId, onTimerUpdate }) => {
  const [timerData, setTimerData] = useState(null)
  const [loading, setLoading] = useState(false)
  const [editingSessions, setEditingSessions] = useState({})
  const [confirmDeleteConfig, setConfirmDeleteConfig] = useState({})
  const [currentTime, setCurrentTime] = useState(new Date())
  const { showError, showSuccess } = useNotification()

  // Fetch timer data when modal opens
  useEffect(() => {
    if (isOpen && choreId) {
      fetchTimerData()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, choreId])

  // Real-time update interval for active timers
  useEffect(() => {
    let interval
    if (isOpen && timerData && !timerData.endTime) {
      // Update every second if timer is active
      interval = setInterval(() => {
        setCurrentTime(new Date())
      }, 1000)
    }
    return () => {
      if (interval) clearInterval(interval)
    }
  }, [isOpen, timerData])

  const fetchTimerData = async () => {
    setLoading(true)
    try {
      const response = await GetChoreTimer(choreId)
      if (response.ok) {
        const data = await response.json()
        setTimerData(data.res) // data.res is the timer session object
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
        onTimerUpdate?.()
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

  const deleteSession = async sessionId => {
    setLoading(true)
    try {
      const response = await DeleteTimeSession(choreId, sessionId)
      if (response.ok) {
        showSuccess({
          title: 'Session deleted',
          message: 'Timer session has been deleted successfully.',
        })
        await fetchTimerData()
        onTimerUpdate?.()
      } else {
        showError({
          title: 'Failed to delete session',
          message: 'Please try again.',
        })
      }
    } catch (error) {
      showError({
        title: 'Error deleting session',
        message: error.message,
      })
    } finally {
      setLoading(false)
    }
  }

  const confirmDeleteSession = sessionId => {
    setConfirmDeleteConfig({
      isOpen: true,
      title: 'Delete Timer Session',
      message: 'Are you sure you want to delete this timer session?',
      confirmText: 'Delete',
      cancelText: 'Cancel',
      color: 'danger',
      onClose: isConfirmed => {
        if (isConfirmed) {
          deleteSession(sessionId)
        }
        setConfirmDeleteConfig({})
        setEditingSessions({})
        onClose?.()
      },
    })
  }

  const handleClose = () => {
    setEditingSessions({})
    onClose?.()
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

  return (
    <>
      <FadeModal open={isOpen} onClose={onClose} size='lg' fullWidth={true}>
        <Typography level='h4'>Timer Details</Typography>

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
          <Box sx={{ maxHeight: '70vh', overflowY: 'auto' }}>
            {/* Timer Summary */}
            <Card
              variant='plain'
              sx={{
                mb: 1,
              }}
            >
              {/* Header with timeline */}

              {/* Stats Grid */}
              <Box
                sx={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
                  gap: 2,
                }}
              >
                {/* Active Time */}
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
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'start',
                      mb: 0.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        backgroundColor: 'success.500',
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
                      sx={{
                        color: 'success.600',
                        fontWeight: 'bold',
                        lineHeight: 1.5,
                      }}
                    >
                      {formatDuration(calculateCurrentActiveDuration())}
                    </Typography>
                  </Box>
                </Card>

                {/* Idle Time */}
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
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'start',
                      mb: 0.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        backgroundColor: 'warning.500',
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
                      sx={{
                        color: 'warning.600',
                        fontWeight: 'bold',
                        lineHeight: 1.5,
                      }}
                    >
                      {formatDuration(calculateIdleTime())}
                    </Typography>
                  </Box>
                </Card>

                {/* Total Sessions */}
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
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'start',
                      mb: 0.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        backgroundColor: 'primary.500',
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
                      Work Sessions
                    </Typography>
                  </Box>
                  <Box>
                    <Typography
                      level='h4'
                      sx={{
                        color: 'primary.600',
                        fontWeight: 'bold',
                        lineHeight: 1.5,
                      }}
                    >
                      {timerData.pauseLog?.length || 0}
                    </Typography>
                  </Box>
                </Card>

                {/* Total Session Time */}
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
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'start',
                      mb: 0.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        backgroundColor: 'neutral.500',
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
                        color: 'neutral.700',
                        fontWeight: 'bold',
                        lineHeight: 1.5,
                      }}
                    >
                      {formatTime(calculateTotalDuration())}
                    </Typography>
                  </Box>
                </Card>
              </Box>

              {/* Progress Bar */}
              <Box sx={{ mt: 3 }}>
                <Box
                  sx={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    mb: 1,
                  }}
                >
                  <Typography
                    level='body-xs'
                    sx={{ color: 'text.secondary', fontWeight: 'medium' }}
                  >
                    Work vs Break Distribution
                  </Typography>
                  <Typography level='body-xs' sx={{ color: 'text.tertiary' }}>
                    {calculateCurrentActiveDuration() > 0
                      ? `${Math.round((calculateCurrentActiveDuration() / calculateTotalDuration()) * 100)}% active`
                      : 'No active time yet'}
                  </Typography>
                </Box>
                <Box
                  sx={{
                    height: 6,
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
              </Box>
            </Card>

            {/* Time Session */}
            <Box>
              <Typography level='h4' sx={{ mb: 2 }}>
                Session Breakdown
              </Typography>

              <Box>
                {!editingSessions[timerData.id] ? (
                  <Box>
                    {/* Read-only view */}
                    {/* Sessions */}
                    {timerData.pauseLog && timerData.pauseLog.length > 0 && (
                      <Box sx={{ mb: 2 }}>
                        <Typography
                          level='body-sm'
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
                              const startTime = moment(pause.start).format(
                                'HH:mm',
                              )
                              const endTime = pause.end
                                ? moment(pause.end).format('HH:mm')
                                : null

                              const realTimeDuration = isOngoing
                                ? Math.max(
                                    0,
                                    Math.floor(
                                      (currentTime - new Date(pause.start)) /
                                        1000,
                                    ),
                                  )
                                : pause.duration

                              return (
                                <Card
                                  key={pauseIndex}
                                  variant='outlined'
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
                                  }}
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
                                        mb: 0.5,
                                      }}
                                    >
                                      {formatDuration(realTimeDuration)}
                                    </Typography>
                                    {isOngoing && (
                                      <Chip
                                        size='sm'
                                        color='success'
                                        variant='soft'
                                        sx={{ fontSize: '0.75rem' }}
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
                                      // this align to the right side of the card
                                      textAlign: 'right',
                                    }}
                                  >
                                    <Typography
                                      level='body-sm'
                                      sx={{
                                        fontWeight: 'medium',
                                        color: 'text.secondary',
                                        mb: 0.3,
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
                                </Card>
                              )
                            })}
                        </Box>
                      </Box>
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
                            mb: 1,
                          }}
                        >
                          <Typography
                            level='body-sm'
                            sx={{ fontWeight: 'bold' }}
                          >
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
                                  level='body-sm'
                                  sx={{ fontWeight: 'bold' }}
                                >
                                  Session #{pauseIndex + 1}
                                </Typography>
                                <Button
                                  size='sm'
                                  variant='outlined'
                                  color='danger'
                                  onClick={() =>
                                    deletePauseLogEntry(
                                      timerData.id,
                                      pauseIndex,
                                    )
                                  }
                                >
                                  <Delete />
                                </Button>
                              </Box>

                              <Box
                                sx={{
                                  display: 'flex',
                                  flexDirection: 'column',
                                  gap: 2,
                                }}
                              >
                                <FormControl size='sm'>
                                  <Typography
                                    level='body-xs'
                                    sx={{ fontWeight: 'bold' }}
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
                                    level='body-xs'
                                    sx={{ fontWeight: 'bold' }}
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
                                          ? new Date(
                                              e.target.value,
                                            ).toISOString()
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
                                    level='body-xs'
                                    sx={{ fontWeight: 'bold', mb: 0.5 }}
                                  >
                                    Duration (Auto-calculated)
                                  </Typography>
                                  <Typography
                                    level='body-xs'
                                    sx={{
                                      p: 1,
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

            {!timerData && (
              <Alert color='neutral' sx={{ mt: 2 }}>
                No timer session found for this chore.
              </Alert>
            )}
          </Box>
        )}

        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            gap: 1,
          }}
        >
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button variant='outlined' onClick={handleClose} color='neutral'>
              Cancel
            </Button>
          </Box>

          <Box sx={{ display: 'flex', gap: 1 }}>
            {/* Action buttons on the right */}
            {!loading && timerData && !editingSessions[timerData.id] && (
              <>
                <Button
                  size='sm'
                  variant='outlined'
                  color='danger'
                  onClick={() => confirmDeleteSession(timerData.id)}
                >
                  Delete
                </Button>
                <Button
                  variant='outlined'
                  startDecorator={<Edit />}
                  onClick={() => startEditingSession()}
                >
                  Edit
                </Button>
              </>
            )}

            {/* Save button when editing */}
            {!loading && timerData && editingSessions[timerData.id] && (
              <Button
                variant='solid'
                color='primary'
                onClick={() => saveSession(timerData.id)}
                loading={loading}
              >
                Save
              </Button>
            )}
          </Box>
        </Box>
      </FadeModal>

      <ConfirmationModal config={confirmDeleteConfig} />
    </>
  )
}

export default TimerEditModal
