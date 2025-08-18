import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from 'recharts'

import { CreditCard, Toll } from '@mui/icons-material'
import {
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  Container,
  Option,
  Select,
  Stack,
  Tab,
  TabList,
  Tabs,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'
import LoadingComponent from '../components/Loading.jsx'

import { useChoresHistory } from '../../queries/ChoreQueries.jsx'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries.jsx'
import { RedeemPoints } from '../../utils/Fetcher.jsx'
import { resolvePhotoURL } from '../../utils/Helpers.jsx'
import RedeemPointsModal from '../Modals/RedeemPointsModal'
const UserPoints = () => {
  const [tabValue, setTabValue] = useState(7)
  const [isRedeemModalOpen, setIsRedeemModalOpen] = useState(false)

  const {
    data: circleMembersData,
    isLoading: isCircleMembersLoading,
    handleRefetch: handleCircleMembersRefetch,
  } = useCircleMembers()

  const {
    data: choresHistoryData,
    isLoading: isChoresHistoryLoading,
    handleLimitChange: handleChoresHistoryLimitChange,
  } = useChoresHistory(7)

  const { data: userProfile } = useUserProfile()
  const [selectedUser, setSelectedUser] = useState(userProfile?.id)
  const [circleUsers, setCircleUsers] = useState([])
  const [selectedHistory, setSelectedHistory] = useState([])

  useEffect(() => {
    if (circleMembersData && choresHistoryData && userProfile) {
      setCircleUsers(circleMembersData.res)
      setSelectedHistory(
        generateWeeklySummary(choresHistoryData, userProfile?.id),
      )
    }
  }, [circleMembersData, choresHistoryData, userProfile])

  useEffect(() => {
    if (choresHistoryData) {
      var history
      if (tabValue === 7) {
        history = generateWeeklySummary(choresHistoryData, selectedUser)
      } else if (tabValue === 30) {
        history = generateMonthSummary(choresHistoryData, selectedUser)
      } else if (tabValue === 6 * 30) {
        history = generateMonthlySummary(choresHistoryData, selectedUser)
      } else {
        history = generateYearlySummary(choresHistoryData, selectedUser)
      }
      setSelectedHistory(history)
    }
  }, [selectedUser, choresHistoryData, tabValue])

  useEffect(() => {
    setSelectedUser(userProfile?.id)
  }, [userProfile])

  const generateWeeklySummary = (history, userId) => {
    const daysAggregated = []
    for (let i = 6; i > -1; i--) {
      const currentDate = new Date()
      currentDate.setDate(currentDate.getDate() - i)
      daysAggregated.push({
        label: currentDate.toLocaleString('en-US', { weekday: 'short' }),
        points: 0,
        tasks: 0,
      })
    }
    history.forEach(chore => {
      const dayName = new Date(chore.performedAt).toLocaleString('en-US', {
        weekday: 'short',
      })

      const dayIndex = daysAggregated.findIndex(dayData => {
        if (userId)
          return dayData.label === dayName && chore.completedBy === userId
        return dayData.label === dayName
      })
      if (dayIndex !== -1) {
        if (chore.points) daysAggregated[dayIndex].points += chore.points
        daysAggregated[dayIndex].tasks += 1
      }
    })
    return daysAggregated
  }

  const generateMonthSummary = (history, userId) => {
    const daysAggregated = []
    for (let i = 29; i > -1; i--) {
      const currentDate = new Date()
      currentDate.setDate(currentDate.getDate() - i)
      daysAggregated.push({
        label: currentDate.toLocaleString('en-US', { day: 'numeric' }),
        points: 0,
        tasks: 0,
      })
    }
    history.forEach(chore => {
      const dayName = new Date(chore.performedAt).toLocaleString('en-US', {
        day: 'numeric',
      })

      const dayIndex = daysAggregated.findIndex(dayData => {
        if (userId)
          return dayData.label === dayName && chore.completedBy === userId
        return dayData.label === dayName
      })

      if (dayIndex !== -1) {
        if (chore.points) daysAggregated[dayIndex].points += chore.points
        daysAggregated[dayIndex].tasks += 1
      }
    })

    return daysAggregated
  }

  const generateMonthlySummary = (history, userId) => {
    const monthlyAggregated = []
    for (let i = 5; i > -1; i--) {
      const currentMonth = new Date()
      currentMonth.setMonth(currentMonth.getMonth() - i)
      monthlyAggregated.push({
        label: currentMonth.toLocaleString('en-US', { month: 'short' }),
        points: 0,
        tasks: 0,
      })
    }
    history.forEach(chore => {
      const monthName = new Date(chore.performedAt).toLocaleString('en-US', {
        month: 'short',
      })

      const monthIndex = monthlyAggregated.findIndex(monthData => {
        if (userId)
          return monthData.label === monthName && chore.completedBy === userId
        return monthData.label === monthName
      })

      if (monthIndex !== -1) {
        if (chore.points) monthlyAggregated[monthIndex].points += chore.points
        monthlyAggregated[monthIndex].tasks += 1
      }
    })
    return monthlyAggregated
  }

  const generateYearlySummary = (history, userId) => {
    const yearlyAggregated = []

    for (let i = 11; i > -1; i--) {
      const currentYear = new Date()
      currentYear.setFullYear(currentYear.getFullYear() - i)
      yearlyAggregated.push({
        label: currentYear.toLocaleString('en-US', { year: 'numeric' }),
        points: 0,
        tasks: 0,
      })
    }
    history.forEach(chore => {
      const yearName = new Date(chore.performedAt).toLocaleString('en-US', {
        year: 'numeric',
      })

      const yearIndex = yearlyAggregated.findIndex(yearData => {
        if (userId)
          return yearData.label === yearName && chore.completedBy === userId
        return yearData.label === yearName
      })

      if (yearIndex !== -1) {
        if (chore.points) yearlyAggregated[yearIndex].points += chore.points
        yearlyAggregated[yearIndex].tasks += 1
      }
    })
    return yearlyAggregated
  }

  if (isChoresHistoryLoading || isCircleMembersLoading || !userProfile) {
    return <LoadingComponent />
  }

  return (
    <Container
      maxWidth='xl'
      sx={{
        display: 'flex',
        flexDirection: 'column',
        px: { xs: 2, sm: 3 },
      }}
    >
      <Typography
        mb={3}
        level='h4'
        sx={{
          alignSelf: 'flex-start',
        }}
      >
        Points Overview
      </Typography>

      {/* Improved Filter Bar */}
      <Card
        variant='outlined'
        sx={{
          width: '100%',
          p: 2,
          mb: 3,
          borderRadius: 12,
          background:
            'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
          backdropFilter: 'blur(10px)',
        }}
      >
        <Stack spacing={2}>
          <Typography level='title-sm' sx={{ color: 'text.secondary' }}>
            Filter Points
          </Typography>

          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={2}
            alignItems={{ xs: 'stretch', sm: 'center' }}
          >
            {/* User Filter */}
            <Box sx={{ flex: 1, minWidth: 200 }}>
              <Typography level='body-sm' sx={{ mb: 1, fontWeight: 500 }}>
                Show points for:
              </Typography>
              <Select
                sx={{
                  width: '100%',
                }}
                variant='outlined'
                value={selectedUser}
                onChange={(e, selected) => {
                  setSelectedUser(selected)
                  setSelectedHistory(
                    generateWeeklySummary(choresHistoryData, selected),
                  )
                }}
                renderValue={() => {
                  return (
                    <Typography
                      startDecorator={
                        <Avatar
                          color='primary'
                          size='sm'
                          src={resolvePhotoURL(
                            circleUsers.find(
                              user => user.userId === selectedUser,
                            )?.image,
                          )}
                        >
                          {circleUsers
                            .find(user => user.userId === selectedUser)
                            ?.displayName?.charAt(0)}
                        </Avatar>
                      }
                    >
                      {
                        circleUsers.find(user => user.userId === selectedUser)
                          ?.displayName
                      }
                    </Typography>
                  )
                }}
              >
                {circleUsers.map(user => (
                  <Option key={user.userId} value={user.userId}>
                    <Avatar
                      color='primary'
                      size='sm'
                      src={resolvePhotoURL(user.image)}
                    >
                      {user.displayName?.charAt(0)}
                    </Avatar>
                    <Typography>{user.displayName}</Typography>
                    <Chip
                      color='success'
                      size='sm'
                      variant='soft'
                      startDecorator={<Toll />}
                    >
                      {user.points - user.pointsRedeemed}
                    </Chip>
                  </Option>
                ))}
              </Select>
            </Box>

            {/* Time Period Filter */}
            <Box sx={{ flex: 1, minWidth: 200 }}>
              <Typography level='body-sm' sx={{ mb: 1, fontWeight: 500 }}>
                Time period:
              </Typography>
              <Tabs
                onChange={(e, tabValue) => {
                  setTabValue(tabValue)
                  handleChoresHistoryLimitChange(tabValue)
                }}
                value={tabValue}
                sx={{
                  borderRadius: 8,
                  backgroundColor: 'background.surface',
                  border: '1px solid',
                  borderColor: 'divider',
                }}
              >
                <TabList
                  disableUnderline
                  sx={{
                    borderRadius: 8,
                    backgroundColor: 'transparent',
                    p: 0.5,
                    gap: 0.5,
                  }}
                >
                  {[
                    { label: '7 Days', value: 7 },
                    { label: '6 Months', value: 6 * 30 },
                    { label: 'All Time', value: 24 * 30 },
                  ].map((tab, index) => (
                    <Tab
                      key={index}
                      sx={{
                        borderRadius: 6,
                        minWidth: 'auto',
                        px: 2,
                        py: 1,
                        fontSize: 'sm',
                        fontWeight: 500,
                        color: 'text.secondary',
                        '&.Mui-selected': {
                          color: 'primary.plainColor',
                          backgroundColor: 'primary.softBg',
                          fontWeight: 600,
                        },
                        '&:hover': {
                          backgroundColor: 'neutral.softHoverBg',
                        },
                      }}
                      disableIndicator
                      value={tab.value}
                    >
                      {tab.label}
                    </Tab>
                  ))}
                </TabList>
              </Tabs>
            </Box>

            {/* Redeem Points Button */}
            {circleUsers.find(user => user.userId === userProfile.id)?.role ===
              'admin' && (
              <Box sx={{ display: 'flex', alignItems: 'flex-end' }}>
                <Button
                  variant='soft'
                  size='md'
                  startDecorator={<CreditCard />}
                  onClick={() => {
                    setIsRedeemModalOpen(true)
                  }}
                  sx={{ mt: 'auto' }}
                >
                  Redeem Points
                </Button>
              </Box>
            )}
          </Stack>
        </Stack>
      </Card>

      {/* Current Filter Summary */}
      <Box sx={{ mb: 3, textAlign: 'center' }}>
        <Typography level='body-sm' sx={{ color: 'text.secondary' }}>
          Showing points for{' '}
          <Typography
            component='span'
            sx={{ fontWeight: 600, color: 'primary.500' }}
          >
            {circleUsers.find(user => user.userId === selectedUser)
              ?.displayName || 'Unknown User'}
          </Typography>{' '}
          over the{' '}
          <Typography
            component='span'
            sx={{ fontWeight: 600, color: 'primary.500' }}
          >
            {tabValue === 24 * 30
              ? 'All Time'
              : tabValue === 6 * 30
                ? 'Last 6 Months'
                : `Last ${tabValue} Days`}
          </Typography>
        </Typography>
      </Box>

      <Box
        sx={{
          mb: 4,
          display: 'flex',
          flexDirection: 'column',
          gap: 3,
        }}
      >
        {/* Points Cards */}
        <Box
          sx={{
            // resposive width based on parent available space:
            width: '100%',
            display: 'flex',
            justifyContent: 'space-evenly',
            gap: 1,
          }}
        >
          {[
            {
              title: 'Total',
              value: circleMembersData.res.find(
                user => user.userId === selectedUser,
              )?.points,
              color: 'primary',
            },
            {
              title: 'Available',
              value: (function () {
                const user = circleMembersData.res.find(
                  user => user.userId === selectedUser,
                )
                if (!user) return 0
                return user.points - user.pointsRedeemed
              })(),
              color: 'success',
            },
            {
              title: 'Redeemed',
              value: circleMembersData.res.find(
                user => user.userId === selectedUser,
              )?.pointsRedeemed,
              color: 'warning',
            },
          ].map(card => (
            <Card
              key={card.title}
              sx={{
                p: 2,
                mb: 1,
                minWidth: 80,
                width: '100%',
              }}
              variant='soft'
            >
              <Typography level='body-xs' textAlign='center' mb={-1}>
                {card.title}
              </Typography>
              <Typography level='title-md' textAlign='center'>
                {card.value}
              </Typography>
            </Card>
          ))}
        </Box>

        {/* Points History Section */}
        <Typography level='h4' sx={{ mt: 2, mb: 2 }}>
          Points History
        </Typography>

        <Box
          sx={{
            // resposive width based on parent available space:
            width: '100%',
            display: 'flex',
            justifyContent: 'left',
            gap: 1,
            mb: 3,
          }}
        >
          {[
            {
              title: 'Points',
              value: selectedHistory.reduce((acc, cur) => acc + cur.points, 0),
              color: 'success',
            },
            {
              title: 'Tasks',
              value: selectedHistory.reduce((acc, cur) => acc + cur.tasks, 0),
              color: 'primary',
            },
          ].map(card => (
            <Card
              key={card.title}
              sx={{
                p: 2,
                mb: 1,
                width: 250,
              }}
              variant='soft'
            >
              <Typography level='body-xs' textAlign='center' mb={-1}>
                {card.title}
              </Typography>
              <Typography level='title-md' textAlign='center'>
                {card.value}
              </Typography>
            </Card>
          ))}
        </Box>

        {/* Bar Chart for points overtime */}
        <Box sx={{ display: 'flex', justifyContent: 'center', gap: 1 }}>
          <ResponsiveContainer height={300}>
            <BarChart
              data={selectedHistory}
              margin={{ top: 5, left: -20, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray={'3 3'} />
              <XAxis dataKey='label' axisLine={false} tickLine={false} />
              <YAxis axisLine={false} tickLine={false} />
              <Bar
                fill='#4183F2'
                dataKey='points'
                barSize={30}
                radius={[5, 5, 0, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
        </Box>
      </Box>

      <RedeemPointsModal
        config={{
          onClose: () => {
            setIsRedeemModalOpen(false)
          },
          isOpen: isRedeemModalOpen,
          available: circleUsers.find(user => user.userId === selectedUser)
            ?.points,
          user: circleUsers.find(user => user.userId === selectedUser),
          onSave: ({ userId, points }) => {
            RedeemPoints(userId, points, userProfile.circleID)
              .then(() => {
                setIsRedeemModalOpen(false)
                handleCircleMembersRefetch()
              })
              .catch(err => {
                console.log(err)
              })
          },
        }}
      />
    </Container>
  )
}

export default UserPoints
