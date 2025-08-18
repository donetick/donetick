import { Save } from '@mui/icons-material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import InfoIcon from '@mui/icons-material/Info'
import NotificationsIcon from '@mui/icons-material/Notifications'
import Alert from '@mui/joy/Alert'
import Badge from '@mui/joy/Badge'
import Box from '@mui/joy/Box'
import Button from '@mui/joy/Button'
import IconButton from '@mui/joy/IconButton'
import Input from '@mui/joy/Input'
import Option from '@mui/joy/Option'
import Select from '@mui/joy/Select'
import Typography from '@mui/joy/Typography'
import { useCallback, useEffect, useState } from 'react'

const timeUnits = [
  { label: 'Mins', value: 'm' },
  { label: 'Hours', value: 'h' },
  { label: 'Days', value: 'd' },
]

const timingOptions = [
  { label: 'Before', value: 'before' },
  { label: 'On Due', value: 'ondue' },
  { label: 'After', value: 'after' },
]

function getRelativeLabel(notification) {
  const { value, unit } = notification
  const numericValue = Number(value)
  if (numericValue === 0) {
    return 'On due date'
  }
  const unitName = unit === 'm' ? 'minutes' : unit === 'h' ? 'hours' : 'days'
  const absValue = Math.abs(numericValue)
  return `${absValue} ${unitName} ${numericValue < 0 ? 'before' : 'after'} due`
}

// Helper functions to convert between internal value and UI representation
function getUIRepresentation(notification) {
  const numericValue = Number(notification.value)
  if (numericValue === 0) {
    return { timing: 'ondue', displayValue: 0, unit: notification.unit }
  } else if (numericValue < 0) {
    return {
      timing: 'before',
      displayValue: Math.abs(numericValue),
      unit: notification.unit,
    }
  } else {
    return {
      timing: 'after',
      displayValue: numericValue,
      unit: notification.unit,
    }
  }
}

function getInternalValue(timing, displayValue) {
  if (timing === 'ondue') return 0
  if (timing === 'before') return -Math.abs(displayValue)
  return Math.abs(displayValue) // 'after'
}

