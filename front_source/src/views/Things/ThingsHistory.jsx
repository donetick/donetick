import { EventBusy, Schedule, TrendingUp } from '@mui/icons-material'
import {
  Avatar,
  Box,
  Button,
  Chip,
  Container,
  Grid,
  List,
  ListDivider,
  ListItem,
  ListItemContent,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { Link, useParams } from 'react-router-dom'
import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { useTheme } from '@mui/joy/styles'
import { useThingHistory } from '../../queries/ThingQueries'
import LoadingComponent from '../components/Loading'

const ThingsHistory = () => {
  const { id } = useParams()
  const theme = useTheme()
  const {
    data,
    error,
    isLoading,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useThingHistory(id)

  // Flatten all pages of history data
  const thingsHistory = data?.pages.flatMap(page => page.res) || []

  const handleLoadMore = () => {
    fetchNextPage()
  }

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
  // if loading show loading spinner:
  if (isLoading) {
    return <LoadingComponent />
  }

  if (error || !thingsHistory || thingsHistory.length === 0) {
    return (
      <Container
        maxWidth='md'
        sx={{
          textAlign: 'center',
          display: 'flex',
          // make sure the content is centered vertically:
          alignItems: 'center',
          justifyContent: 'center',
          flexDirection: 'column',
          height: '50vh',
        }}
      >
        <EventBusy
          sx={{
            fontSize: '6rem',
            // color: 'text.disabled',
            mb: 1,
          }}
        />

        <Typography level='h3' gutterBottom>
          No history found
        </Typography>
        <Typography level='body1'>
          It looks like there is no history for this thing yet.
        </Typography>
        <Button variant='soft' sx={{ mt: 2 }}>
          <Link to='/things'>Go back to things</Link>
        </Button>
      </Container>
    )
  }

  return (
    <Container maxWidth='md'>
      <Typography level='h3' mb={1.5}>
        History:
      </Typography>
      {/* check if all the states are number the show it: */}
      {thingsHistory.every(history => !isNaN(history.state)) &&
        thingsHistory.length > 1 && (
          <>
            <Typography level='h4' gutterBottom>
              Chart:
            </Typography>

            <Box sx={{ borderRadius: 'sm', p: 2, boxShadow: 'md', mb: 2 }}>
              <ResponsiveContainer width='100%' height={200}>
                <LineChart
                  width={500}
                  height={300}
                  data={thingsHistory.toReversed()}
                >
                  {/* <CartesianGrid strokeDasharray='3 3' /> */}
                  <XAxis
                    dataKey='updatedAt'
                    hide='true'
                    tick='false'
                    tickLine='false'
                    axisLine='false'
                    tickFormatter={tick =>
                      moment(tick).format('ddd MM/DD/yyyy HH:mm:ss')
                    }
                  />
                  <YAxis
                    hide='true'
                    dataKey='state'
                    tick='false'
                    tickLine='true'
                    axisLine='false'
                  />
                  <Tooltip
                    labelFormatter={label =>
                      moment(label).format('ddd MM/DD/yyyy HH:mm:ss')
                    }
                  />

                  <Line
                    type='monotone'
                    dataKey='state'
                    stroke={theme.palette.primary[500]}
                    activeDot={{
                      r: 8,
                      fill: theme.palette.primary[600],
                      stroke: theme.palette.primary[300],
                    }}
                    dot={{
                      r: 4,
                      fill: theme.palette.primary[500],
                      stroke: theme.palette.primary[300],
                    }}
                  />
                </LineChart>
              </ResponsiveContainer>
            </Box>
          </>
        )}
      <Typography level='h4' gutterBottom>
        Change log:
      </Typography>
      <Box sx={{ borderRadius: 'sm', p: 1, boxShadow: 'md' }}>
        <List sx={{ p: 0 }}>
          {thingsHistory.map((history, index) => (
            <Box key={index}>
              <ListItem
                sx={{
                  py: 1.5,
                  px: 2,
                  borderRadius: 'sm',
                  transition: 'background-color 0.2s',
                  '&:hover': {
                    backgroundColor: 'background.level1',
                  },
                }}
              >
                <ListItemContent>
                  <Grid container spacing={1} alignItems='center'>
                    {/* First Row: Status and Time Info */}
                    <Grid xs={12} sm={8}>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 1,
                          flexWrap: 'wrap',
                        }}
                      >
                        <Avatar
                          size='sm'
                          color='primary'
                          variant='solid'
                          sx={{
                            width: 24,
                            height: 24,
                            '& svg': { fontSize: '14px' },
                          }}
                        >
                          <TrendingUp />
                        </Avatar>

                        <Typography
                          level='body-sm'
                          sx={{
                            color: 'text.secondary',
                            fontWeight: 'md',
                            display: { xs: 'none', sm: 'block' },
                          }}
                        >
                          Updated
                        </Typography>

                        <Chip
                          size='sm'
                          variant='soft'
                          color='primary'
                          startDecorator={<Schedule />}
                        >
                          {moment(history.updatedAt).format('MMM DD, h:mm A')}
                        </Chip>
                      </Box>
                    </Grid>

                    {/* Second Row: State Value */}
                    <Grid xs={12} sm={4}>
                      <Box
                        sx={{
                          display: 'flex',
                          justifyContent: { xs: 'flex-start', sm: 'flex-end' },
                          alignItems: 'center',
                          gap: 1,
                        }}
                      >
                        <Chip
                          size='md'
                          variant='solid'
                          color='success'
                          sx={{ fontWeight: 'bold' }}
                        >
                          {history.state}
                        </Chip>
                      </Box>
                    </Grid>
                  </Grid>
                </ListItemContent>
              </ListItem>

              {/* Divider with time difference */}
              {index < thingsHistory.length - 1 && (
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
                      history.createdAt,
                      thingsHistory[index + 1].createdAt,
                    )}{' '}
                    before
                  </Typography>
                </ListDivider>
              )}
            </Box>
          ))}
        </List>
      </Box>
      {/* Load more Button  */}
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'center',
          mt: 2,
        }}
      >
        <Button
          variant='plain'
          fullWidth
          color='primary'
          onClick={handleLoadMore}
          disabled={!hasNextPage || isFetchingNextPage}
        >
          {isFetchingNextPage
            ? 'Loading...'
            : !hasNextPage
              ? 'No more history'
              : 'Load more'}
        </Button>
      </Box>
    </Container>
  )
}

export default ThingsHistory
