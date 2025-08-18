import {
  CheckCircle,
  EventNote,
  Notes,
  Person,
  Redo,
  Refresh,
  Timelapse,
  Toll,
  WatchLater,
} from '@mui/icons-material'
import {
  Avatar,
  Box,
  Chip,
  Divider,
  IconButton,
  List,
  ListItem,
  ListItemContent,
  ListItemDecorator,
  Sheet,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useChores, useChoresHistory } from '../../queries/ChoreQueries'
import { useCircleMembers } from '../../queries/UserQueries'
import { resolvePhotoURL } from '../../utils/Helpers'

const ActivityItem = ({ activity, members }) => {
  // Find the member who completed the activity
  const completedByMember = members?.find(
    member => member.userId === activity.completedBy,
  )

  const getTimeDisplay = dateToDisplay => {
    const now = moment()
    const completed = moment(dateToDisplay)
    const diffInHours = now.diff(completed, 'hours')
    const diffInDays = now.diff(completed, 'days')

    if (diffInHours < 1) {
      return 'Just now'
    } else if (diffInHours < 24) {
      return `${diffInHours}h ago`
    } else if (diffInDays < 7) {
      return `${diffInDays}d ago`
    } else {
      return completed.format('MMM DD')
    }
  }

  const getStatusInfo = activity => {
    if (activity.status === 0) {
      return {
        color: 'primary',
        text: 'Started',
        icon: <Timelapse />,
      }
    }
    if (!activity.status === 1) {
      return {
        color: 'neutral',
        text: 'Completed',
        icon: <CheckCircle />,
      }
    } else if (activity.status === 2) {
      // skipped
      return {
        color: 'warning',
        text: 'Skipped',
        icon: <Redo />,
      }
    }

    const wasOnTime = moment(activity.performedAt).isSameOrBefore(
      moment(activity.dueDate),
    )

    if (wasOnTime) {
      return {
        color: 'success',
        text: 'Done',
        icon: <CheckCircle />,
      }
    } else {
      return {
        color: 'primary',
        text: 'Late',
        icon: <WatchLater />,
      }
    }
  }

  return (
    <ListItem sx={{ alignItems: 'flex-start', py: 0.5 }}>
      <ListItemDecorator sx={{ mt: 0.5 }}>
        <Avatar
          size='sm'
          src={resolvePhotoURL(completedByMember?.image)}
          sx={{ width: 32, height: 32 }}
        >
          {completedByMember?.displayName?.charAt(0) ||
            completedByMember?.name?.charAt(0) || <Person />}
        </Avatar>
      </ListItemDecorator>

      <ListItemContent sx={{ flex: 1 }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          {/* Activity header */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography level='title-sm' sx={{ flex: 1 }}>
              {activity.choreName}
            </Typography>
            <Typography level='body-xs' color='text.secondary'>
              {getTimeDisplay(
                activity.performedAt ||
                  activity.updatedAt ||
                  activity.createdAt,
              )}
            </Typography>
          </Box>

          {/* Who completed it */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            {/* Status chip */}

            <Chip
              size='sm'
              variant='soft'
              color={getStatusInfo(activity).color}
              startDecorator={getStatusInfo(activity).icon}
            >
              {getStatusInfo(activity).text}
            </Chip>
            <Typography level='body-xs' color='text.secondary' sx={{ ml: 0 }}>
              by{' '}
              {completedByMember?.displayName ||
                completedByMember?.name ||
                'Unknown'}
            </Typography>
            {/* Points chip */}
            {activity.points && activity.points > 0 && (
              <Chip
                size='sm'
                variant='soft'
                color='success'
                startDecorator={<Toll />}
              >
                {activity.points} pts
              </Chip>
            )}
          </Box>

          {/* Status, Points, and Notes */}
          <Box
            sx={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: 0.5,
              mt: 0.5,
              ml: 2.5,
            }}
          ></Box>

          {/* Notes */}
          {activity.notes && (
            <Box sx={{ mt: 0.5, ml: 2.5 }}>
              <Typography
                level='body-xs'
                sx={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: 0.5,
                  fontStyle: 'italic',
                  color: 'text.secondary',
                }}
              >
                <Notes sx={{ fontSize: 14, mt: 0.1 }} />
                {activity.notes}
              </Typography>
            </Box>
          )}
        </Box>
      </ListItemContent>
    </ListItem>
  )
}

const groupActivitiesByDate = activities => {
  const groups = {}

  activities.forEach(activity => {
    const date = moment(
      activity.performedAt || activity.updatedAt || activity.createdAt,
    ).format('YYYY-MM-DD')
    if (!groups[date]) {
      groups[date] = []
    }
    groups[date].push(activity)
  })

  return groups
}

const ActivitiesCard = ({ title = 'Recent Activities' }) => {
  // Use hooks to fetch data
  const {
    data: choresData,
    isLoading: isChoresLoading,
    refetch: refetchChores,
  } = useChores(true) // Include archived chores

  const {
    data: choreHistory,
    isLoading: isChoresHistoryLoading,
    refetch: refetchHistory,
  } = useChoresHistory(10, true) // Limit to 10 items, include members

  const {
    data: circleMembersData,
    isLoading: isCircleMembersLoading,
    refetch: refetchMembers,
  } = useCircleMembers()

  // Extract data from responses
  const chores = choresData?.res || []
  const members = circleMembersData?.res || []

  // Refresh function to refetch all data
  const handleRefresh = async () => {
    await Promise.all([refetchChores(), refetchHistory(), refetchMembers()])
  }

  // Show loading state
  if (isChoresLoading || isChoresHistoryLoading || isCircleMembersLoading) {
    return (
      <Sheet
        variant='plain'
        sx={{
          p: 2,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'space-between',
          boxShadow: 'sm',
          borderRadius: 20,
          minHeight: 300,
          width: '315px',
          mb: 1,
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
          <Typography level='title-md'>{title}</Typography>
        </Box>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            height: 200,
          }}
        >
          <Typography level='body-sm' color='neutral'>
            Loading activities...
          </Typography>
        </Box>
      </Sheet>
    )
  }

  // Enrich history with chore names
  const enrichedHistory =
    choreHistory?.map(history => {
      const chore = chores?.find(c => c.id === history.choreId)
      return {
        ...history,
        choreName: chore?.name || 'Unknown Chore',
      }
    }) || []

  // Sort by completion date (most recent first)
  const sortedHistory = enrichedHistory
    .sort(
      (a, b) =>
        moment(b.performedAt || b.updatedAt).valueOf() -
        moment(a.performedAt || a.updatedAt).valueOf(),
    )
    .slice(0, 10) // Show only latest 10 activities

  const groupedActivities = groupActivitiesByDate(sortedHistory)

  if (!sortedHistory.length) {
    return (
      <Sheet
        variant='plain'
        sx={{
          p: 2,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'space-between',
          boxShadow: 'sm',
          borderRadius: 20,
          //   width: '290px',
          minHeight: 300,
          mb: 1,
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
          <EventNote color='' />
          <Typography level='title-md'>{title}</Typography>
        </Box>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            minHeight: 200,
            color: 'text.secondary',
          }}
        >
          <EventNote sx={{ fontSize: 48, opacity: 0.3, mb: 1 }} />
          <Typography level='body-sm'>No recent activities</Typography>
        </Box>
      </Sheet>
    )
  }

  return (
    <Sheet
      variant='plain'
      sx={{
        p: 2,
        display: 'flex',
        flexDirection: 'column',
        boxShadow: 'sm',
        borderRadius: 20,
        width: '310px',
        minHeight: 300,
        maxHeight: 400,
        mb: 1,
      }}
    >
      {/* Header */}
      <Box sx={{ mb: 2 }}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <EventNote color='' />
            <Typography level='title-md'>{title}</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Chip size='sm' variant='soft' color='neutral'>
              {sortedHistory.length}
            </Chip>
            <IconButton
              size='sm'
              variant='soft'
              color='neutral'
              onClick={handleRefresh}
              sx={{ minHeight: 24, minWidth: 24 }}
            >
              <Refresh sx={{ fontSize: 16 }} />
            </IconButton>
          </Box>
        </Box>
      </Box>

      {/* Scrollable activity list */}
      <Box sx={{ maxHeight: 280, overflowY: 'auto', flex: 1 }}>
        {Object.entries(groupedActivities).map(([date, activities]) => {
          const isToday = moment(date).isSame(moment(), 'day')
          const isYesterday = moment(date).isSame(
            moment().subtract(1, 'day'),
            'day',
          )

          let dateLabel
          if (isToday) {
            dateLabel = 'Today'
          } else if (isYesterday) {
            dateLabel = 'Yesterday'
          } else {
            dateLabel = moment(date).format('MMM DD')
          }

          return (
            <Box key={date} sx={{ mb: 1 }}>
              {/* Date separator */}
              <Box sx={{ display: 'flex', alignItems: 'center', my: 1, px: 1 }}>
                <Divider sx={{ flex: 1 }} />
                <Typography
                  level='body-xs'
                  sx={{
                    px: 1,
                    pr: 1,
                    mt: -1,
                    color: 'text.secondary',
                    fontSize: '0.75rem',
                    fontWeight: 500,
                  }}
                >
                  {dateLabel}
                </Typography>
                <Divider sx={{ flex: 1 }} />
              </Box>

              {/* Activities for this date */}
              <List sx={{ py: 0 }}>
                {activities.map(activity => (
                  <ActivityItem
                    key={activity.id}
                    activity={activity}
                    members={members}
                  />
                ))}
              </List>
            </Box>
          )
        })}
      </Box>
    </Sheet>
  )
}

export default ActivitiesCard
