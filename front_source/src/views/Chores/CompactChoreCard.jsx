import {
  CancelScheduleSend,
  Check,
  Delete,
  Edit,
  Pause,
  PlayArrow,
  Repeat,
  Schedule,
  TimesOneMobiledata,
  Webhook,
} from '@mui/icons-material'
import {
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  IconButton,
  Snackbar,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import React from 'react'
import { useNavigate } from 'react-router-dom'
import { useImpersonateUser } from '../../contexts/ImpersonateUserContext.jsx'
import { useUserProfile } from '../../queries/UserQueries.jsx'
import { useNotification } from '../../service/NotificationProvider'
import { notInCompletionWindow } from '../../utils/Chores.jsx'
import {
  getTextColorFromBackgroundColor,
  TASK_COLOR,
} from '../../utils/Colors.jsx'
import {
  DeleteChore,
  MarkChoreComplete,
  PauseChore,
  StartChore,
  UpdateChoreAssignee,
  UpdateDueDate,
} from '../../utils/Fetcher'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import DateModal from '../Modals/Inputs/DateModal'
import SelectModal from '../Modals/Inputs/SelectModal'
import TextModal from '../Modals/Inputs/TextModal'
import WriteNFCModal from '../Modals/Inputs/WriteNFCModal'
import ChoreActionMenu from '../components/ChoreActionMenu'

const CompactChoreCard = ({
  chore,
  performers,
  onChoreUpdate,
  onChoreRemove,
  sx,
  viewOnly,
  onChipClick,
  // Multi-select props
  isMultiSelectMode = false,
  isSelected = false,
  onSelectionToggle,
}) => {
  const [isChangeDueDateModalOpen, setIsChangeDueDateModalOpen] =
    React.useState(false)
  const [isCompleteWithPastDateModalOpen, setIsCompleteWithPastDateModalOpen] =
    React.useState(false)
  const [isChangeAssigneeModalOpen, setIsChangeAssigneeModalOpen] =
    React.useState(false)
  const [isCompleteWithNoteModalOpen, setIsCompleteWithNoteModalOpen] =
    React.useState(false)
  const [confirmModelConfig, setConfirmModelConfig] = React.useState({})
  const [isNFCModalOpen, setIsNFCModalOpen] = React.useState(false)
  const navigate = useNavigate()

  const [isPendingCompletion, setIsPendingCompletion] = React.useState(false)
  const [secondsLeftToCancel, setSecondsLeftToCancel] = React.useState(null)
  const [timeoutId, setTimeoutId] = React.useState(null)
  const { data: userProfile } = useUserProfile()

  const { impersonatedUser } = useImpersonateUser()

  const { showError } = useNotification()

  // Swipe functionality state
  const [swipeTranslateX, setSwipeTranslateX] = React.useState(0)
  const [isDragging, setIsDragging] = React.useState(false)
  const [isSwipeRevealed, setIsSwipeRevealed] = React.useState(false)
  const [hoverTimer, setHoverTimer] = React.useState(null)
  const [isTouchDevice, setIsTouchDevice] = React.useState(false)
  const swipeThreshold = 80 // Minimum swipe distance to reveal actions
  const maxSwipeDistance = 220 // Maximum swipe distance
  const dragStartX = React.useRef(0)
  const cardRef = React.useRef(null)

  // Detect if device supports touch
  React.useEffect(() => {
    const checkTouchDevice = () => {
      setIsTouchDevice('ontouchstart' in window || navigator.maxTouchPoints > 0)
    }
    checkTouchDevice()
  }, [])

  // Swipe gesture handlers
  const handleTouchStart = e => {
    if (isMultiSelectMode || viewOnly) return

    dragStartX.current = e.touches[0].clientX
    setIsDragging(true)
  }

  const handleTouchMove = e => {
    if (isMultiSelectMode || viewOnly || !isDragging) return

    const currentX = e.touches[0].clientX
    const deltaX = currentX - dragStartX.current

    if (isSwipeRevealed) {
      // When actions are revealed, allow right swipe to hide
      if (deltaX > 0) {
        const clampedDelta = Math.min(deltaX - maxSwipeDistance, 0)
        setSwipeTranslateX(clampedDelta)
      }
    } else {
      // When actions are hidden, allow left swipe to reveal
      if (deltaX < 0) {
        const clampedDelta = Math.max(deltaX, -maxSwipeDistance)
        setSwipeTranslateX(clampedDelta)
      }
    }
  }

  const handleTouchEnd = () => {
    if (isMultiSelectMode || viewOnly || !isDragging) return

    setIsDragging(false)

    if (isSwipeRevealed) {
      // When actions are revealed, check if user swiped right enough to hide
      if (swipeTranslateX > -swipeThreshold) {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      } else {
        // Snap back to revealed position
        setSwipeTranslateX(-maxSwipeDistance)
      }
    } else {
      // When actions are hidden, check if user swiped left enough to reveal
      if (Math.abs(swipeTranslateX) > swipeThreshold) {
        setSwipeTranslateX(-maxSwipeDistance)
        setIsSwipeRevealed(true)
      } else {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      }
    }
  }

  const handleMouseDown = e => {
    if (isMultiSelectMode || viewOnly) return

    dragStartX.current = e.clientX
    setIsDragging(true)
  }

  const handleMouseMove = e => {
    if (isMultiSelectMode || viewOnly || !isDragging) return

    const currentX = e.clientX
    const deltaX = currentX - dragStartX.current

    if (isSwipeRevealed) {
      // When actions are revealed, allow right swipe to hide
      if (deltaX > 0) {
        const clampedDelta = Math.min(deltaX - maxSwipeDistance, 0)
        setSwipeTranslateX(clampedDelta)
      }
    } else {
      // When actions are hidden, allow left swipe to reveal
      if (deltaX < 0) {
        const clampedDelta = Math.max(deltaX, -maxSwipeDistance)
        setSwipeTranslateX(clampedDelta)
      }
    }
  }

  const handleMouseUp = () => {
    if (isMultiSelectMode || viewOnly || !isDragging) return

    setIsDragging(false)

    if (isSwipeRevealed) {
      // When actions are revealed, check if user swiped right enough to hide
      if (swipeTranslateX > -swipeThreshold) {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      } else {
        // Snap back to revealed position
        setSwipeTranslateX(-maxSwipeDistance)
      }
    } else {
      // When actions are hidden, check if user swiped left enough to reveal
      if (Math.abs(swipeTranslateX) > swipeThreshold) {
        setSwipeTranslateX(-maxSwipeDistance)
        setIsSwipeRevealed(true)
      } else {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      }
    }
  }

  const resetSwipe = () => {
    setSwipeTranslateX(0)
    setIsSwipeRevealed(false)
  }

  // Hover functionality for desktop
  const handleMouseEnter = () => {
    if (isMultiSelectMode || viewOnly || isSwipeRevealed || isTouchDevice)
      return
    const timer = setTimeout(() => {
      setSwipeTranslateX(-maxSwipeDistance)
      setIsSwipeRevealed(true)
      setHoverTimer(null)
    }, 1500)
    setHoverTimer(timer)
  }

  const handleMouseLeave = () => {
    if (isTouchDevice) return

    if (hoverTimer) {
      clearTimeout(hoverTimer)
      setHoverTimer(null)
    }

    // Add a small delay before hiding to allow moving to action area
    if (isSwipeRevealed) {
      const hideTimer = setTimeout(() => {
        resetSwipe()
      }, 300)
      setHoverTimer(hideTimer)
    }
  }

  const handleActionAreaMouseEnter = () => {
    if (isTouchDevice) return

    // Clear any pending timer when entering action area (both show and hide timers)
    if (hoverTimer) {
      clearTimeout(hoverTimer)
      setHoverTimer(null)
    }
  }

  const handleActionAreaMouseLeave = () => {
    if (isTouchDevice) return

    // Hide immediately when leaving action area
    if (isSwipeRevealed) {
      resetSwipe()
    }
  }

  // Clean up timer on unmount
  React.useEffect(() => {
    return () => {
      if (hoverTimer) {
        clearTimeout(hoverTimer)
      }
    }
  }, [hoverTimer])

  // All the existing handler methods (same as original ChoreCard)
  const handleDelete = () => {
    setConfirmModelConfig({
      isOpen: true,
      title: 'Delete Chore',
      confirmText: 'Delete',
      cancelText: 'Cancel',
      message: 'Are you sure you want to delete this chore?',
      onClose: isConfirmed => {
        if (isConfirmed === true) {
          DeleteChore(chore.id).then(response => {
            if (response.ok) {
              onChoreRemove(chore)
            }
          })
        }
        setConfirmModelConfig({})
      },
    })
  }

  const handleTaskCompletion = () => {
    setIsPendingCompletion(true)
    let seconds = 3
    setSecondsLeftToCancel(seconds)

    const countdownInterval = setInterval(() => {
      seconds -= 1
      setSecondsLeftToCancel(seconds)

      if (seconds <= 0) {
        clearInterval(countdownInterval)
        setIsPendingCompletion(false)
      }
    }, 1000)

    const id = setTimeout(() => {
      MarkChoreComplete(
        chore.id,
        impersonatedUser ? { completedBy: impersonatedUser.userId } : null,
        null,
        null,
      )
        .then(resp => {
          if (resp.ok) {
            return resp.json().then(data => {
              onChoreUpdate(data.res, 'completed')
            })
          }
        })
        .then(() => {
          setIsPendingCompletion(false)
          clearTimeout(id)
          clearInterval(countdownInterval)
          setTimeoutId(null)
          setSecondsLeftToCancel(null)
        })
        .catch(error => {
          if (error?.queued) {
            showError({
              title: 'Update Failed',
              message: 'Request will be reattempt when you are online',
            })
          } else {
            showError({
              title: 'Failed to update',
              message: error,
            })
          }

          setIsPendingCompletion(false)
          clearTimeout(id)
          clearInterval(countdownInterval)
          setTimeoutId(null)
          setSecondsLeftToCancel(null)
        })
    }, 2000)

    setTimeoutId(id)
  }

  const handleChangeDueDate = newDate => {
    UpdateDueDate(chore.id, newDate).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = data.res
          onChoreUpdate(newChore, 'rescheduled')
        })
      }
    })
  }

  const handleCompleteWithPastDate = newDate => {
    MarkChoreComplete(
      chore.id,
      impersonatedUser ? { completedBy: impersonatedUser.userId } : null,
      new Date(newDate).toISOString(),
      null,
    ).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = data.res
          onChoreUpdate(newChore, 'completed')
        })
      }
    })
  }

  const handleAssigneChange = assigneeId => {
    UpdateChoreAssignee(chore.id, assigneeId).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = data.res
          onChoreUpdate(newChore, 'assigned')
        })
      }
    })
  }

  const handleCompleteWithNote = note => {
    MarkChoreComplete(
      chore.id,
      impersonatedUser
        ? { note, completedBy: impersonatedUser.userId }
        : { note },
      null,
      null,
    ).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = data.res
          onChoreUpdate(newChore, 'completed')
        })
      }
    })
  }

  // Utility functions
  const getDueDateText = nextDueDate => {
    if (chore.nextDueDate === null) return 'No Due Date'
    // if due in next 48 hours, we should it in this format : Tomorrow 11:00 AM
    const diff = moment(nextDueDate).diff(moment(), 'hours')
    if (diff < 48 && diff > 0) {
      return moment(nextDueDate).calendar().replace(' at', '')
    }

    return moment(nextDueDate).fromNow()
  }

  const getDueDateColor = nextDueDate => {
    if (chore.nextDueDate === null) return 'neutral'
    const diff = moment(nextDueDate).diff(moment(), 'hours')
    if (diff < 48 && diff > 0) {
      return 'warning'
    }
    if (diff < 0) {
      return 'danger'
    }
    return 'neutral'
  }

  const getRecurrentText = chore => {
    // if chore.frequencyMetadata is type string then parse it otherwise assigned to the metadata:
    const metadata =
      typeof chore.frequencyMetadata === 'string'
        ? JSON.parse(chore.frequencyMetadata)
        : chore.frequencyMetadata

    const dayOfMonthSuffix = n => {
      if (n >= 11 && n <= 13) {
        return 'th'
      }
      switch (n % 10) {
        case 1:
          return 'st'
        case 2:
          return 'nd'
        case 3:
          return 'rd'
        default:
          return 'th'
      }
    }
    if (chore.frequencyType === 'once') {
      return 'Once'
    } else if (chore.frequencyType === 'trigger') {
      return 'Trigger'
    } else if (chore.frequencyType === 'daily') {
      return 'Daily'
    } else if (chore.frequencyType === 'adaptive') {
      return 'Adaptive'
    } else if (chore.frequencyType === 'weekly') {
      return 'Weekly'
    } else if (chore.frequencyType === 'monthly') {
      return 'Monthly'
    } else if (chore.frequencyType === 'yearly') {
      return 'Yearly'
    } else if (chore.frequencyType === 'days_of_the_week') {
      let days = metadata.days
      if (days.length > 4) {
        const allDays = [
          'Sunday',
          'Monday',
          'Tuesday',
          'Wednesday',
          'Thursday',
          'Friday',
          'Saturday',
        ]
        const selectedDays = days.map(d => moment().day(d).format('dddd'))
        const notSelectedDay = allDays.filter(
          day => !selectedDays.includes(day),
        )
        const notSelectedShortdays = notSelectedDay.map(d =>
          moment().day(d).format('ddd'),
        )
        return `Daily except ${notSelectedShortdays.join(', ')}`
      } else {
        days = days.map(d => moment().day(d).format('ddd'))
        return days.join(', ')
      }
    } else if (chore.frequencyType === 'day_of_the_month') {
      let months = metadata.months
      if (months.length > 6) {
        const allMonths = [
          'January',
          'February',
          'March',
          'April',
          'May',
          'June',
          'July',
          'August',
          'September',
          'October',
          'November',
          'December',
        ]
        const selectedMonths = months.map(m => moment().month(m).format('MMMM'))
        const notSelectedMonth = allMonths.filter(
          month => !selectedMonths.includes(month),
        )
        const notSelectedShortMonths = notSelectedMonth.map(m =>
          moment().month(m).format('MMM'),
        )
        let result = `Monthly ${chore.frequency}${dayOfMonthSuffix(
          chore.frequency,
        )}`
        if (notSelectedShortMonths.length > 0)
          result += `
        except ${notSelectedShortMonths.join(', ')}`
        return result
      } else {
        let freqData = metadata
        const months = freqData.months.map(m => moment().month(m).format('MMM'))
        return `${chore.frequency}${dayOfMonthSuffix(
          chore.frequency,
        )} of ${months.join(', ')}`
      }
    } else if (chore.frequencyType === 'interval') {
      return `Every ${chore.frequency} ${metadata.unit}`
    } else {
      return chore.frequencyType
    }
  }

  const getFrequencyIcon = chore => {
    if (['once', 'no_repeat'].includes(chore.frequencyType)) {
      return <TimesOneMobiledata sx={{ fontSize: 14 }} />
    } else if (chore.frequencyType === 'trigger') {
      return <Webhook sx={{ fontSize: 14 }} />
    } else {
      return <Repeat sx={{ fontSize: 14 }} />
    }
  }

  const formatMetadata = () => {
    const parts = []

    // Frequency
    parts.push(getRecurrentText(chore))

    // Assignee (if not current user)
    if (userProfile && chore.assignedTo !== userProfile.id) {
      const assignee = performers.find(
        p => p.id === chore.assignedTo,
      )?.displayName
      if (assignee) parts.push(assignee)
    }

    // Points
    if (chore.points > 0) {
      parts.push(`${chore.points}pts`)
    }

    return parts.join(' • ')
  }

  const getPriorityColor = priority => {
    switch (priority) {
      case 1:
        return TASK_COLOR.PRIORITY_1
      case 2:
        return TASK_COLOR.PRIORITY_2
      case 3:
        return TASK_COLOR.PRIORITY_3
      case 4:
        return TASK_COLOR.PRIORITY_4
      default:
        return TASK_COLOR.NO_PRIORITY
    }
  }
  const handleChorePause = () => {
    PauseChore(chore.id).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = {
            ...chore,
            ...data.res,
          }
          onChoreUpdate(newChore, 'paused')
        })
      }
    })
  }
  const handleChoreStart = () => {
    StartChore(chore.id).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = {
            ...chore,
            ...data.res,
          }
          onChoreUpdate(newChore, 'started')
        })
      }
    })
  }

  return (
    <Box key={chore.id + '-compact-box'}>
      <Box
        sx={{
          position: 'relative',
          overflow: 'hidden',
          borderBottom: '1px solid',
          borderColor: 'divider',
          '&:last-child': {
            borderBottom: 'none',
          },
        }}
        onMouseLeave={handleMouseLeave}
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
            // soft background color for the swipe area
            // bgcolor: 'background.backdrop',
            boxShadow: 'inset 2px 0 4px rgba(0,0,0,0.06)',
            zIndex: 0,
          }}
          onMouseEnter={handleActionAreaMouseEnter}
          onMouseLeave={handleActionAreaMouseLeave}
        >
          <IconButton
            variant='soft'
            color='success'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()

              if (chore.status === 0 || chore.status === 2) {
                handleChoreStart()
              } else {
                // handleChorePause()
                handleTaskCompletion()
              }
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
              // bgcolor: 'success.100',
              // color: 'success.600',
              // '&:hover': {
              //   bgcolor: 'success.200',
              // },
            }}
          >
            {chore.status !== 1 ? (
              <PlayArrow sx={{ fontSize: 16 }} />
            ) : (
              <Check sx={{ fontSize: 16 }} />
            )}
          </IconButton>

          <IconButton
            variant='soft'
            color='warning'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()
              setIsChangeDueDateModalOpen(true)
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
              // bgcolor: 'warning.100',
              // color: 'warning.600',
              // '&:hover': {
              //   bgcolor: 'warning.200',
              // },
            }}
          >
            <Schedule sx={{ fontSize: 16 }} />
          </IconButton>

          <IconButton
            variant='soft'
            color='neutral'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()
              navigate(`/chores/${chore.id}/edit`)
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
              // bgcolor: 'neutral.100',
              // color: 'neutral.600',
              // '&:hover': {
              //   bgcolor: 'neutral.200',
              // },
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
              resetSwipe()
              handleDelete()
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

        {/* Main card content */}
        <Box
          ref={cardRef}
          style={viewOnly ? { pointerEvents: 'none' } : {}}
          sx={{
            ...sx,
            display: 'flex',
            alignItems: 'center',
            minHeight: 56,
            cursor: 'pointer',
            position: 'relative',
            pl: '16px',
            bgcolor: 'background.body',
            transform: `translateX(${swipeTranslateX}px)`,
            transition: isDragging ? 'none' : 'transform 0.3s ease-out',
            zIndex: 1,
            '&:hover': {
              bgcolor: isSwipeRevealed
                ? 'background.surface'
                : 'background.level1',
              boxShadow: isSwipeRevealed ? 'none' : 'sm',
            },
            '&::before': {
              content: '""',
              position: 'absolute',
              left: 0,
              top: 0,
              bottom: 0,
              width: '3px',
              backgroundColor: getPriorityColor(chore.priority),
              borderRadius: '16px',
            },
          }}
          onClick={() => {
            if (isSwipeRevealed) {
              resetSwipe()
              return
            }
            if (isMultiSelectMode) {
              onSelectionToggle()
            } else {
              navigate(`/chores/${chore.id}`)
            }
          }}
          onTouchStart={handleTouchStart}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          // onMouseEnter={handleMouseEnter}
        >
          {/* Priority bar clickable area */}
          {chore.priority > 0 && (
            <Box
              sx={{
                position: 'absolute',
                left: 0,
                top: 0,
                bottom: 0,
                width: '12px',
                cursor: 'pointer',
                zIndex: 1,
              }}
              onClick={e => {
                e.stopPropagation()
                onChipClick({ priority: chore.priority })
              }}
            />
          )}

          {/* Animated transition container for Complete Button / Multi-select checkbox */}
          <Box
            sx={{
              position: 'relative',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 40,
              height: 40,
              mr: 1.5,
              flexShrink: 0,
            }}
          >
            {/* Complete Button */}
            <Box
              sx={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: '100%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition:
                  'opacity 0.3s ease-in-out, transform 0.3s ease-in-out',
                opacity: isMultiSelectMode ? 0 : 1,
                transform: isMultiSelectMode
                  ? 'scale(0.8) rotate(45deg)'
                  : 'scale(1) rotate(0deg)',
                pointerEvents: isMultiSelectMode ? 'none' : 'auto',
              }}
            >
              <IconButton
                variant='soft'
                color={chore.status === 0 ? 'success' : 'warning'}
                size='sm'
                onClick={e => {
                  e.stopPropagation()
                  if (chore.status === 0) {
                    handleTaskCompletion()
                  } else if (chore.status === 1) {
                    handleChorePause()
                  } else {
                    handleChoreStart()
                  }
                }}
                disabled={isPendingCompletion || notInCompletionWindow(chore)}
                sx={{
                  width: 32,
                  height: 32,
                  borderRadius: '50%',
                  transition: 'all 0.2s ease',
                  '&:hover': {
                    transform: 'scale(1.05)',
                  },

                  '&:active': {
                    transform: 'scale(0.95)',
                  },
                  '&:disabled': {
                    opacity: 0.5,
                    transform: 'none',
                  },
                }}
              >
                {isPendingCompletion ? (
                  <CircularProgress size='sm' />
                ) : chore.status === 0 ? (
                  <Check sx={{ fontSize: 16 }} />
                ) : chore.status === 1 ? (
                  <Pause sx={{ fontSize: 16 }} />
                ) : (
                  <PlayArrow sx={{ fontSize: 16 }} />
                )}
              </IconButton>
            </Box>

            {/* Multi-select Checkbox */}
            <Box
              sx={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: '100%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                transition:
                  'opacity 0.3s ease-in-out, transform 0.3s ease-in-out',
                opacity: isMultiSelectMode ? 1 : 0,
                transform: isMultiSelectMode
                  ? 'scale(1) rotate(0deg)'
                  : 'scale(0.8) rotate(-45deg)',
                pointerEvents: isMultiSelectMode ? 'auto' : 'none',
              }}
            >
              <Checkbox
                checked={isSelected}
                onChange={onSelectionToggle}
                sx={{
                  bgcolor: 'background.surface',
                  borderRadius: 'md',
                  boxShadow: 'sm',
                  border: '2px solid',
                  borderColor: 'divider',
                  '&:hover': {
                    bgcolor: 'background.level1',
                    borderColor: 'primary.300',
                  },
                  '&.Mui-checked': {
                    bgcolor: 'primary.500',
                    borderColor: 'primary.500',
                    color: 'primary.solidColor',
                    '&:hover': {
                      bgcolor: 'primary.600',
                      borderColor: 'primary.600',
                    },
                  },
                }}
                onClick={e => e.stopPropagation()}
              />
            </Box>
          </Box>

          {/* Content - Center */}
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              mr: 1.5,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {/* Line 1: Name + Due Date */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                mb: 0.25,
              }}
            >
              {/* Chore Name */}
              <Typography
                level='title-sm'
                sx={{
                  fontWeight: 600,
                  fontSize: 14,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  mr: 1,
                  flex: 1,
                  minWidth: 0,
                }}
              >
                {chore.name}
              </Typography>

              {/* Due Date - Inline with name */}
              <Chip
                variant='soft'
                size='sm'
                color={getDueDateColor(chore.nextDueDate)}
                sx={{
                  fontSize: 10,
                  height: 18,
                  px: 0.75,
                  flexShrink: 0,
                  ml: 1,
                }}
              >
                {getDueDateText(chore.nextDueDate)}
              </Chip>
            </Box>

            {/* Line 2: Metadata */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
              {getFrequencyIcon(chore)}
              <Typography
                level='body-xs'
                color='text.secondary'
                sx={{
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  fontSize: 11,
                }}
              >
                {formatMetadata()}
              </Typography>

              {/* Labels - Priority chip removed, now shown as vertical bar */}
              {chore.labelsV2?.map(l => (
                <div
                  role='none'
                  tabIndex={0}
                  onClick={e => {
                    e.stopPropagation()
                    onChipClick({ label: l })
                  }}
                  onKeyDown={e => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.stopPropagation()
                      onChipClick({ label: l })
                    }
                  }}
                  style={{
                    cursor: 'pointer',
                    padding: 0,
                    margin: 0,
                    display: 'flex',
                    alignItems: 'center',
                  }}
                  key={`compact-chorecard-${chore.id}-label-${l.id}`}
                >
                  <Chip
                    variant='solid'
                    color='primary'
                    size='sm'
                    sx={{
                      ml: 0.5,
                      // height: 16,
                      // fontSize: 9,
                      // px: 0.5,
                      backgroundColor: `${l?.color} !important`,
                      color: getTextColorFromBackgroundColor(l?.color),
                    }}
                  >
                    {l?.name}
                  </Chip>
                </div>
              ))}
            </Box>
          </Box>

          {/* Right side - Action Menu with animation */}
          <Box
            sx={{
              transition:
                'opacity 0.3s ease-in-out, transform 0.3s ease-in-out, width 0.3s ease-in-out, margin 0.3s ease-in-out',
              opacity: isMultiSelectMode ? 0 : 1,
              transform: isMultiSelectMode
                ? 'translateX(20px) scale(0.8)'
                : 'translateX(0) scale(1)',
              width: isMultiSelectMode ? 0 : 32,
              marginRight: isMultiSelectMode ? 0 : undefined,
              overflow: 'hidden',
              pointerEvents: isMultiSelectMode ? 'none' : 'auto',
            }}
          >
            <ChoreActionMenu
              variant='plain'
              chore={chore}
              onChoreUpdate={onChoreUpdate}
              onChoreRemove={onChoreRemove}
              onCompleteWithNote={() => setIsCompleteWithNoteModalOpen(true)}
              onCompleteWithPastDate={() =>
                setIsCompleteWithPastDateModalOpen(true)
              }
              onChangeAssignee={() => setIsChangeAssigneeModalOpen(true)}
              onChangeDueDate={() => setIsChangeDueDateModalOpen(true)}
              onWriteNFC={() => setIsNFCModalOpen(true)}
              onDelete={handleDelete}
              onMouseEnter={handleMouseEnter}
              // onMouseLeave={handleMouseLeave}
              sx={{
                width: 32,
                height: 32,
                color: 'text.tertiary',
                flexShrink: 0,
                '&:hover': {
                  color: 'text.secondary',
                  bgcolor: 'background.level1',
                },
              }}
              onOpen={() => {
                handleMouseLeave()
              }}
            />
          </Box>
        </Box>
      </Box>

      {/* All modals (same as original) */}
      <DateModal
        isOpen={isChangeDueDateModalOpen}
        key={'changeDueDate' + chore.id}
        current={chore.nextDueDate}
        title={`Change due date`}
        onClose={() => setIsChangeDueDateModalOpen(false)}
        onSave={handleChangeDueDate}
      />

      <DateModal
        isOpen={isCompleteWithPastDateModalOpen}
        key={'completedInPast' + chore.id}
        current={chore.nextDueDate}
        title={`Save Chore that you completed in the past`}
        onClose={() => setIsCompleteWithPastDateModalOpen(false)}
        onSave={handleCompleteWithPastDate}
      />

      <SelectModal
        isOpen={isChangeAssigneeModalOpen}
        options={performers}
        displayKey='displayName'
        title={`Delegate to someone else`}
        placeholder={'Select a performer'}
        onClose={() => setIsChangeAssigneeModalOpen(false)}
        onSave={selected => handleAssigneChange(selected.id)}
      />

      {confirmModelConfig?.isOpen && (
        <ConfirmationModal config={confirmModelConfig} />
      )}

      <TextModal
        isOpen={isCompleteWithNoteModalOpen}
        title='Add note to attach to this completion:'
        onClose={() => setIsCompleteWithNoteModalOpen(false)}
        okText={'Complete'}
        onSave={handleCompleteWithNote}
      />

      <WriteNFCModal
        config={{
          isOpen: isNFCModalOpen,
          url: `${window.location.origin}/chores/${chore.id}`,
          onClose: () => setIsNFCModalOpen(false),
        }}
      />

      {/* Snackbar for pending completion */}
      <Snackbar
        open={isPendingCompletion}
        endDecorator={
          <Button
            onClick={() => {
              if (timeoutId) {
                clearTimeout(timeoutId)
                setIsPendingCompletion(false)
                setTimeoutId(null)
                setSecondsLeftToCancel(null)
              }
            }}
            size='sm'
            variant='outlined'
            color='primary'
            startDecorator={<CancelScheduleSend />}
          >
            Cancel
          </Button>
        }
      >
        <Typography level='body-xs' textAlign={'center'}>
          Task will be marked as completed in {secondsLeftToCancel} seconds
        </Typography>
      </Snackbar>
    </Box>
  )
}

export default CompactChoreCard