const NotificationTemplate = ({
  maxNotifications = 5,
  onChange,
  value,
  showTimeline = true,
}) => {
  const [notifications, setNotifications] = useState(
    value?.templates ||
      JSON.parse(localStorage.getItem('defaultNotificationTemplate')) ||
      [],
  )

  const [error, setError] = useState(null)
  const [showSaveDefault, setShowSaveDefault] = useState(false)
  // Create a map of notification indices for timeline display
  const [notificationIndexMap, setNotificationIndexMap] = useState({})

  const updateNotificationIndices = useCallback(() => {
    // Convert notifications to minutes for proper chronological sorting
    const convertToMinutes = (value, unit) => {
      const numericValue = Number(value)
      if (numericValue === 0) return 0
      let minutes = Math.abs(numericValue)
      if (unit === 'h') minutes *= 60
      if (unit === 'd') minutes *= 24 * 60
      return numericValue < 0 ? -minutes : minutes
    }

    // Sort notifications for consistent ordering by actual time duration
    const sorted = [...notifications].sort((a, b) => {
      const aMinutes = convertToMinutes(a.value, a.unit)
      const bMinutes = convertToMinutes(b.value, b.unit)
      return aMinutes - bMinutes
    })

    const indexMap = {}
    // Map original array indices to their chronological position numbers
    notifications.forEach((originalNotification, originalIdx) => {
      const chronologicalPosition = sorted.findIndex(
        sortedNotification =>
          Number(sortedNotification.value) ===
            Number(originalNotification.value) &&
          sortedNotification.unit === originalNotification.unit,
      )
      indexMap[originalIdx] = chronologicalPosition + 1
    })

    setNotificationIndexMap(indexMap)
  }, [notifications])

  // Sort notifications and update the index mapping
  useEffect(() => {
    updateNotificationIndices()
    setError(null)
  }, [updateNotificationIndices])

  // Notify parent component of changes including the template name
  useEffect(() => {
    if (onChange) {
      onChange({ notifications })
    }
  }, [notifications, onChange])

  // Validates if a notification configuration already exists
  const isDuplicate = (notification, currentIdx = -1) => {
    return notifications.some((n, idx) => {
      if (idx === currentIdx) return false

      return (
        Number(n.value) === Number(notification.value) &&
        n.unit === notification.unit
      )
    })
  }

  const handleChange = (idx, field, value) => {
    const currentNotification = notifications[idx]
    const uiRep = getUIRepresentation(currentNotification)

    let updatedUIRep = { ...uiRep }
    let updatedNotification = { ...currentNotification }

    // Update the UI representation based on the field being changed
    if (field === 'timing') {
      updatedUIRep.timing = value
      // Reset display value when switching to "On Due"
      if (value === 'ondue') {
        updatedUIRep.displayValue = 0
      }
    } else if (field === 'displayValue') {
      updatedUIRep.displayValue = Math.max(0, Number(value))
    } else if (field === 'unit') {
      updatedUIRep.unit = value
      updatedNotification.unit = value
    }

    // Convert back to internal representation
    const newInternalValue = getInternalValue(
      updatedUIRep.timing,
      updatedUIRep.displayValue,
    )
    updatedNotification = {
      ...updatedNotification,
      value: newInternalValue,
      unit: updatedUIRep.unit,
    }

    // Check if another notification is already "On Due" (value = 0)
    if (newInternalValue === 0) {
      const existingOnDue = notifications.findIndex(
        (n, i) => i !== idx && Number(n.value) === 0,
      )

      if (existingOnDue !== -1) {
        setError(
          'Only one notification can be set to "On Due". Please choose a different timing.',
        )
        return
      }
    }

    if (isDuplicate(updatedNotification, idx)) {
      setError(
        'This notification setting already exists. Please use a different timing.',
      )
      return
    }

    const updated = notifications.map((n, i) =>
      i === idx ? updatedNotification : n,
    )
    setNotifications(updated)
    setError(null)
  }

  const addSmartNotification = type => {
    if (notifications.length >= maxNotifications) return
    setShowSaveDefault(true)
    let newNotification
    let suggestions = []

    switch (type) {
      case 'reminder':
        // Suggest common reminder times that don't exist
        suggestions = [
          { value: -1, unit: 'd' }, // 1 day before
          { value: -3, unit: 'h' }, // 3 hours before
          { value: -30, unit: 'm' }, // 3 days before
        ]
        break

      case 'due':
        if (notifications.some(n => Number(n.value) === 0)) {
          setError('Only one "Due Alert" notification is allowed.')
          return
        }
        newNotification = { value: 0, unit: 'm' }
        break

      case 'followup':
        suggestions = [
          { value: 1, unit: 'd' }, // 1 day after
          { value: 3, unit: 'd' }, // 3 days after
          { value: 7, unit: 'd' }, // 1 week after
        ]
        break
    }

    // For reminder/followup, find first non-duplicate suggestion
    if (suggestions.length > 0) {
      newNotification = suggestions.find(suggestion => !isDuplicate(suggestion))

      if (!newNotification) {
        setError(`All common ${type} times are already configured.`)
        return
      }
    }

    // Add the new notification to the end (don't sort, keep form order)
    const updatedNotifications = [...notifications, newNotification]

    setNotifications(updatedNotifications)
    setError(null)
  }

  const removeNotification = idx => {
    const updated = notifications.filter((_, i) => i !== idx)
    setNotifications(updated)
    onChange && onChange(updated)
    setShowSaveDefault(true)
  }
  const renderTimeline = () => {
    // Convert notifications to minutes for proper chronological sorting
    const convertToMinutes = (value, unit) => {
      const numericValue = Number(value)
      if (numericValue === 0) return 0
      let minutes = Math.abs(numericValue)
      if (unit === 'h') minutes *= 60
      if (unit === 'd') minutes *= 24 * 60
      return numericValue < 0 ? -minutes : minutes
    }

    // Sort notifications chronologically by actual time (in minutes)
    const sorted = [...notifications].sort((a, b) => {
      const aMinutes = convertToMinutes(a.value, a.unit)
      const bMinutes = convertToMinutes(b.value, b.unit)
      return aMinutes - bMinutes
    })

    // Get min and max notification times in minutes for dynamic scaling
    const minutesValues = sorted.map(n => convertToMinutes(n.value, n.unit))
    const minBefore = Math.min(0, ...minutesValues) // Default to 0 if no "before" notifications
    const maxAfter = Math.max(0, ...minutesValues) // Default to 0 if no "after" notifications

    const getPositionPercent = (value, unit) => {
      const minutes = convertToMinutes(value, unit)

      // Due date is always at center (50%)
      if (minutes === 0) return 50

      // For notifications before due date (negative values)
      if (minutes < 0) {
        if (minBefore === 0) return 30 // Default position if no before notifications
        // Scale between 10% (furthest left) and 45% (closest to due)
        return 45 - (Math.abs(minutes) / Math.abs(minBefore)) * 35
      }

      // For notifications after due date (positive values)
      if (maxAfter === 0) return 70 // Default position if no after notifications
      // Scale between 55% (closest to due) and 90% (furthest right)
      return 55 + (minutes / maxAfter) * 35
    }

    return (
      <Box sx={{ mt: 3, mb: 2 }}>
        <Typography level={'body-md'} sx={{ mb: 1 }}>
          Notification Timeline
        </Typography>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            position: 'relative',
            height: 90,
            bgcolor: 'background.level1',
            borderRadius: 'md',
            p: 2,
            transition: 'height 0.3s ease',
            // '&:hover': {
            //   height: 130,
            // },
          }}
        >
          {/* Timeline line */}
          <Box
            sx={{
              position: 'relative',
              width: '100%',
              height: 3,
              bgcolor: 'neutral.outlinedBorder',
              mt: 2,
            }}
          >
            {/* Due date marker */}
            <Box
              sx={{
                position: 'absolute',
                left: '50%',
                height: 16,
                width: 3,
                bgcolor: 'warning.500',
                top: -8,
                transform: 'translateX(-50%)',
                borderRadius: 'sm',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
              }}
            >
              <Typography
                level={'body-xs'}
                sx={{
                  mt: 4,
                  fontWeight: 'md',
                  color: 'warning.700',
                  fontSize: '0.6rem',
                }}
              >
                Due Date
              </Typography>
            </Box>

            {/* Notification markers */}
            {sorted.map((n, i) => {
              // Calculate position based on actual time duration
              const percent = getPositionPercent(n.value, n.unit)

              return (
                <Box
                  key={i}
                  sx={{
                    position: 'absolute',
                    left: `${percent}%`,
                    transform: 'translateX(-50%)',
                    color:
                      Number(n.value) < 0
                        ? 'primary.600'
                        : Number(n.value) === 0
                          ? 'warning.600'
                          : 'success.600',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    transition: 'all 0.3s ease',
                    cursor: 'pointer',
                    opacity: 0.85,
                    '&:hover': {
                      opacity: 1,
                      transform: 'translateX(-50%) scale(1.1)',
                      zIndex: 10,
                    },
                  }}
                  title={getRelativeLabel(n)}
                >
                  <Badge
                    badgeContent={
                      notificationIndexMap[
                        notifications.findIndex(
                          original =>
                            Number(original.value) === Number(n.value) &&
                            original.unit === n.unit,
                        )
                      ] || i + 1
                    }
                    size={'sm'}
                    variant={'solid'}
                    color={
                      Number(n.value) < 0
                        ? 'success'
                        : Number(n.value) === 0
                          ? 'warning'
                          : 'danger'
                    }
                    sx={{
                      '--Badge-paddingX': '4px',
                      '--Badge-minHeight': '16px',
                      '--Badge-fontSize': '0.65rem',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <NotificationsIcon
                      fontSize={'small'}
                      sx={{
                        height: 18,
                        width: 18,
                      }}
                    />
                  </Badge>
                </Box>
              )
            })}
          </Box>
        </Box>
      </Box>
    )
  }

  return (
    <Box
      sx={
        {
          // border: '1px solid',
          // borderColor: 'neutral.outlinedBorder',
          // borderRadius: 2,
          // p: 3,
          // maxWidth: 500,
          // bgcolor: 'background.body',
          // boxShadow: 'sm',
        }
      }
    >
      {/* <Typography level={'h4'} sx={{ mb: 2 }}>
        Schedule Name
      </Typography> */}

      {/* Template Name Field */}
      {/* <Box sx={{ mb: 3 }}>
        <Typography level={'body2'} sx={{ mb: 1, fontWeight: 'md' }}>
          Template Name
        </Typography>
        <Input
          value={templateName}
          onChange={handleNameChange}
          placeholder='Enter template name'
          sx={{ width: '100%' }}
        />
      </Box> */}

      {error && (
        <Alert
          variant='soft'
          color='danger'
          sx={{ mb: 2 }}
          startDecorator={<InfoIcon />}
        >
          {error}
        </Alert>
      )}

      {notifications
        .map((n, idx) => ({ notification: n, originalIndex: idx }))
        .sort((a, b) => {
          const aBadgeNumber = notificationIndexMap[a.originalIndex] || 0
          const bBadgeNumber = notificationIndexMap[b.originalIndex] || 0
          return aBadgeNumber - bBadgeNumber
        })
        .map(({ notification: n, originalIndex: idx }) => {
          // Get ordered badge number from timeline sorting
          const badgeNumber = notificationIndexMap[idx]
          const uiRep = getUIRepresentation(n)

          return (
            <Box
              key={idx}
              sx={{ display: 'flex', alignItems: 'center', mb: 1 }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'start',
                  width: 18,
                  flexShrink: 0,
                }}
              >
                <Badge
                  badgeContent={badgeNumber}
                  size={'sm'}
                  sx={{
                    '--Badge-minHeight': '20px',
                    '--Badge-fontSize': '0.75rem',
                  }}
                  color={
                    Number(n.value) < 0
                      ? 'success'
                      : Number(n.value) === 0
                        ? 'warning'
                        : 'danger'
                  }
                >
                  {/* Empty box to attach badge to */}
                </Badge>
              </Box>
              <Select
                value={uiRep.timing}
                onChange={(_, value) => handleChange(idx, 'timing', value)}
                sx={{ mr: 1, minWidth: 100 }}
                size={'sm'}
              >
                {timingOptions.map(opt => (
                  <Option key={opt.value} value={opt.value}>
                    {opt.label}
                  </Option>
                ))}
              </Select>
              <Input
                type={'number'}
                min={0}
                value={uiRep.displayValue}
                disabled={uiRep.timing === 'ondue'}
                onChange={e =>
                  handleChange(idx, 'displayValue', e.target.value)
                }
                sx={{
                  width: 80,
                  mr: 1,
                  opacity: uiRep.timing === 'ondue' ? 0.6 : 1,
                }}
                size={'sm'}
                placeholder='0'
              />
              <Select
                value={n.unit}
                disabled={uiRep.timing === 'ondue'}
                onChange={(_, value) => handleChange(idx, 'unit', value)}
                sx={{
                  mr: 1,
                  minWidth: 80,
                  opacity: uiRep.timing === 'ondue' ? 0.6 : 1,
                }}
                size={'sm'}
              >
                {timeUnits.map(opt => (
                  <Option key={opt.value} value={opt.value}>
                    {opt.label}
                  </Option>
                ))}
              </Select>
              <IconButton
                onClick={() => removeNotification(idx)}
                disabled={notifications.length === 1}
                color={'danger'}
                size={'sm'}
                sx={{ mr: 1 }}
                variant={'soft'}
              >
                <DeleteIcon fontSize={'small'} />
              </IconButton>
            </Box>
          )
        })}
      <Box sx={{ display: 'flex', gap: 1, mt: 1, mb: 2, flexWrap: 'wrap' }}>
        <Button
          onClick={() => addSmartNotification('reminder')}
          disabled={notifications.length >= maxNotifications}
          startDecorator={<AddIcon />}
          size={'sm'}
          variant={'outlined'}
          color={'primary'}
        >
          Reminder
        </Button>
        <Button
          onClick={() => addSmartNotification('due')}
          disabled={
            notifications.length >= maxNotifications ||
            notifications.some(n => Number(n.value) === 0)
          }
          startDecorator={<AddIcon />}
          size={'sm'}
          variant={'outlined'}
          color={'warning'}
        >
          Due Alert
        </Button>
        <Button
          onClick={() => addSmartNotification('followup')}
          disabled={notifications.length >= maxNotifications}
          startDecorator={<AddIcon />}
          size={'sm'}
          variant={'outlined'}
          color={'danger'}
        >
          Follow-up
        </Button>
      </Box>
      {showSaveDefault && (
        <Button
          variant='outlined'
          size='sm'
          color='neutral'
          // sx={{ ml: 'auto', mt: 0.5 }}
          startDecorator={<Save />}
          onClick={() => {
            localStorage.setItem(
              'defaultNotificationTemplate',
              JSON.stringify(notifications),
            )
            setShowSaveDefault(false)
          }}
        >
          Save as Default for Future Tasks
        </Button>
      )}

      {showTimeline && renderTimeline()}
    </Box>
  )
}

export default NotificationTemplate
