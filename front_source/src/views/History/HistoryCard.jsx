import {
  AccessTime,
  CalendarMonth,
  Check,
  CheckCircle,
  EventNote,
  Person,
  Redo,
  Timelapse,
  Toll,
} from '@mui/icons-material'
import {
  Avatar,
  Box,
  Chip,
  Grid,
  ListDivider,
  ListItem,
  ListItemContent,
  Typography,
} from '@mui/joy'
import moment from 'moment'

const getCompletedChip = historyEntry => {
  if (historyEntry.status === 0) {
    return null
  }
  if (!historyEntry.dueDate) {
    return null
    // <Chip
    //   size='sm'
    //   variant='soft'
    //   color='neutral'
    //   startDecorator={<CalendarViewDay />}
    // >
    //   No Due Date
    // </Chip>
  }

  const performedAt = moment(historyEntry.performedAt)
  const dueDate = moment(historyEntry.dueDate)
  // TODO: make this a config at some point
  const gracePeriod = 6 * 60 * 60 * 1000 // 6 hours in milliseconds

  if (Math.abs(performedAt - dueDate) <= gracePeriod) {
    return (
      <Chip
        size='sm'
        variant='solid'
        color='success'
        startDecorator={<Check />}
      >
        On Time
      </Chip>
    )
  } else if (performedAt.isBefore(dueDate)) {
    return (
      <Chip size='sm' variant='soft' color='primary' startDecorator={<Check />}>
        Early
      </Chip>
    )
  } else {
    return (
      <Chip
        size='sm'
        variant='solid'
        color='warning'
        startDecorator={<Timelapse />}
      >
        Late
      </Chip>
    )
  }
}

const formatTime = seconds => {
  if (typeof seconds !== 'number' || isNaN(seconds) || seconds < 0) {
    return null
  }
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
}

/**
 * Compact HistoryCard component with improved UX and 2-row height design
 */
const HistoryCard = ({
  allHistory,
  performers,
  historyEntry,
  index,
  onClick,
}) => {
  const performer = performers.find(p => p.userId === historyEntry.completedBy)
  const assignedTo = performers.find(p => p.userId === historyEntry.assignedTo)

  const formatTimeDifference = (startDate, endDate) => {
    const diffInMinutes = moment(startDate).diff(endDate, 'minutes')
    let timeValue = diffInMinutes
    let unit = 'minute'

    if (diffInMinutes >= 60) {
      const diffInHours = moment(startDate).diff(endDate, 'hours')
      timeValue = diffInHours
      unit = 'hour'

      if (diffInHours >= 24) {
        const diffInDays = moment(startDate).diff(endDate, 'days')
        timeValue = diffInDays
        unit = 'day'
      }
    }

    return `${timeValue} ${unit}${timeValue !== 1 ? 's' : ''}`
  }

  const getStatusAvatar = () => {
    const statusMap = {
      0: { icon: <AccessTime />, color: 'primary' }, // Started
      1: { icon: <Check />, color: 'success' }, // Completed
      2: { icon: <Redo />, color: 'warning' }, // Skipped
    }

    const config = statusMap[historyEntry.status] || statusMap[1]
    return (
      <Avatar
        size='sm'
        color={config.color}
        variant='soft'
        sx={{
          width: 24,
          height: 24,
          '& svg': { fontSize: '14px' },
        }}
      >
        {config.icon}
      </Avatar>
    )
  }

  return (
    <>
      <ListItem
        onClick={onClick}
        sx={{
          cursor: onClick ? 'pointer' : 'default',
          py: 1.5,
          px: 2,
          '&:hover': onClick
            ? {
                backgroundColor: 'background.level1',
              }
            : {},
          borderRadius: 'sm',
          transition: 'background-color 0.2s',
        }}
      >
        <ListItemContent>
          <Grid container spacing={1} alignItems='center'>
            {/* First Row/Column: Status and Time Info */}
            <Grid xs={12} sm={8}>
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  flexWrap: 'wrap',
                }}
              >
                {getStatusAvatar()}

                <Typography
                  level='body-sm'
                  sx={{
                    color: 'text.secondary',
                    fontWeight: 'md',
                  }}
                >
                  {historyEntry.status === 0
                    ? 'In Progress'
                    : historyEntry.status === 1
                      ? 'Completed'
                      : 'Skipped'}
                </Typography>

                <Chip size='sm' startDecorator={<EventNote />}>
                  {moment(
                    historyEntry.performedAt || historyEntry.updatedAt,
                  ).format('MMM DD, h:mm A')}
                </Chip>

                <Box sx={{ display: 'flex', gap: 0.5 }}>
                  {getCompletedChip(historyEntry)}
                </Box>
              </Box>
            </Grid>

            {/* Second Row/Column: Completion Status (right side on desktop) */}
            <Grid xs={12} sm={4}>
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: { xs: 'flex-start', sm: 'flex-end' },
                  alignItems: 'center',
                  gap: 1,
                }}
              >
                {historyEntry.dueDate && (
                  <Chip size='sm' startDecorator={<CalendarMonth />}>
                    {moment(historyEntry.dueDate).format('MMM DD h:mm A')}
                  </Chip>
                )}
              </Box>
            </Grid>

            {/* Third Row: Performer and Assignment Info */}
            <Grid xs={12}>
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  flexWrap: 'wrap',
                  mt: 0.5,
                }}
              >
                <Chip size='sm' variant='outlined' startDecorator={<Person />}>
                  {performer?.displayName || 'Unknown'}
                </Chip>

                {historyEntry.completedBy !== historyEntry.assignedTo &&
                  assignedTo && (
                    <>
                      <Typography
                        level='body-xs'
                        sx={{ color: 'text.tertiary' }}
                      >
                        →
                      </Typography>
                      <Chip
                        size='sm'
                        variant='soft'
                        color='neutral'
                        startDecorator={<CheckCircle />}
                      >
                        {assignedTo.displayName}
                      </Chip>
                    </>
                  )}

                {historyEntry.notes && (
                  <Chip
                    size='sm'
                    variant='plain'
                    color='neutral'
                    startDecorator={<EventNote />}
                    sx={{ maxWidth: '120px', overflow: 'hidden' }}
                  >
                    Note
                  </Chip>
                )}
                {/* add a duration chip if we have duration */}
                {historyEntry?.duration > 0 && (
                  <Chip
                    size='sm'
                    variant='soft'
                    color='primary'
                    startDecorator={<AccessTime />}
                  >
                    {formatTime(historyEntry.duration)}
                  </Chip>
                )}
                {historyEntry?.points > 0 && (
                  <Chip
                    size='sm'
                    variant='solid'
                    color='success'
                    startDecorator={<Toll />}
                  >
                    {historyEntry.points} pt
                    {historyEntry.points > 1 ? 's' : ''}
                  </Chip>
                )}
              </Box>
            </Grid>
          </Grid>
        </ListItemContent>
      </ListItem>

      {/* Compact Divider with Time Difference */}
      {index < allHistory.length - 1 && allHistory[index + 1].performedAt && (
        <ListDivider
          component='li'
          sx={{
            my: 0.5,
          }}
        >
          <Typography
            level='body-xs'
            sx={{
              color: 'text.tertiary',
              backgroundColor: 'background.surface',
              px: 1,
              fontSize: '0.75rem',
            }}
          >
            {formatTimeDifference(
              historyEntry.performedAt || historyEntry.updatedAt,
              allHistory[index + 1].performedAt,
            )}{' '}
            before
          </Typography>
        </ListDivider>
      )}
    </>
  )
}

export default HistoryCard
